package usuario

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/historial"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store expone las consultas que sqlc genera y añade las operaciones de
// usuarios que necesitan más de una sentencia dentro de una transacción.
type Store struct {
	*sqlc.Queries
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Queries: sqlc.New(pool), pool: pool}
}

// CreateUserWithLocalProvider inserta el usuario y su proveedor de
// autenticación local en la misma transacción, para que nunca quede un
// usuario sin forma de autenticarse (o un auth_provider huérfano) si una de
// las dos falla.
func (s *Store) CreateUserWithLocalProvider(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	var user sqlc.User

	err := db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		var err error
		if user, err = q.CreateUser(ctx, arg); err != nil {
			return err
		}
		if err = q.CreateLocalAuthProvider(ctx, user.ID); err != nil {
			return err
		}
		// El actor es el propio usuario: se registra a sí mismo.
		return historial.Registrar(ctx, q, user.ID, "usuario.registrar", "usuario", user.ID.String(), nil)
	})
	if err != nil {
		return sqlc.User{}, err
	}

	return user, nil
}

// CreatePinRecuperacion, en una transacción que bloquea la fila del usuario,
// revisa el límite por hora, borra los PIN expirados o usados del usuario,
// cancela los activos e inserta el nuevo. Devuelve false, sin crear nada, si
// el usuario ya llegó a arg.MaxPorHora.
func (s *Store) CreatePinRecuperacion(ctx context.Context, arg CreatePinTxParams) (bool, error) {
	creado := false
	err := db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		if err := q.LockUserByID(ctx, arg.UserID); err != nil {
			return err
		}
		n, err := q.CountPINsLastHourByUserID(ctx, arg.UserID)
		if err != nil {
			return err
		}
		if n >= arg.MaxPorHora {
			return nil
		}
		if err := q.DeleteUsedOrExpiredPINByUserID(ctx, arg.UserID); err != nil {
			return err
		}
		if err := q.CancelPINByUserID(ctx, arg.UserID); err != nil {
			return err
		}
		if err := q.CreatePIN(ctx, sqlc.CreatePINParams{
			UserID:    arg.UserID,
			PinHash:   arg.PinHash,
			ExpiresAt: arg.ExpiresAt,
		}); err != nil {
			return err
		}
		creado = true
		return nil
	})
	return creado, err
}

// ResetPasswordTx, en una transacción, guarda el nuevo password_hash, pone
// password_changed_at a la hora actual de la API, cierra el token de reset y
// revoca todas las sesiones del usuario.
// Devuelve ErrCodigoInvalido si el token ya se usó, se canceló o expiró.
func (s *Store) ResetPasswordTx(ctx context.Context, arg ResetPasswordTxParams) error {
	return db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		// Mismo orden de bloqueo que CreatePinRecuperacion (usuario, luego PIN) para no causar deadlock.
		if err := q.LockUserByID(ctx, arg.UserID); err != nil {
			return err
		}
		filas, err := q.ConsumeResetByID(ctx, arg.PinID)
		if err != nil {
			return err
		}
		if filas == 0 {
			return ErrCodigoInvalido
		}
		// Cierra todas las sesiones: quien cambió la contraseña puede estar echando a un intruso.
		if err := q.RevocarSesionesPorUserID(ctx, arg.UserID); err != nil {
			return err
		}
		changedAt := time.Now()
		if err := q.SetNewPasswordByUserID(ctx, sqlc.SetNewPasswordByUserIDParams{
			PasswordHash:      &arg.PasswordHash,
			PasswordChangedAt: &changedAt,
			ID:                arg.UserID,
		}); err != nil {
			return err
		}
		// El actor es el propio usuario: demostró con el PIN (vía el token) que controla su email.
		return historial.Registrar(ctx, q, arg.UserID, "usuario.recuperar_password", "usuario", arg.UserID.String(), nil)
	})
}

// Índices únicos que una petición concurrente con el mismo email o el mismo
// sub puede romper; LoginGoogleTx reintenta una vez si pasa.
const (
	uniqueEmail     = "users_email_key"
	uniqueGoogleSub = "auth_providers_provider_provider_id_key"
)

// erroresGoogle son los errores de dominio que LoginGoogleTx devuelve tal cual.
var erroresGoogle = []error{ErrUsuarioSuspendido, ErrGoogleEmailNoAdmitido, ErrGoogleCuentaDistinta, ErrGoogleVinculoNoPermitido}

// LoginGoogleTx inicia sesión con una identidad de Google ya verificada:
// encuentra la cuenta vinculada, vincula la cuenta local con el mismo email o
// crea una nueva (pasos 1 a 3 del plan técnico).
//
// Dos peticiones concurrentes pueden chocar en el UNIQUE de users.email (dos
// altas) o en el del sub (dos vínculos). La que pierde reintenta una vez y,
// como la otra ya terminó, encuentra la cuenta en el paso 1.
func (s *Store) LoginGoogleTx(ctx context.Context, p GoogleLoginParams) (GoogleLoginResult, error) {
	res, err := s.loginGoogleOnce(ctx, p)
	if err != nil && (db.IsUniqueViolationOn(err, uniqueEmail) || db.IsUniqueViolationOn(err, uniqueGoogleSub)) {
		res, err = s.loginGoogleOnce(ctx, p)
	}
	if err != nil {
		for _, e := range erroresGoogle {
			if errors.Is(err, e) {
				return GoogleLoginResult{}, err
			}
		}
		return GoogleLoginResult{}, fmt.Errorf("login google: %w", err)
	}
	return res, nil
}

func (s *Store) loginGoogleOnce(ctx context.Context, p GoogleLoginParams) (GoogleLoginResult, error) {
	var res GoogleLoginResult
	err := db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		// Paso 1: la identidad de Google ya está vinculada.
		vinculado, err := q.GetUserWithRolByGoogleSub(ctx, &p.Sub)
		switch {
		case err == nil:
			if estaSuspendido(vinculado.User.State) {
				return ErrUsuarioSuspendido
			}
			res = GoogleLoginResult{User: vinculado.User, RolNombre: vinculado.RolName}
			return nil
		case !db.IsNotFound(err):
			return err
		}

		// Paso 2: antes de buscar por email, para no revelar si existe una
		// cuenta con un email del que Google no es la autoridad.
		if !emailAutoritativo(p.Email, p.HD) {
			return ErrGoogleEmailNoAdmitido
		}

		// Paso 3: la cuenta local con el mismo email, bloqueada.
		row, err := q.GetUserWithRolByEmailForUpdate(ctx, &p.Email)
		if db.IsNotFound(err) {
			return s.crearConGoogle(ctx, q, p, &res)
		}
		if err != nil {
			return err
		}

		googleID, err := q.GetGoogleProviderIDByUserID(ctx, row.User.ID)
		if err != nil && !db.IsNotFound(err) {
			return err
		}
		yaVinculado, quitarPassword, err := decidirVinculo(candidatoVinculo{
			Suspendido:            estaSuspendido(row.User.State),
			RolNombre:             row.RolName,
			RolDefault:            p.DefaultRoleName,
			GoogleProviderID:      googleID,
			PasswordChangedAtNulo: row.User.PasswordChangedAt == nil,
		}, p.Sub)
		if err != nil {
			return err
		}
		if yaVinculado {
			res = GoogleLoginResult{User: row.User, RolNombre: row.RolName}
			return nil
		}

		// Vincular.
		if err := q.CreateGoogleAuthProvider(ctx, sqlc.CreateGoogleAuthProviderParams{UserID: row.User.ID, ProviderID: &p.Sub}); err != nil {
			return err
		}
		if quitarPassword {
			// Viene de la API, no de now(), como en SetNewPasswordByUserID:
			// middleware.Auth la compara con el pwd_at del JWT.
			changedAt := time.Now()
			if row.User, err = q.RemovePasswordByUserID(ctx, sqlc.RemovePasswordByUserIDParams{ID: row.User.ID, PasswordChangedAt: &changedAt}); err != nil {
				return err
			}
			if err := q.DeleteLocalAuthProviderByUserID(ctx, row.User.ID); err != nil {
				return err
			}
			// Cierra los PIN, tokens de reset y sesiones que alguien más pudo abrir.
			if err := q.CancelPINByUserID(ctx, row.User.ID); err != nil {
				return err
			}
			if err := q.RevocarSesionesPorUserID(ctx, row.User.ID); err != nil {
				return err
			}
		}
		if err := historial.Registrar(ctx, q, row.User.ID, "usuario.vincular_google", "usuario", row.User.ID.String(),
			map[string]any{"password_removed": quitarPassword}); err != nil {
			return err
		}
		res = GoogleLoginResult{User: row.User, RolNombre: row.RolName, Linked: true, PasswordRemoved: quitarPassword}
		return nil
	})
	if err != nil {
		return GoogleLoginResult{}, err
	}
	return res, nil
}

// crearConGoogle crea la cuenta sin password y su vínculo con Google.
func (s *Store) crearConGoogle(ctx context.Context, q *sqlc.Queries, p GoogleLoginParams, res *GoogleLoginResult) error {
	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{Email: &p.Email, Username: p.Username, RolID: p.DefaultRoleID, PasswordHash: nil})
	if err != nil {
		return err
	}
	if err := q.CreateGoogleAuthProvider(ctx, sqlc.CreateGoogleAuthProviderParams{UserID: user.ID, ProviderID: &p.Sub}); err != nil {
		return err
	}
	// El actor es el propio usuario: se registra a sí mismo.
	if err := historial.Registrar(ctx, q, user.ID, "usuario.registrar_google", "usuario", user.ID.String(), nil); err != nil {
		return err
	}
	*res = GoogleLoginResult{User: user, RolNombre: p.DefaultRoleName, Created: true}
	return nil
}
