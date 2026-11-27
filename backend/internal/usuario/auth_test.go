package usuario

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/googleid"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// errUniqueViolation imita lo que devuelve Postgres al romper el UNIQUE de
// users.email, para ejercer la traducción a ErrEmailTaken.
var errUniqueViolation = &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}

// fakeAuthStore implementa authStore en memoria.
// rolName es el rol que devuelve GetUserWithRolByID; vacío = "ciudadano".
type fakeAuthStore struct {
	byEmail       map[string]sqlc.User
	defaultRoleID int
	rolName       string

	// LoginGoogleTx devuelve googleRes o googleErr y guarda lo que recibió.
	googleRes     GoogleLoginResult
	googleErr     error
	googleParams  GoogleLoginParams
	googleLlamado bool

	// sesiones son las filas de refresh_tokens (ver sesion_test.go).
	sesiones []*sqlc.RefreshToken
	// errSesion, si no es nil, lo devuelve GetSesionPorTokenHash (base caída).
	errSesion error
	// rivalRota hace que el próximo RotarSesion rote primero con otro token y
	// devuelva 0 filas, como si otra petición hubiera ganado la carrera.
	rivalRota bool
}

func (f *fakeAuthStore) LoginGoogleTx(_ context.Context, p GoogleLoginParams) (GoogleLoginResult, error) {
	f.googleLlamado = true
	f.googleParams = p
	return f.googleRes, f.googleErr
}

func (f *fakeAuthStore) CreateUserWithLocalProvider(_ context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	email := ""
	if arg.Email != nil {
		email = *arg.Email
	}
	if _, exists := f.byEmail[email]; exists {
		return sqlc.User{}, errUniqueViolation
	}
	u := sqlc.User{
		ID:           uuid.New(),
		Email:        arg.Email,
		Username:     arg.Username,
		RolID:        arg.RolID,
		PasswordHash: arg.PasswordHash,
		CreatedAt:    time.Now(),
	}
	f.byEmail[email] = u
	return u, nil
}

func (f *fakeAuthStore) GetUserByEmail(_ context.Context, email *string) (sqlc.User, error) {
	if email != nil {
		if u, ok := f.byEmail[*email]; ok {
			return u, nil
		}
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (f *fakeAuthStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (f *fakeAuthStore) GetUserWithRolByID(_ context.Context, id uuid.UUID) (sqlc.GetUserWithRolByIDRow, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return sqlc.GetUserWithRolByIDRow{
				ID:       u.ID,
				Email:    u.Email,
				Username: u.Username,
				RolID:    u.RolID,
				RolName:  f.rol(),
			}, nil
		}
	}
	return sqlc.GetUserWithRolByIDRow{}, pgx.ErrNoRows
}

func (f *fakeAuthStore) GetDefaultRole(_ context.Context) (sqlc.Role, error) {
	return sqlc.Role{ID: f.defaultRoleID, Nombre: "ciudadano"}, nil
}

func TestAuthService_RegisterAndLogin(t *testing.T) {
	store := &fakeAuthStore{byEmail: map[string]sqlc.User{}, defaultRoleID: 1}
	ctx := context.Background()
	svc, err := NewAuth(ctx, store, []byte("test-secret-at-least-32-bytes!!"), time.Hour)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}

	user, tk, err := svc.Register(ctx, "  Test@Example.com  ", "tester", "s3cretpw", false)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if tk.Access == "" || tk.Refresh == "" || tk.TTL != time.Hour {
		t.Fatalf("Register() tokens = %+v, want access y refresh no vacíos y ttl=1h", tk)
	}
	if user.Email == nil || *user.Email != "test@example.com" {
		t.Fatalf("Register() email = %v, want normalized \"test@example.com\"", user.Email)
	}
	if user.RolID != 1 {
		t.Fatalf("Register() rolID = %d, want 1 (rol por defecto)", user.RolID)
	}

	if _, _, err := svc.Register(ctx, "test@example.com", "otro", "s3cretpw", false); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("Register() with duplicate email error = %v, want ErrEmailTaken", err)
	}

	if _, _, err := svc.Login(ctx, "test@example.com", "wrong-password", false); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() with wrong password error = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := svc.Login(ctx, "nobody@example.com", "whatever", false); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() with unknown email error = %v, want ErrInvalidCredentials", err)
	}
	if _, tk, err := svc.Login(ctx, "test@example.com", "s3cretpw", false); err != nil || tk.Access == "" {
		t.Fatalf("Login() with correct credentials error = %v, token = %q", err, tk.Access)
	}

	if _, err := svc.GetUser(ctx, uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GetUser() with unknown id error = %v, want ErrUserNotFound", err)
	}
}

func (f *fakeAuthStore) rol() string {
	if f.rolName == "" {
		return "ciudadano"
	}
	return f.rolName
}

// rolDelToken valida el token con el secreto y devuelve su claim "rol", el
// mismo que lee middleware.Auth.
func rolDelToken(t *testing.T, token string, secret []byte) string {
	t.Helper()

	var claims tokenClaims
	if _, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return secret, nil }); err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	return claims.Rol
}

func TestAuthService_TokenLlevaRol(t *testing.T) {
	store := &fakeAuthStore{byEmail: map[string]sqlc.User{}, defaultRoleID: 1}
	secret := []byte("test-secret-at-least-32-bytes!!")
	ctx := context.Background()
	svc, err := NewAuth(ctx, store, secret, time.Hour)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}

	_, tk, err := svc.Register(ctx, "test@example.com", "tester", "s3cretpw", false)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if rol := rolDelToken(t, tk.Access, secret); rol != "ciudadano" {
		t.Fatalf("Register() token rol = %q, want \"ciudadano\" (rol por defecto)", rol)
	}

	// Login lee el rol actual de la base, no asume el rol por defecto.
	store.rolName = "admin"
	_, tk, err = svc.Login(ctx, "test@example.com", "s3cretpw", false)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if rol := rolDelToken(t, tk.Access, secret); rol != "admin" {
		t.Fatalf("Login() token rol = %q, want \"admin\"", rol)
	}
}

const secretoGoogle = "test-secret-at-least-32-bytes!!"

// fakeGoogleVerifier devuelve los claims o el error configurados.
type fakeGoogleVerifier struct {
	claims googleid.Claims
	err    error
}

func (f fakeGoogleVerifier) Verificar(context.Context, string) (googleid.Claims, error) {
	return f.claims, f.err
}

// fakeMailer avisa por enviado cada vez que se manda el aviso de vínculo,
// porque LoginGoogle lo manda en segundo plano.
type fakeMailer struct {
	NoopMailer
	enviado chan string
}

func (f *fakeMailer) EnviarAvisoCuentaVinculadaGoogle(_ context.Context, _ uuid.UUID, email string) error {
	f.enviado <- email
	return nil
}

// claimsGoogle son claims válidos de una cuenta @gmail.com.
func claimsGoogle() googleid.Claims {
	return googleid.Claims{Sub: "sub-1", Email: "ana@gmail.com", Name: "Ana", Aud: "web-client", EmailVerified: true}
}

// authConGoogle arma Auth con el verificador y el mailer dados; sin
// verificador queda sin Google.
func authConGoogle(t *testing.T, store *fakeAuthStore, v googleVerifier, m Mailer) *Auth {
	t.Helper()
	var opts []AuthOption
	if v != nil {
		opts = append(opts, WithGoogle(v, m))
	}
	a, err := NewAuth(context.Background(), store, []byte(secretoGoogle), time.Hour, opts...)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	return a
}

// capturarLog reemplaza slog.Default por uno que escribe JSON en el buffer
// devuelto, y lo restaura al terminar el test.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &buf
}

// registrosLoginGoogle devuelve los registros "login google" del buffer.
func registrosLoginGoogle(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, linea := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if linea == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(linea), &r); err != nil {
			t.Fatalf("registro de log no es JSON: %q", linea)
		}
		if r["msg"] == "login google" {
			out = append(out, r)
		}
	}
	return out
}

func TestLoginGoogle_Errores(t *testing.T) {
	tests := []struct {
		name       string
		verifier   googleVerifier
		storeErr   error
		wantErr    error
		wantReason string
		wantStore  bool
	}{
		{name: "sin verificador", wantErr: ErrGoogleNoConfigurado, wantReason: "google_no_configurado"},
		{name: "token inválido", verifier: fakeGoogleVerifier{err: googleid.ErrTokenInvalido}, wantErr: ErrTokenGoogleInvalido, wantReason: "token_invalido"},
		{name: "otro error del verificador", verifier: fakeGoogleVerifier{err: errors.New("x")}, wantErr: ErrTokenGoogleInvalido, wantReason: "token_invalido"},
		{name: "claves no disponibles", verifier: fakeGoogleVerifier{err: googleid.ErrClavesNoDisponibles}, wantErr: ErrGoogleNoDisponible, wantReason: "claves_no_disponibles"},
		{name: "email no verificado", verifier: fakeGoogleVerifier{claims: googleid.Claims{Sub: "s", Email: "ana@gmail.com"}}, wantErr: ErrEmailGoogleNoVerificado, wantReason: "email_no_verificado"},
		{name: "email vacío", verifier: fakeGoogleVerifier{claims: googleid.Claims{Sub: "s", EmailVerified: true}}, wantErr: ErrEmailGoogleNoVerificado, wantReason: "email_no_verificado"},
		{name: "email no autoritativo", verifier: fakeGoogleVerifier{claims: claimsGoogle()}, storeErr: ErrGoogleEmailNoAdmitido, wantErr: ErrGoogleEmailNoAdmitido, wantReason: "email_no_autoritativo", wantStore: true},
		{name: "suspendido", verifier: fakeGoogleVerifier{claims: claimsGoogle()}, storeErr: ErrUsuarioSuspendido, wantErr: ErrUsuarioSuspendido, wantReason: "suspendido", wantStore: true},
		{name: "cuenta distinta", verifier: fakeGoogleVerifier{claims: claimsGoogle()}, storeErr: ErrGoogleCuentaDistinta, wantErr: ErrGoogleCuentaDistinta, wantReason: "cuenta_distinta", wantStore: true},
		{name: "vínculo no permitido", verifier: fakeGoogleVerifier{claims: claimsGoogle()}, storeErr: ErrGoogleVinculoNoPermitido, wantErr: ErrGoogleVinculoNoPermitido, wantReason: "vinculo_no_permitido", wantStore: true},
		{name: "error interno", verifier: fakeGoogleVerifier{claims: claimsGoogle()}, storeErr: errors.New("db caída"), wantReason: "interno", wantStore: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := capturarLog(t)
			store := &fakeAuthStore{googleErr: tt.storeErr}
			a := authConGoogle(t, store, tt.verifier, &fakeMailer{enviado: make(chan string, 1)})

			_, _, _, err := a.LoginGoogle(context.Background(), "token", false)
			if err == nil {
				t.Fatal("LoginGoogle() error = nil, quería un error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("LoginGoogle() error = %v, quería %v", err, tt.wantErr)
			}
			if store.googleLlamado != tt.wantStore {
				t.Errorf("store llamado = %v, quería %v", store.googleLlamado, tt.wantStore)
			}

			regs := registrosLoginGoogle(t, buf)
			if len(regs) != 1 {
				t.Fatalf("registros de log = %d, quería 1", len(regs))
			}
			if regs[0]["outcome"] != "rejected" || regs[0]["reason"] != tt.wantReason {
				t.Errorf("log outcome/reason = %v/%v, quería rejected/%s", regs[0]["outcome"], regs[0]["reason"], tt.wantReason)
			}
			wantLevel := "INFO"
			if tt.wantReason == "claves_no_disponibles" {
				wantLevel = "WARN"
			}
			if regs[0]["level"] != wantLevel {
				t.Errorf("log level = %v, quería %s", regs[0]["level"], wantLevel)
			}
		})
	}
}

func TestLoginGoogle_Exito(t *testing.T) {
	changedAt := time.Date(2026, 11, 24, 10, 0, 0, 123_000, time.UTC)
	usuario := sqlc.User{ID: uuid.New(), Username: "Ana", PasswordChangedAt: &changedAt}

	tests := []struct {
		name        string
		res         GoogleLoginResult
		wantOutcome string
		wantLevel   string
		wantAviso   bool
	}{
		{name: "sesión", res: GoogleLoginResult{User: usuario, RolNombre: "ciudadano"}, wantOutcome: "logged_in", wantLevel: "INFO"},
		{name: "alta", res: GoogleLoginResult{User: usuario, RolNombre: "ciudadano", Created: true}, wantOutcome: "registered", wantLevel: "INFO"},
		{name: "vínculo sin quitar password", res: GoogleLoginResult{User: usuario, RolNombre: "ciudadano", Linked: true}, wantOutcome: "linked", wantLevel: "WARN"},
		{name: "vínculo quitando password", res: GoogleLoginResult{User: usuario, RolNombre: "ciudadano", Linked: true, PasswordRemoved: true}, wantOutcome: "linked", wantLevel: "WARN", wantAviso: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := capturarLog(t)
			store := &fakeAuthStore{googleRes: tt.res}
			mailer := &fakeMailer{enviado: make(chan string, 1)}
			claims := claimsGoogle()
			claims.Email = "  Ana@Gmail.COM "
			claims.HD = "empresa.mx"
			a := authConGoogle(t, store, fakeGoogleVerifier{claims: claims}, mailer)

			user, tk, created, err := a.LoginGoogle(context.Background(), "token", false)
			if err != nil {
				t.Fatalf("LoginGoogle() error = %v", err)
			}
			if user.ID != usuario.ID || tk.TTL != time.Hour || created != tt.res.Created {
				t.Errorf("LoginGoogle() user/ttl/created = %v/%v/%v", user.ID, tk.TTL, created)
			}

			// Lo que llega al store: email normalizado, sub y hd tal cual.
			p := store.googleParams
			if p.Email != "ana@gmail.com" || p.Sub != "sub-1" || p.HD != "empresa.mx" || p.Username != "Ana" || p.DefaultRoleName != "ciudadano" {
				t.Errorf("params al store = %+v", p)
			}

			// El token sale de la fila que devolvió el store.
			var tc tokenClaims
			if _, err := jwt.ParseWithClaims(tk.Access, &tc, func(*jwt.Token) (any, error) { return []byte(secretoGoogle), nil }); err != nil {
				t.Fatalf("token inválido: %v", err)
			}
			if tc.Subject != usuario.ID.String() || tc.Rol != "ciudadano" || tc.PwdAt != middleware.VersionCredencial(usuario.PasswordChangedAt) {
				t.Errorf("claims del token = sub %q rol %q pwd_at %q", tc.Subject, tc.Rol, tc.PwdAt)
			}

			regs := registrosLoginGoogle(t, buf)
			if len(regs) != 1 {
				t.Fatalf("registros de log = %d, quería 1", len(regs))
			}
			r := regs[0]
			if r["outcome"] != tt.wantOutcome || r["level"] != tt.wantLevel || r["aud"] != "web-client" {
				t.Errorf("log = %v, quería outcome %s level %s aud web-client", r, tt.wantOutcome, tt.wantLevel)
			}
			if _, ok := r["reason"]; ok {
				t.Errorf("log con reason en un éxito: %v", r)
			}
			if tt.wantOutcome == "linked" && r["password_removed"] != tt.res.PasswordRemoved {
				t.Errorf("log password_removed = %v, quería %v", r["password_removed"], tt.res.PasswordRemoved)
			}

			// El aviso se manda en segundo plano: se espera hasta 100 ms.
			select {
			case email := <-mailer.enviado:
				if !tt.wantAviso {
					t.Errorf("se mandó aviso a %q, no debía", email)
				} else if email != "ana@gmail.com" {
					t.Errorf("aviso a %q, quería ana@gmail.com", email)
				}
			case <-time.After(100 * time.Millisecond):
				if tt.wantAviso {
					t.Error("no se mandó el aviso de vínculo")
				}
			}
		})
	}
}

func TestLogin_Suspendido(t *testing.T) {
	store := &fakeAuthStore{byEmail: map[string]sqlc.User{}, defaultRoleID: 1}
	ctx := context.Background()
	svc, err := NewAuth(ctx, store, []byte(secretoGoogle), time.Hour)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	if _, _, err := svc.Register(ctx, "ana@example.com", "ana", "s3cretpw", false); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	u := store.byEmail["ana@example.com"]
	u.State = sqlc.NullUserState{UserState: sqlc.UserStateSuspendido, Valid: true}
	store.byEmail["ana@example.com"] = u

	if _, _, err := svc.Login(ctx, "ana@example.com", "s3cretpw", false); !errors.Is(err, ErrUsuarioSuspendido) {
		t.Errorf("Login() con el password correcto error = %v, quería ErrUsuarioSuspendido", err)
	}
	// Sin el password correcto no se revela la suspensión.
	if _, _, err := svc.Login(ctx, "ana@example.com", "incorrecto", false); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() con un password incorrecto error = %v, quería ErrInvalidCredentials", err)
	}
}
