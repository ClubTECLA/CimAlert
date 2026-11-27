package usuario

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Métodos de sesión del fakeAuthStore: imitan las consultas de sesion.sql.

func (f *fakeAuthStore) CreateSesion(_ context.Context, arg sqlc.CreateSesionParams) error {
	f.sesiones = append(f.sesiones, &sqlc.RefreshToken{
		ID: uuid.New(), UserID: arg.UserID, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt,
		PwdAt: arg.PwdAt, RememberMe: arg.RememberMe, LastUsedAt: arg.Ahora, CreatedAt: arg.Ahora,
	})
	return nil
}

func (f *fakeAuthStore) GetSesionPorTokenHash(_ context.Context, hash string) (sqlc.RefreshToken, error) {
	if f.errSesion != nil {
		return sqlc.RefreshToken{}, f.errSesion
	}
	for _, s := range f.sesiones {
		if s.TokenHash == hash || (s.PrevTokenHash != nil && *s.PrevTokenHash == hash) {
			return *s, nil
		}
	}
	return sqlc.RefreshToken{}, pgx.ErrNoRows
}

func (f *fakeAuthStore) GetUserConRolPorID(_ context.Context, id uuid.UUID) (sqlc.GetUserConRolPorIDRow, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return sqlc.GetUserConRolPorIDRow{User: u, RolName: f.rol()}, nil
		}
	}
	return sqlc.GetUserConRolPorIDRow{}, pgx.ErrNoRows
}

func (f *fakeAuthStore) RotarSesion(_ context.Context, arg sqlc.RotarSesionParams) (int64, error) {
	for _, s := range f.sesiones {
		if s.ID == arg.ID && s.TokenHash == arg.Anterior && s.RevokedAt == nil {
			prev := s.TokenHash
			if f.rivalRota {
				f.rivalRota = false
				s.PrevTokenHash, s.TokenHash, s.LastUsedAt = &prev, hashRefresh("token-del-rival"), arg.Ahora
				return 0, nil
			}
			s.PrevTokenHash, s.TokenHash, s.ExpiresAt, s.LastUsedAt = &prev, arg.Nuevo, arg.ExpiresAt, arg.Ahora
			return 1, nil
		}
	}
	return 0, nil
}

func (f *fakeAuthStore) RevocarSesionPorID(_ context.Context, id uuid.UUID) error {
	for _, s := range f.sesiones {
		if s.ID == id && s.RevokedAt == nil {
			now := time.Now()
			s.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeAuthStore) RevocarSesionPorTokenHash(_ context.Context, hash string) error {
	for _, s := range f.sesiones {
		if s.TokenHash == hash || (s.PrevTokenHash != nil && *s.PrevTokenHash == hash) {
			return f.RevocarSesionPorID(context.Background(), s.ID)
		}
	}
	return nil
}

// sesionAbierta registra un usuario y devuelve el servicio, el store y su refresh token.
func sesionAbierta(t *testing.T, rememberMe bool) (*Auth, *fakeAuthStore, string) {
	t.Helper()
	store := &fakeAuthStore{byEmail: map[string]sqlc.User{}, defaultRoleID: 1}
	svc, err := NewAuth(context.Background(), store, []byte("test-secret-at-least-32-bytes!!"), time.Hour)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	_, tk, err := svc.Register(context.Background(), "ana@example.com", "ana", "s3cretpw", rememberMe)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return svc, store, tk.Refresh
}

func TestExpiracion(t *testing.T) {
	inicio := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		rememberMe bool
		ahora      time.Time
		want       time.Time
	}{
		{"recordar, recién abierta", true, inicio, inicio.Add(30 * 24 * time.Hour)},
		{"recordar, cerca del tope", true, inicio.Add(80 * 24 * time.Hour), inicio.Add(90 * 24 * time.Hour)},
		{"sin recordar, recién abierta", false, inicio, inicio.Add(24 * time.Hour)},
		{"sin recordar, cerca del tope", false, inicio.Add(6*24*time.Hour + 12*time.Hour), inicio.Add(7 * 24 * time.Hour)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expiracion(tt.rememberMe, inicio, tt.ahora); !got.Equal(tt.want) {
				t.Errorf("expiracion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefresh_RotaElToken(t *testing.T) {
	svc, _, viejo := sesionAbierta(t, true)

	_, tk, err := svc.Refresh(context.Background(), viejo)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if tk.Access == "" || tk.Refresh == "" || tk.Refresh == viejo || !tk.RememberMe {
		t.Fatalf("Refresh() = %+v, want access y refresh nuevo, remember_me", tk)
	}
	if _, _, err := svc.Refresh(context.Background(), tk.Refresh); err != nil {
		t.Fatalf("Refresh() con el token rotado error = %v", err)
	}
}

func TestRefresh_TokenAnteriorDentroDeLaGracia(t *testing.T) {
	svc, _, viejo := sesionAbierta(t, false)
	if _, _, err := svc.Refresh(context.Background(), viejo); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Segunda pestaña con la misma cookie: token de acceso sin rotar.
	_, tk, err := svc.Refresh(context.Background(), viejo)
	if err != nil {
		t.Fatalf("Refresh() con el token anterior error = %v", err)
	}
	if tk.Access == "" || tk.Refresh != "" {
		t.Fatalf("Refresh() = %+v, want access sin refresh nuevo", tk)
	}
}

// Otra petición rota el mismo token entre la lectura y el UPDATE: esta entra
// por la gracia en vez de fallar.
func TestRefresh_CarreraPerdidaEntraPorLaGracia(t *testing.T) {
	svc, store, token := sesionAbierta(t, true)
	store.rivalRota = true

	_, tk, err := svc.Refresh(context.Background(), token)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if tk.Access == "" || tk.Refresh != "" {
		t.Fatalf("Refresh() = %+v, want access sin refresh nuevo", tk)
	}
	if store.sesiones[0].RevokedAt != nil {
		t.Fatal("la carrera perdida revocó la sesión")
	}
}

func TestRefresh_ReusoFueraDeLaGraciaRevoca(t *testing.T) {
	svc, store, viejo := sesionAbierta(t, false)
	_, tk, err := svc.Refresh(context.Background(), viejo)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	store.sesiones[0].LastUsedAt = time.Now().Add(-graciaRotacion - time.Second)

	if _, _, err := svc.Refresh(context.Background(), viejo); !errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("Refresh() con token reusado = %v, want ErrSesionInvalida", err)
	}
	// La sesión quedó revocada: el token vigente tampoco sirve.
	if _, _, err := svc.Refresh(context.Background(), tk.Refresh); !errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("Refresh() tras el reuso = %v, want ErrSesionInvalida", err)
	}
}

func TestRefresh_Rechazos(t *testing.T) {
	tests := []struct {
		name     string
		preparar func(store *fakeAuthStore)
	}{
		{"vencida", func(s *fakeAuthStore) { s.sesiones[0].ExpiresAt = time.Now().Add(-time.Second) }},
		{"revocada", func(s *fakeAuthStore) { now := time.Now(); s.sesiones[0].RevokedAt = &now }},
		{"contraseña cambiada", func(s *fakeAuthStore) {
			u := s.byEmail["ana@example.com"]
			changed := time.Now()
			u.PasswordChangedAt = &changed
			s.byEmail["ana@example.com"] = u
		}},
		{"usuario suspendido", func(s *fakeAuthStore) {
			u := s.byEmail["ana@example.com"]
			u.State = sqlc.NullUserState{UserState: sqlc.UserStateSuspendido, Valid: true}
			s.byEmail["ana@example.com"] = u
		}},
		{"usuario borrado", func(s *fakeAuthStore) { delete(s.byEmail, "ana@example.com") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, store, token := sesionAbierta(t, true)
			tt.preparar(store)
			if _, _, err := svc.Refresh(context.Background(), token); !errors.Is(err, ErrSesionInvalida) {
				t.Fatalf("Refresh() = %v, want ErrSesionInvalida", err)
			}
		})
	}

	svc, _, _ := sesionAbierta(t, true)
	for _, token := range []string{"", "no-existe"} {
		if _, _, err := svc.Refresh(context.Background(), token); !errors.Is(err, ErrSesionInvalida) {
			t.Errorf("Refresh(%q) = %v, want ErrSesionInvalida", token, err)
		}
	}
}

func TestLogout_RevocaLaSesion(t *testing.T) {
	svc, _, token := sesionAbierta(t, true)
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, _, err := svc.Refresh(context.Background(), token); !errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("Refresh() tras Logout = %v, want ErrSesionInvalida", err)
	}
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout(\"\") error = %v, want nil", err)
	}
}

// cookieDeRefresh corre el handler Refresh con la cookie dada y devuelve el
// error y el Set-Cookie de la respuesta.
func cookieDeRefresh(t *testing.T, svc *Auth, token string) (error, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	gc.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	h := NewHandler(svc, nil, nil, CookieConfig{Path: "/api/v1/auth", Secure: true})
	_, err := h.Refresh(gc, httpapi.RefreshRequestObject{Params: httpapi.RefreshParams{RefreshToken: &token}})
	return err, rec.Header().Get("Set-Cookie")
}

func TestRefreshHandler_ErrorInternoConservaLaCookie(t *testing.T) {
	svc, store, token := sesionAbierta(t, true)
	store.errSesion = errors.New("base caída")

	err, setCookie := cookieDeRefresh(t, svc, token)
	if err == nil || errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("Refresh() error = %v, want el error de la base", err)
	}
	if setCookie != "" {
		t.Errorf("Set-Cookie = %q, want ninguno (la cookie se conserva)", setCookie)
	}
}

func TestRefreshHandler_SesionInvalidaBorraLaCookie(t *testing.T) {
	svc, _, _ := sesionAbierta(t, true)

	err, setCookie := cookieDeRefresh(t, svc, "no-existe")
	if !errors.Is(err, ErrSesionInvalida) {
		t.Fatalf("Refresh() error = %v, want ErrSesionInvalida", err)
	}
	if setCookie == "" {
		t.Error("Set-Cookie vacío, want la cookie borrada")
	}
}
