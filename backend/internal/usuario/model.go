package usuario

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/google/uuid"
)

// CreatePinTxParams son los datos para emitir un PIN de recuperación nuevo.
type CreatePinTxParams struct {
	UserID    uuid.UUID
	PinHash   []byte
	ExpiresAt time.Time
	// MaxPorHora es el tope de PIN emitidos en la última hora; al llegar a él no se crea ninguno.
	MaxPorHora int64
}

// ResetPasswordTxParams son los datos para cambiar la contraseña y cerrar
// el token de reset en una sola transacción.
type ResetPasswordTxParams struct {
	UserID uuid.UUID
	// PinID es la fila de recovery_pins que guarda el token de reset.
	PinID        uuid.UUID
	PasswordHash string
}

// ConRol es un usuario junto con el nombre de su rol, ya resuelto contra
// la tabla roles.
type ConRol struct {
	ID       uuid.UUID
	Email    *string
	Username string
	RolName  string
}

// GoogleLoginParams son los datos verificados del token de Google para
// LoginGoogleTx. Email ya viene normalizado.
type GoogleLoginParams struct {
	Sub, Email, Username, HD string
	DefaultRoleName          string
	DefaultRoleID            int
}

// GoogleLoginResult es lo que hizo LoginGoogleTx.
type GoogleLoginResult struct {
	User      sqlc.User
	RolNombre string
	Created   bool
	// Linked es true solo si esta petición insertó el vínculo con Google.
	Linked          bool
	PasswordRemoved bool
}

// candidatoVinculo es la cuenta local con el mismo email que el token, ya
// bloqueada, tal como la necesita decidirVinculo.
type candidatoVinculo struct {
	Suspendido bool
	RolNombre  string
	// RolDefault es el rol de los usuarios nuevos, que NewAuth resuelve de la
	// tabla roles; así usuario no necesita una constante de rol.
	RolDefault            string
	GoogleProviderID      *string
	PasswordChangedAtNulo bool
}

// emailAutoritativo dice si Google es la autoridad del email: una cuenta
// @gmail.com o una de Workspace (hd no vacío). Solo entonces el token prueba
// que el usuario controla el buzón. email ya viene normalizado.
func emailAutoritativo(email, hd string) bool {
	return strings.HasSuffix(email, "@gmail.com") || hd != ""
}

// decidirVinculo aplica, en orden, las reglas para vincular una cuenta local
// existente con la identidad de Google sub (paso 3 del plan técnico).
//
//   - yaVinculado: la cuenta ya está vinculada a este sub; no hay que escribir nada.
//   - quitarPassword: nadie probó nunca el control del buzón
//     (password_changed_at es NULL), así que el password pudo ponerlo otra
//     persona y hay que quitarlo al vincular.
func decidirVinculo(c candidatoVinculo, sub string) (yaVinculado bool, quitarPassword bool, err error) {
	if c.Suspendido {
		return false, false, ErrUsuarioSuspendido
	}
	if c.RolNombre != c.RolDefault {
		return false, false, ErrGoogleVinculoNoPermitido
	}
	if c.GoogleProviderID != nil {
		if *c.GoogleProviderID == sub {
			return true, false, nil
		}
		return false, false, ErrGoogleCuentaDistinta
	}
	return false, c.PasswordChangedAtNulo, nil
}

// maxUsernameRunes es el largo máximo de username, el mismo que valida Register.
const maxUsernameRunes = 100

// usernameDesdeGoogle arma el username de una cuenta nueva: el nombre de
// Google, o la parte del email antes de la @, recortado a maxUsernameRunes.
// Nunca devuelve vacío, porque username es NOT NULL y Register no acepta "".
func usernameDesdeGoogle(name, email string) string {
	u := strings.TrimSpace(name)
	if u == "" {
		u, _, _ = strings.Cut(email, "@")
		u = strings.TrimSpace(u)
	}
	if utf8.RuneCountInString(u) > maxUsernameRunes {
		u = string([]rune(u)[:maxUsernameRunes])
	}
	if u == "" {
		return "usuario"
	}
	return u
}

// estaSuspendido dice si state es Suspendido. NULL no lo es.
func estaSuspendido(state sqlc.NullUserState) bool {
	return state.Valid && state.UserState == sqlc.UserStateSuspendido
}
