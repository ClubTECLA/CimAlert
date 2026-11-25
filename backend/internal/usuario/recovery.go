package usuario

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/logging"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Parámetros de la recuperación de contraseña.
const (
	pinTTLMinutos       = 15
	maxIntentosPin      = 5
	maxSolicitudesPorHr = 3
	resetTTLMinutos     = 15 // vigencia del token de reset, desde que se verifica el PIN
	resetTokenBytes     = 32
)

// recoveryStore es lo que Recuperacion necesita de Store; los tests lo
// cumplen con un falso en memoria.
type recoveryStore interface {
	GetUserByEmail(ctx context.Context, email *string) (sqlc.User, error)
	GetLatestActivePINByUserID(ctx context.Context, userID uuid.UUID) (sqlc.RecoveryPin, error)
	AddPINAttemptByPINID(ctx context.Context, id uuid.UUID) (int, error)
	CreatePinRecuperacion(ctx context.Context, arg CreatePinTxParams) (bool, error)
	SetPinAsVerifiedByID(ctx context.Context, arg sqlc.SetPinAsVerifiedByIDParams) (int64, error)
	GetActiveResetByTokenHash(ctx context.Context, resetTokenHash []byte) (sqlc.GetActiveResetByTokenHashRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error)
	ResetPasswordTx(ctx context.Context, arg ResetPasswordTxParams) error
}

// Recuperacion implementa el flujo de recuperación de contraseña junto a Auth.
type Recuperacion struct {
	store  recoveryStore
	mailer Mailer
	secret []byte // RECOVERY_SECRET, llave del HMAC de los PIN
}

func NewRecuperacion(store recoveryStore, mailer Mailer, secret []byte) *Recuperacion {
	return &Recuperacion{store: store, mailer: mailer, secret: secret}
}

// Solicitar, si la cuenta tiene email y password local y no llegó al límite
// de maxSolicitudesPorHr, cancela los PIN anteriores, guarda uno nuevo y lo
// manda. Devuelve nil exista o no la cuenta y aunque haya llegado al límite
// (en ese caso no envía nada): un 429 solo para cuentas existentes revelaría
// cuáles existen.
//
// Solo la búsqueda del usuario ocurre antes de responder; el resto corre en
// segundo plano para que el tiempo de respuesta no dependa de si la cuenta existe.
func (r *Recuperacion) Solicitar(ctx context.Context, email string) error {
	email = normalizeEmail(email)

	user, err := r.store.GetUserByEmail(ctx, &email)
	if db.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.PasswordHash == nil {
		return nil
	}

	enviarEnSegundoPlano(ctx, user.ID, "recuperación de contraseña en segundo plano", func(ctx context.Context) error {
		pin, err := generarPin()
		if err != nil {
			return err
		}
		creado, err := r.store.CreatePinRecuperacion(ctx, CreatePinTxParams{
			UserID:     user.ID,
			PinHash:    r.hashPin(pin),
			ExpiresAt:  time.Now().Add(pinTTLMinutos * time.Minute),
			MaxPorHora: maxSolicitudesPorHr,
		})
		if err != nil || !creado {
			return err
		}
		return r.mailer.EnviarPinRecuperacion(ctx, user.ID, email, pin)
	})
	return nil
}

// Verificar valida el PIN (con HMAC dummy si no hay PIN activo) y, si es
// correcto, lo cierra para nuevos intentos y devuelve un token de reset de un
// solo uso que vence a los resetTTLMinutos. La contraseña no cambia aquí:
// eso lo hace Restablecer con el token. Todo fallo devuelve ErrCodigoInvalido.
func (r *Recuperacion) Verificar(ctx context.Context, email, pin string) (string, error) {
	email = normalizeEmail(email)

	user, err := r.store.GetUserByEmail(ctx, &email)
	if err != nil && !db.IsNotFound(err) {
		return "", err
	}

	var activo sqlc.RecoveryPin
	hayPin := false
	if err == nil && user.PasswordHash != nil {
		activo, err = r.store.GetLatestActivePINByUserID(ctx, user.ID)
		if err != nil && !db.IsNotFound(err) {
			return "", err
		}
		hayPin = err == nil
	}

	if !hayPin {
		// Mismo trabajo que con un PIN real, para que el tiempo no revele nada.
		hmac.Equal(r.hashPin(pin), dummyPinHash)
		return "", ErrCodigoInvalido
	}

	intentos, err := r.store.AddPINAttemptByPINID(ctx, activo.ID)
	if err != nil {
		return "", err
	}
	if !hmac.Equal(r.hashPin(pin), activo.PinHash) || intentos > maxIntentosPin {
		return "", ErrCodigoInvalido
	}

	token, err := generarResetToken()
	if err != nil {
		return "", err
	}
	expira := time.Now().Add(resetTTLMinutos * time.Minute)
	verificado, err := r.store.SetPinAsVerifiedByID(ctx, sqlc.SetPinAsVerifiedByIDParams{
		ID:             activo.ID,
		ResetTokenHash: r.hashPin(token),
		ResetExpiresAt: &expira,
	})
	if err != nil {
		return "", err
	}
	if verificado == 0 {
		// Otra verificación concurrente del mismo PIN ganó, o se canceló o expiró entretanto.
		return "", ErrCodigoInvalido
	}
	return token, nil
}

// Restablecer cambia la contraseña con un token de Verificar, en una
// transacción que también lo cierra, y manda el aviso. Todo fallo del token
// devuelve ErrCodigoInvalido.
func (r *Recuperacion) Restablecer(ctx context.Context, resetToken, newPassword string) error {
	reset, err := r.store.GetActiveResetByTokenHash(ctx, r.hashPin(resetToken))
	if db.IsNotFound(err) {
		return ErrCodigoInvalido
	}
	if err != nil {
		return err
	}

	user, err := r.store.GetUserByID(ctx, reset.UserID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := r.store.ResetPasswordTx(ctx, ResetPasswordTxParams{
		UserID:       reset.UserID,
		PinID:        reset.ID,
		PasswordHash: string(hash),
	}); err != nil {
		return err
	}

	if user.Email != nil {
		email := *user.Email
		enviarEnSegundoPlano(ctx, user.ID, "aviso de password cambiado", func(ctx context.Context) error {
			return r.mailer.EnviarAvisoPasswordCambiado(ctx, user.ID, email)
		})
	}
	return nil
}

// dummyPinHash se compara cuando no hay PIN activo; su valor no importa,
// solo que tenga el largo de un HMAC-SHA256.
var dummyPinHash = make([]byte, sha256.Size)

// generarPin devuelve un PIN de 6 dígitos con crypto/rand.
func generarPin() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// generarResetToken devuelve 32 bytes de crypto/rand en base64url sin relleno.
func generarResetToken() (string, error) {
	b := make([]byte, resetTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashPin devuelve HMAC-SHA256(secret, pin). También se usa para el token de reset.
func (r *Recuperacion) hashPin(pin string) []byte {
	mac := hmac.New(sha256.New, r.secret)
	mac.Write([]byte(pin))
	return mac.Sum(nil)
}

// segundoPlanoTimeout es lo máximo que puede tardar un envío en segundo plano.
const segundoPlanoTimeout = 30 * time.Second

// enviarEnSegundoPlano corre send sin bloquear la respuesta. Loguea el error
// con el userID y descripcion, nunca con el PIN.
//
// El ctx de la petición es el *gin.Context, que gin recicla al responder, así
// que la goroutine no puede usarlo (ni envuelto en WithoutCancel): arranca de
// un contexto nuevo y solo se lleva el request_id para el log.
func enviarEnSegundoPlano(ctx context.Context, userID uuid.UUID, descripcion string, send func(context.Context) error) {
	bg := logging.ContextWithRequestID(context.Background(), logging.RequestIDFromContext(ctx))
	ctx, cancel := context.WithTimeout(bg, segundoPlanoTimeout)
	go func() {
		defer cancel()
		if err := send(ctx); err != nil {
			slog.ErrorContext(ctx, descripcion, "user_id", userID, "error", err)
		}
	}()
}
