package usuario

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
)

// Vida de una sesión: se extiende hasta idle con cada refresh, pero nunca pasa
// de tope desde el login. Con remember_me dura más y la cookie es persistente.
const (
	idleRecordar = 30 * 24 * time.Hour
	topeRecordar = 90 * 24 * time.Hour
	idleSesion   = 24 * time.Hour
	topeSesion   = 7 * 24 * time.Hour

	// graciaRotacion es cuánto sigue sirviendo el token anterior tras rotarlo,
	// para que dos refresh concurrentes (dos pestañas) no cierren la sesión.
	graciaRotacion = 30 * time.Second

	refreshTokenBytes = 32
)

// Tokens es lo que recibe el cliente al iniciar o renovar sesión.
type Tokens struct {
	Access string
	TTL    time.Duration
	// Refresh es "" cuando el refresh entró por la ventana de gracia: el
	// cliente se queda con el que le dio la petición que sí rotó.
	Refresh    string
	RememberMe bool
	// Expira es cuándo vence la sesión si no se vuelve a usar.
	Expira time.Time
}

// expiracion es el nuevo vencimiento de una sesión que empezó en inicio y se usa ahora.
func expiracion(rememberMe bool, inicio, ahora time.Time) time.Time {
	idle, tope := idleSesion, topeSesion
	if rememberMe {
		idle, tope = idleRecordar, topeRecordar
	}
	if fin := inicio.Add(tope); fin.Before(ahora.Add(idle)) {
		return fin
	}
	return ahora.Add(idle)
}

// hashRefresh es lo que se guarda del refresh token. SHA-256 sin secreto basta:
// el token tiene 256 bits aleatorios, no se puede adivinar desde el hash.
func hashRefresh(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func nuevoRefreshToken() (string, error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// abrirSesion firma el token de acceso y crea la sesión con su refresh token.
// user debe ser la fila leída junto con la credencial validada: su
// password_changed_at queda como pwd_at de la sesión.
func (s *Auth) abrirSesion(ctx context.Context, user sqlc.User, rol string, rememberMe bool) (Tokens, error) {
	access, err := s.issueToken(user, rol)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := nuevoRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	ahora := time.Now()
	exp := expiracion(rememberMe, ahora, ahora)
	if err := s.store.CreateSesion(ctx, sqlc.CreateSesionParams{
		UserID:     user.ID,
		TokenHash:  hashRefresh(refresh),
		ExpiresAt:  exp,
		PwdAt:      middleware.VersionCredencial(user.PasswordChangedAt),
		RememberMe: rememberMe,
		Ahora:      ahora,
	}); err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, TTL: s.ttl, Refresh: refresh, RememberMe: rememberMe, Expira: exp}, nil
}

// Refresh renueva el token de acceso con el refresh token y lo rota. Si llega
// el token anterior dentro de graciaRotacion, firma otro token de acceso sin
// rotar; fuera de ella lo toma como robado y revoca la sesión. Cualquier
// rechazo es ErrSesionInvalida, sin decir por qué.
func (s *Auth) Refresh(ctx context.Context, token string) (sqlc.User, Tokens, error) {
	if token == "" {
		return sqlc.User{}, Tokens{}, ErrSesionInvalida
	}
	hash := hashRefresh(token)
	ahora := time.Now()

	// Dos vueltas: si otra petición rota primero, la segunda lectura encuentra
	// el token como anterior y entra por la gracia.
	for range 2 {
		ses, err := s.store.GetSesionPorTokenHash(ctx, hash)
		if db.IsNotFound(err) {
			return sqlc.User{}, Tokens{}, ErrSesionInvalida
		}
		if err != nil {
			return sqlc.User{}, Tokens{}, err
		}
		if ses.RevokedAt != nil || !ahora.Before(ses.ExpiresAt) {
			return sqlc.User{}, Tokens{}, ErrSesionInvalida
		}

		actual := ses.TokenHash == hash
		if !actual && ahora.Sub(ses.LastUsedAt) > graciaRotacion {
			slog.WarnContext(ctx, "refresh token reused; session revoked",
				slog.String("user_id", ses.UserID.String()), slog.String("session_id", ses.ID.String()))
			return sqlc.User{}, Tokens{}, s.revocar(ctx, ses)
		}

		row, err := s.store.GetUserConRolPorID(ctx, ses.UserID)
		if db.IsNotFound(err) {
			return sqlc.User{}, Tokens{}, ErrSesionInvalida
		}
		if err != nil {
			return sqlc.User{}, Tokens{}, err
		}
		// Un reset o el vínculo con Google cambian password_changed_at: la
		// sesión abierta con la versión anterior ya no sirve.
		if estaSuspendido(row.User.State) || middleware.VersionCredencial(row.User.PasswordChangedAt) != ses.PwdAt {
			return sqlc.User{}, Tokens{}, s.revocar(ctx, ses)
		}

		t := Tokens{TTL: s.ttl, RememberMe: ses.RememberMe, Expira: ses.ExpiresAt}
		if actual {
			if t.Refresh, err = nuevoRefreshToken(); err != nil {
				return sqlc.User{}, Tokens{}, err
			}
			t.Expira = expiracion(ses.RememberMe, ses.CreatedAt, ahora)
			n, err := s.store.RotarSesion(ctx, sqlc.RotarSesionParams{
				Nuevo: hashRefresh(t.Refresh), ExpiresAt: t.Expira, Ahora: ahora, ID: ses.ID, Anterior: hash,
			})
			if err != nil {
				return sqlc.User{}, Tokens{}, err
			}
			if n == 0 {
				continue
			}
		}

		if t.Access, err = s.issueToken(row.User, row.RolName); err != nil {
			return sqlc.User{}, Tokens{}, err
		}
		return row.User, t, nil
	}
	return sqlc.User{}, Tokens{}, ErrSesionInvalida
}

// revocar cierra la sesión y devuelve ErrSesionInvalida, o el error de la base.
func (s *Auth) revocar(ctx context.Context, ses sqlc.RefreshToken) error {
	if err := s.store.RevocarSesionPorID(ctx, ses.ID); err != nil {
		return err
	}
	return ErrSesionInvalida
}

// Logout revoca la sesión del refresh token. Sin token, o con uno que no
// existe, no hace nada: cerrar sesión siempre sale bien.
func (s *Auth) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.RevocarSesionPorTokenHash(ctx, hashRefresh(token))
}
