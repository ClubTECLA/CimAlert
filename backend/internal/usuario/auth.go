package usuario

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/googleid"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash se compara contra el password recibido cuando el email
// no existe o el usuario no tiene password local (login con Google, p. ej.).
var dummyPasswordHash = func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("tijuana-reporta-dummy"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}()

// authStore es lo que Auth necesita de Store; los tests lo cumplen con un
// falso en memoria.
type authStore interface {
	CreateUserWithLocalProvider(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByEmail(ctx context.Context, email *string) (sqlc.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error)
	GetDefaultRole(ctx context.Context) (sqlc.Role, error)
	GetUserWithRolByID(ctx context.Context, id uuid.UUID) (sqlc.GetUserWithRolByIDRow, error)
	LoginGoogleTx(ctx context.Context, p GoogleLoginParams) (GoogleLoginResult, error)
}

// googleVerifier verifica el ID token de Google; lo cumple googleid.Verificador
// y, en los tests, un falso.
type googleVerifier interface {
	Verificar(ctx context.Context, raw string) (googleid.Claims, error)
}

type Auth struct {
	store           authStore
	secret          []byte
	ttl             time.Duration
	defaultRoleID   int
	defaultRoleName string

	// google es nil cuando no hay GOOGLE_CLIENT_IDS: LoginGoogle responde 503.
	google googleVerifier
	// mailer manda el aviso de vínculo con Google.
	mailer Mailer
}

// AuthOption ajusta NewAuth.
type AuthOption func(*Auth)

// WithGoogle habilita el inicio de sesión con Google.
func WithGoogle(v googleVerifier, m Mailer) AuthOption {
	return func(a *Auth) {
		a.google = v
		a.mailer = m
	}
}

// NewAuth resuelve el id del rol "ciudadano" contra la base una sola
// vez, en vez de asumir un id fijo.
func NewAuth(ctx context.Context, store authStore, secret []byte, ttl time.Duration, opts ...AuthOption) (*Auth, error) {
	defaultRole, err := store.GetDefaultRole(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolving default role: %w", err)
	}
	a := &Auth{store: store, secret: secret, ttl: ttl, defaultRoleID: defaultRole.ID, defaultRoleName: defaultRole.Nombre}
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}

// Register crea el usuario con password local y devuelve el token de acceso
// ya firmado, junto con su tiempo de vida.
func (s *Auth) Register(ctx context.Context, email, username, password string) (sqlc.User, string, time.Duration, error) {
	email = normalizeEmail(email)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return sqlc.User{}, "", 0, err
	}
	hashed := string(hash)

	user, err := s.store.CreateUserWithLocalProvider(ctx, sqlc.CreateUserParams{
		Email:        &email,
		Username:     username,
		RolID:        s.defaultRoleID,
		PasswordHash: &hashed,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return sqlc.User{}, "", 0, ErrEmailTaken
		}
		return sqlc.User{}, "", 0, err
	}

	token, err := s.issueToken(user, s.defaultRoleName)
	if err != nil {
		return sqlc.User{}, "", 0, err
	}
	return user, token, s.ttl, nil
}

// Login valida el password contra el hash guardado. Si el email no existe o
// el usuario no tiene password local, compara igual contra un hash dummy
// para no revelar cuál de los dos casos sucedió.
func (s *Auth) Login(ctx context.Context, email, password string) (sqlc.User, string, time.Duration, error) {
	email = normalizeEmail(email)

	user, err := s.store.GetUserByEmail(ctx, &email)
	notFound := db.IsNotFound(err)
	if err != nil && !notFound {
		return sqlc.User{}, "", 0, err
	}

	hash := dummyPasswordHash
	if user.PasswordHash != nil {
		hash = *user.PasswordHash
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return sqlc.User{}, "", 0, ErrInvalidCredentials
	}
	if notFound {
		return sqlc.User{}, "", 0, ErrInvalidCredentials
	}
	// Después de bcrypt: solo quien sabe el password se entera de la suspensión.
	if estaSuspendido(user.State) {
		return sqlc.User{}, "", 0, ErrUsuarioSuspendido
	}

	row, err := s.store.GetUserWithRolByID(ctx, user.ID)
	if err != nil {
		return sqlc.User{}, "", 0, err
	}

	token, err := s.issueToken(user, row.RolName)
	if err != nil {
		return sqlc.User{}, "", 0, err
	}
	return user, token, s.ttl, nil
}

// razonesGoogle es el campo reason del log de LoginGoogle para cada error.
var razonesGoogle = []struct {
	err    error
	reason string
}{
	{ErrGoogleNoConfigurado, "google_no_configurado"},
	{ErrTokenGoogleInvalido, "token_invalido"},
	{ErrGoogleNoDisponible, "claves_no_disponibles"},
	{ErrEmailGoogleNoVerificado, "email_no_verificado"},
	{ErrGoogleEmailNoAdmitido, "email_no_autoritativo"},
	{ErrUsuarioSuspendido, "suspendido"},
	{ErrGoogleCuentaDistinta, "cuenta_distinta"},
	{ErrGoogleVinculoNoPermitido, "vinculo_no_permitido"},
}

func razonGoogle(err error) string {
	for _, r := range razonesGoogle {
		if errors.Is(err, r.err) {
			return r.reason
		}
	}
	return "interno"
}

// LoginGoogle inicia sesión con un ID token de Google: lo verifica, busca,
// vincula o crea la cuenta y devuelve el token de acceso. El bool es true si
// creó la cuenta. Escribe un solo registro de log por llamada.
func (s *Auth) LoginGoogle(ctx context.Context, idToken string) (_ sqlc.User, _ string, _ time.Duration, _ bool, err error) {
	var (
		claims googleid.Claims
		res    GoogleLoginResult
	)
	defer func() {
		attrs := []slog.Attr{slog.String("aud", claims.Aud)}
		level := slog.LevelInfo
		switch {
		case err != nil:
			reason := razonGoogle(err)
			attrs = append(attrs, slog.String("outcome", "rejected"), slog.String("reason", reason))
			if reason == "claves_no_disponibles" {
				level = slog.LevelWarn
			}
		case res.Created:
			attrs = append(attrs, slog.String("outcome", "registered"))
		case res.Linked:
			// WARN: un vínculo puede ser una toma de cuenta; conviene verlo.
			attrs = append(attrs, slog.String("outcome", "linked"), slog.Bool("password_removed", res.PasswordRemoved))
			level = slog.LevelWarn
		default:
			attrs = append(attrs, slog.String("outcome", "logged_in"))
		}
		slog.LogAttrs(ctx, level, "login google", attrs...)
	}()

	if s.google == nil {
		return sqlc.User{}, "", 0, false, ErrGoogleNoConfigurado
	}

	claims, err = s.google.Verificar(ctx, idToken)
	if err != nil {
		claims = googleid.Claims{}
		if errors.Is(err, googleid.ErrClavesNoDisponibles) {
			return sqlc.User{}, "", 0, false, ErrGoogleNoDisponible
		}
		return sqlc.User{}, "", 0, false, ErrTokenGoogleInvalido
	}
	if !claims.EmailVerified || claims.Email == "" {
		return sqlc.User{}, "", 0, false, ErrEmailGoogleNoVerificado
	}

	email := normalizeEmail(claims.Email)
	res, err = s.store.LoginGoogleTx(ctx, GoogleLoginParams{
		Sub:             claims.Sub,
		Email:           email,
		Username:        usernameDesdeGoogle(claims.Name, email),
		HD:              claims.HD,
		DefaultRoleID:   s.defaultRoleID,
		DefaultRoleName: s.defaultRoleName,
	})
	if err != nil {
		return sqlc.User{}, "", 0, false, err
	}

	// Siempre de la fila que devolvió el store: si se quitó el password, su
	// password_changed_at nuevo va en el pwd_at del token.
	token, err := s.issueToken(res.User, res.RolNombre)
	if err != nil {
		return sqlc.User{}, "", 0, false, err
	}

	if res.PasswordRemoved {
		userID := res.User.ID
		enviarEnSegundoPlano(ctx, userID, "aviso de vínculo con Google", func(ctx context.Context) error {
			return s.mailer.EnviarAvisoCuentaVinculadaGoogle(ctx, userID, email)
		})
	}
	return res.User, token, s.ttl, res.Created, nil
}

// GetUser devuelve el usuario con su rol según el id.
func (s *Auth) GetUser(ctx context.Context, id uuid.UUID) (ConRol, error) {
	row, err := s.store.GetUserWithRolByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return ConRol{}, ErrUserNotFound
		}
		return ConRol{}, err
	}

	return ConRol{
		ID:       row.ID,
		Email:    row.Email,
		Username: row.Username,
		RolName:  row.RolName,
	}, nil
}

// tokenClaims son los claims del access token. middleware.Auth lee "sub" y
// "rol"; si cambias un nombre aquí, cámbialo allá también.
type tokenClaims struct {
	Rol string `json:"rol"`
	// PwdAt es middleware.VersionCredencial(user.PasswordChangedAt), leído en
	// la misma fila que el hash validado; Auth exige que siga igual al actual.
	PwdAt string `json:"pwd_at,omitempty"`
	jwt.RegisteredClaims
}

// issueToken genera un JWT firmado con el id del usuario, su rol, la versión
// de su contraseña y la fecha de expiración.
func (s *Auth) issueToken(user sqlc.User, rol string) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		Rol:   rol,
		PwdAt: middleware.VersionCredencial(user.PasswordChangedAt),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// normalizeEmail convierte el email a minúsculas y le quita espacios al inicio y al final.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
