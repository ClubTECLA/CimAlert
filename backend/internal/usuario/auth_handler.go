package usuario

import (
	"context"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/httperr"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// maxBcryptPasswordBytes es el límite de bcrypt: cualquier byte después del 72,
// así que una contraseña más larga debe rechazarse antes de llegar a bcrypt.
const maxBcryptPasswordBytes = 72

// Register crea un usuario con password local y devuelve un token de acceso
func (h *Handler) Register(ctx context.Context, request httpapi.RegisterRequestObject) (httpapi.RegisterResponseObject, error) {
	username := request.Body.Username
	password := request.Body.Password

	if username == "" || password == "" {
		return httpapi.Register400JSONResponse{Message: "username y password son requeridos"}, nil
	}
	if utf8.RuneCountInString(username) > 100 {
		return httpapi.Register400JSONResponse{Message: "username debe tener entre 1 y 100 caracteres"}, nil
	}
	if len(password) < 8 || len(password) > maxBcryptPasswordBytes {
		return httpapi.Register400JSONResponse{Message: "password debe tener entre 8 y 72 bytes"}, nil
	}

	user, t, err := h.auth.Register(ctx, string(request.Body.Email), username, password, verdad(request.Body.RememberMe))
	if err != nil {
		return nil, err
	}

	resp, err := h.entregar(ctx, user, t, esMovil(request.Body.Client))
	if err != nil {
		return nil, err
	}
	return httpapi.Register201JSONResponse(resp), nil
}

// Login valida el email y password, y devuelve un token de acceso firmado.
func (h *Handler) Login(ctx context.Context, request httpapi.LoginRequestObject) (httpapi.LoginResponseObject, error) {
	user, t, err := h.auth.Login(ctx, string(request.Body.Email), request.Body.Password, verdad(request.Body.RememberMe))
	if err != nil {
		return nil, err
	}

	resp, err := h.entregar(ctx, user, t, esMovil(request.Body.Client))
	if err != nil {
		return nil, err
	}
	return httpapi.Login200JSONResponse(resp), nil
}

// LoginGoogle inicia sesión con un ID token de Google. Responde 200 también
// cuando crea la cuenta; created distingue los dos casos.
func (h *Handler) LoginGoogle(ctx context.Context, request httpapi.LoginGoogleRequestObject) (httpapi.LoginGoogleResponseObject, error) {
	user, t, created, err := h.auth.LoginGoogle(ctx, request.Body.IdToken, verdad(request.Body.RememberMe))
	if err != nil {
		return nil, err
	}

	a, err := h.entregar(ctx, user, t, esMovil(request.Body.Client))
	if err != nil {
		return nil, err
	}
	// oapi-codegen genera un struct plano para el allOf de GoogleAuthResponse.
	return httpapi.LoginGoogle200JSONResponse{
		AccessToken:  a.AccessToken,
		TokenType:    a.TokenType,
		ExpiresIn:    a.ExpiresIn,
		RefreshToken: a.RefreshToken,
		User:         a.User,
		Created:      created,
	}, nil
}

// Retorna el usuario autenticado según el token de la cabecera Authorization.
func (h *Handler) Me(ctx context.Context, request httpapi.MeRequestObject) (httpapi.MeResponseObject, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, httperr.ErrNoAutenticado
	}

	user, err := h.auth.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return httpapi.Me200JSONResponse(toUsuarioMeResponse(user)), nil
}

// Refresh renueva la sesión con el refresh token del body (móvil) o de la
// cookie (web) y lo devuelve rotado por el mismo camino.
func (h *Handler) Refresh(ctx context.Context, request httpapi.RefreshRequestObject) (httpapi.RefreshResponseObject, error) {
	token, movil := refreshDe(request.Body, request.Params.RefreshToken)
	user, t, err := h.auth.Refresh(ctx, token)
	if errors.Is(err, ErrSesionInvalida) && !movil {
		// Una cookie que ya no sirve se borra, para que el web no la siga mandando.
		if errCookie := h.borrarCookie(ctx); errCookie != nil {
			return nil, errCookie
		}
	}
	if err != nil {
		return nil, err
	}

	resp, err := h.entregar(ctx, user, t, movil)
	if err != nil {
		return nil, err
	}
	return httpapi.Refresh200JSONResponse(resp), nil
}

// Logout revoca la sesión del refresh token (body o cookie) y borra la cookie.
func (h *Handler) Logout(ctx context.Context, request httpapi.LogoutRequestObject) (httpapi.LogoutResponseObject, error) {
	token, movil := refreshDe(request.Body, request.Params.RefreshToken)
	if err := h.auth.Logout(ctx, token); err != nil {
		return nil, err
	}
	if !movil {
		if err := h.borrarCookie(ctx); err != nil {
			return nil, err
		}
	}
	return httpapi.Logout204Response{}, nil
}

// SolicitarRecuperacion pide un PIN de recuperación. Siempre responde el mismo 202.
func (h *Handler) SolicitarRecuperacion(ctx context.Context, request httpapi.SolicitarRecuperacionRequestObject) (httpapi.SolicitarRecuperacionResponseObject, error) {
	if err := h.rec.Solicitar(ctx, string(request.Body.Email)); err != nil {
		return nil, err
	}

	return httpapi.SolicitarRecuperacion202Response{}, nil
}

// VerificarRecuperacion valida el PIN y devuelve el token de reset de un solo uso.
func (h *Handler) VerificarRecuperacion(ctx context.Context, request httpapi.VerificarRecuperacionRequestObject) (httpapi.VerificarRecuperacionResponseObject, error) {
	token, err := h.rec.Verificar(ctx, string(request.Body.Email), request.Body.Pin)
	if err != nil {
		return nil, err
	}

	return httpapi.VerificarRecuperacion200JSONResponse{ResetToken: token}, nil
}

// RestablecerContrasena cambia la contraseña con el token de reset; responde 204 sin token de sesión.
func (h *Handler) RestablecerContrasena(ctx context.Context, request httpapi.RestablecerContrasenaRequestObject) (httpapi.RestablecerContrasenaResponseObject, error) {
	// El contrato solo pone el mínimo; el máximo es el de bcrypt, igual que en el registro.
	if len(request.Body.NewPassword) > maxBcryptPasswordBytes {
		return httpapi.RestablecerContrasena400JSONResponse{Message: "password debe tener entre 8 y 72 bytes"}, nil
	}

	if err := h.rec.Restablecer(ctx, request.Body.ResetToken, request.Body.NewPassword); err != nil {
		return nil, err
	}

	return httpapi.RestablecerContrasena204Response{}, nil
}

// entregar arma la respuesta de sesión: en el móvil el refresh token va en
// el body; en el web, en la cookie. Sin refresh token (ventana de gracia) no
// toca la cookie.
func (h *Handler) entregar(ctx context.Context, user sqlc.User, t Tokens, movil bool) (httpapi.AuthResponse, error) {
	resp := toAuthResponse(user, t.Access, t.TTL)
	switch {
	case t.Refresh == "":
	case movil:
		resp.RefreshToken = &t.Refresh
	default:
		// Sin remember_me es cookie de sesión (sin Max-Age): se borra al cerrar el navegador.
		maxAge := 0
		if t.RememberMe {
			maxAge = int(time.Until(t.Expira).Seconds())
		}
		if err := h.ponerCookie(ctx, t.Refresh, maxAge); err != nil {
			return httpapi.AuthResponse{}, err
		}
	}
	return resp, nil
}

func (h *Handler) borrarCookie(ctx context.Context) error {
	return h.ponerCookie(ctx, "", -1)
}

// ponerCookie escribe la cookie del refresh token. Los handlers strict reciben
// el *gin.Context como context.Context; es la única forma de poner una cookie.
func (h *Handler) ponerCookie(ctx context.Context, valor string, maxAge int) error {
	gc, ok := ctx.(*gin.Context)
	if !ok {
		return errors.New("ponerCookie: el contexto no es un *gin.Context")
	}
	gc.SetSameSite(http.SameSiteStrictMode)
	gc.SetCookie(cookieRefresh, valor, maxAge, h.cookie.Path, "", h.cookie.Secure, true)
	return nil
}

// refreshDe devuelve el refresh token del body (móvil) o, sin body, el de la cookie.
func refreshDe(body *httpapi.RefreshRequest, cookie *string) (token string, movil bool) {
	if body != nil {
		return body.RefreshToken, true
	}
	if cookie != nil {
		return *cookie, false
	}
	return "", false
}

func esMovil(c *httpapi.Cliente) bool { return c != nil && *c == httpapi.Mobile }

func verdad(b *bool) bool { return b != nil && *b }

// toAuthResponse convierte un sqlc.User y un token en la respuesta de login/register.
func toAuthResponse(user sqlc.User, token string, ttl time.Duration) httpapi.AuthResponse {
	return httpapi.AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
		User:        toUsuario(user),
	}
}

// toUsuario convierte un sqlc.User en el Usuario que anida AuthResponse.
func toUsuario(user sqlc.User) httpapi.Usuario {
	var email openapi_types.Email
	if user.Email != nil {
		email = openapi_types.Email(*user.Email)
	}

	return httpapi.Usuario{
		Id:        user.ID,
		Email:     email,
		Username:  user.Username,
		RolId:     user.RolID,
		CreatedAt: user.CreatedAt,
	}
}

// toUsuarioMeResponse convierte un ConRol en la respuesta de
// /auth/me, que expone el nombre del rol en vez de su id.
func toUsuarioMeResponse(user ConRol) httpapi.UsuarioMeResponse {
	var email openapi_types.Email
	if user.Email != nil {
		email = openapi_types.Email(*user.Email)
	}

	return httpapi.UsuarioMeResponse{
		Id:       user.ID,
		Email:    email,
		Username: user.Username,
		RolName:  user.RolName,
	}
}
