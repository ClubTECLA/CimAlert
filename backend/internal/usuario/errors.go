package usuario

import (
	"errors"
	"net/http"

	"github.com/ClubTECLA/cimalert/backend/internal/httperr"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	// ErrCodigoInvalido cubre todo fallo de verificación, para no revelar
	// si la cuenta existe ni el estado del PIN.
	ErrCodigoInvalido = errors.New("invalid or expired recovery code")

	// Inicio de sesión con Google.
	ErrTokenGoogleInvalido     = errors.New("google token invalid")
	ErrEmailGoogleNoVerificado = errors.New("google email not verified")
	// ErrGoogleEmailNoAdmitido: Google no es la autoridad del email (no es
	// @gmail.com ni trae hd), así que no prueba que el usuario lo controle.
	ErrGoogleEmailNoAdmitido    = errors.New("google email not authoritative")
	ErrUsuarioSuspendido        = errors.New("user suspended")
	ErrGoogleCuentaDistinta     = errors.New("google account mismatch")
	ErrGoogleVinculoNoPermitido = errors.New("google link not allowed")
	ErrGoogleNoConfigurado      = errors.New("google login not configured")
	ErrGoogleNoDisponible       = errors.New("google keys unavailable")

	// ErrSesionInvalida cubre todo rechazo del refresh token, sin decir por qué.
	ErrSesionInvalida = errors.New("invalid or expired session")
)

// ReglasHTTP dice con qué status y mensaje responde la API cada error de
// arriba. Al agregar un Err…, agrégale aquí su regla.
var ReglasHTTP = []httperr.Regla{
	{Err: ErrEmailTaken, Status: http.StatusConflict, Message: "el email ya está registrado"},
	{Err: ErrInvalidCredentials, Status: http.StatusUnauthorized, Message: "credenciales invalidas"},
	{Err: ErrUserNotFound, Status: http.StatusUnauthorized, Message: "no autenticado"},
	{Err: ErrCodigoInvalido, Status: http.StatusBadRequest, Message: "código inválido o expirado"},
	// Los mensajes de Google son parte del contrato: los clientes los muestran.
	{Err: ErrTokenGoogleInvalido, Status: http.StatusUnauthorized, Message: "token de Google inválido"},
	{Err: ErrEmailGoogleNoVerificado, Status: http.StatusUnauthorized, Message: "email de Google no verificado"},
	{Err: ErrGoogleEmailNoAdmitido, Status: http.StatusForbidden, Message: "este email de Google no se puede usar aquí; regístrate o inicia sesión con email y contraseña"},
	{Err: ErrUsuarioSuspendido, Status: http.StatusForbidden, Message: "usuario suspendido"},
	{Err: ErrGoogleCuentaDistinta, Status: http.StatusConflict, Message: "la cuenta ya está vinculada a otra cuenta de Google"},
	{Err: ErrGoogleVinculoNoPermitido, Status: http.StatusConflict, Message: "esta cuenta no puede vincularse con Google; inicia sesión con tu contraseña"},
	{Err: ErrGoogleNoConfigurado, Status: http.StatusServiceUnavailable, Message: "login con Google no configurado"},
	{Err: ErrGoogleNoDisponible, Status: http.StatusServiceUnavailable, Message: "login con Google no disponible"},
	{Err: ErrSesionInvalida, Status: http.StatusUnauthorized, Message: "sesión inválida o expirada"},
}
