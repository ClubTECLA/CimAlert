package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
)

const (
	rutaRefresh = "/v1/auth/refresh"
	rutaLogout  = "/v1/auth/logout"

	// testCookiePath es el Path de la cookie en los tests, el default de producción.
	testCookiePath = "/api/v1/auth"
)

// conCookie manda la petición con la cookie refresh_token (si no es "") y sin
// bearer. body se serializa a JSON si no es nil.
func (c *testClient) conCookie(method, path, cookie string, body any) *httptest.ResponseRecorder {
	c.t.Helper()

	var b []byte
	if body != nil {
		var err error
		if b, err = json.Marshal(body); err != nil {
			c.t.Fatalf("serializando body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookie})
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec
}

// cookieRefresh devuelve la cookie refresh_token que puso la respuesta, o nil.
func cookieRefresh(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "refresh_token" {
			return ck
		}
	}
	return nil
}

// loginWeb registra un usuario, inicia sesión desde el web y devuelve su email y la cookie.
func (c *testClient) loginWeb(rememberMe bool) (string, *http.Cookie) {
	c.t.Helper()

	email, password := c.registrar()
	rec := c.expect(http.StatusOK, http.MethodPost, "/v1/auth/login", map[string]any{
		"email": email, "password": password, "remember_me": rememberMe,
	})
	if decode[httpapi.AuthResponse](c.t, rec).RefreshToken != nil {
		c.t.Fatal("login web: el refresh token vino en el body")
	}
	ck := cookieRefresh(rec)
	if ck == nil || ck.Value == "" {
		c.t.Fatalf("login web: sin cookie refresh_token; Set-Cookie: %v", rec.Header().Values("Set-Cookie"))
	}
	return email, ck
}

// sesionesVivas cuenta las sesiones sin revocar del usuario con ese email.
// Los tests de revocación lo miran directo: el chequeo de pwd_at ya da 401
// aunque la revocación falle, así que el status solo no la prueba.
func sesionesVivas(t *testing.T, email string) int {
	t.Helper()

	var n int
	err := testPool(t).QueryRow(t.Context(),
		`SELECT count(*) FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id WHERE u.email = $1 AND rt.revoked_at IS NULL`,
		email).Scan(&n)
	if err != nil {
		t.Fatalf("contando sesiones vivas: %v", err)
	}
	return n
}

// refreshCookie exige el status y devuelve la cookie nueva (nil si no vino).
func (c *testClient) refreshCookie(want int, cookie string) *http.Cookie {
	c.t.Helper()

	rec := c.conCookie(http.MethodPost, rutaRefresh, cookie, nil)
	if rec.Code != want {
		c.t.Fatalf("refresh: status = %d, quería %d; body: %s", rec.Code, want, rec.Body)
	}
	return cookieRefresh(rec)
}

func TestSesion_CookieWeb(t *testing.T) {
	c := newTestClient(t)

	_, ck := c.loginWeb(true)
	if !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteStrictMode || ck.Path != testCookiePath || ck.MaxAge <= 0 {
		t.Errorf("cookie = %+v, quería HttpOnly, Secure, SameSite=Strict, Path=%s y Max-Age > 0", ck, testCookiePath)
	}

	_, sinRecordar := c.loginWeb(false)
	if sinRecordar.MaxAge != 0 || !sinRecordar.Expires.IsZero() {
		t.Errorf("cookie sin remember_me = %+v, quería cookie de sesión (sin Max-Age ni Expires)", sinRecordar)
	}
}

func TestSesion_RefreshRotaYDetectaReuso(t *testing.T) {
	c := newTestClient(t)
	_, ck := c.loginWeb(true)

	nueva := c.refreshCookie(http.StatusOK, ck.Value)
	if nueva == nil || nueva.Value == ck.Value {
		t.Fatalf("refresh: cookie = %+v, quería un token rotado", nueva)
	}

	// Otra pestaña con la cookie anterior, dentro de la gracia: 200 sin tocar la cookie.
	if got := c.refreshCookie(http.StatusOK, ck.Value); got != nil {
		t.Errorf("refresh en la gracia: cookie = %+v, quería ninguna", got)
	}

	// Fuera de la gracia, el token anterior revoca la sesión y borra la cookie.
	if _, err := testPool(t).Exec(t.Context(),
		`UPDATE refresh_tokens SET last_used_at = now() - interval '1 minute' WHERE prev_token_hash IS NOT NULL AND token_hash = encode(sha256($1::bytea), 'hex')`,
		nueva.Value); err != nil {
		t.Fatalf("envejeciendo la rotación: %v", err)
	}
	if borrada := c.refreshCookie(http.StatusUnauthorized, ck.Value); borrada == nil || borrada.MaxAge >= 0 {
		t.Errorf("refresh con token reusado: cookie = %+v, quería que la borrara", borrada)
	}
	c.refreshCookie(http.StatusUnauthorized, nueva.Value)
}

func TestSesion_RefreshDevuelveUnTokenDeAccesoQueSirve(t *testing.T) {
	c := newTestClient(t)
	_, ck := c.loginWeb(false)

	rec := c.conCookie(http.MethodPost, rutaRefresh, ck.Value, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d; body: %s", rec.Code, rec.Body)
	}
	c.token = decode[httpapi.AuthResponse](t, rec).AccessToken
	c.expect(http.StatusOK, http.MethodGet, "/v1/auth/me", nil)
}

func TestSesion_SinCookieNiBody(t *testing.T) {
	c := newTestClient(t)
	c.refreshCookie(http.StatusUnauthorized, "")
	c.refreshCookie(http.StatusUnauthorized, "no-existe")
}

func TestSesion_LogoutWeb(t *testing.T) {
	c := newTestClient(t)
	_, ck := c.loginWeb(true)

	// Sin bearer: el token de acceso puede haber expirado.
	rec := c.conCookie(http.MethodPost, rutaLogout, ck.Value, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d; body: %s", rec.Code, rec.Body)
	}
	if borrada := cookieRefresh(rec); borrada == nil || borrada.MaxAge >= 0 {
		t.Errorf("logout: cookie = %+v, quería que la borrara", borrada)
	}
	c.refreshCookie(http.StatusUnauthorized, ck.Value)

	// Sin sesión también sale bien.
	if rec := c.conCookie(http.MethodPost, rutaLogout, "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("logout sin cookie: status = %d, quería 204", rec.Code)
	}
}

func TestSesion_Movil(t *testing.T) {
	c := newTestClient(t)
	email, password := c.registrar()

	rec := c.expect(http.StatusOK, http.MethodPost, "/v1/auth/login", map[string]any{
		"email": email, "password": password, "client": "mobile",
	})
	if ck := cookieRefresh(rec); ck != nil {
		t.Errorf("login móvil: puso cookie %+v", ck)
	}
	viejo := decode[httpapi.AuthResponse](t, rec).RefreshToken
	if viejo == nil || *viejo == "" {
		t.Fatal("login móvil: sin refresh_token en el body")
	}

	rec = c.conCookie(http.MethodPost, rutaRefresh, "", map[string]any{"refresh_token": *viejo})
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh móvil: status = %d; body: %s", rec.Code, rec.Body)
	}
	nuevo := decode[httpapi.AuthResponse](t, rec).RefreshToken
	if nuevo == nil || *nuevo == *viejo || cookieRefresh(rec) != nil {
		t.Fatalf("refresh móvil: refresh_token = %v, cookie = %v; quería token rotado en el body y sin cookie", nuevo, cookieRefresh(rec))
	}

	if rec := c.conCookie(http.MethodPost, rutaLogout, "", map[string]any{"refresh_token": *nuevo}); rec.Code != http.StatusNoContent {
		t.Fatalf("logout móvil: status = %d; body: %s", rec.Code, rec.Body)
	}
	if rec := c.conCookie(http.MethodPost, rutaRefresh, "", map[string]any{"refresh_token": *nuevo}); rec.Code != http.StatusUnauthorized {
		t.Errorf("refresh tras logout: status = %d, quería 401", rec.Code)
	}
}

func TestSesion_RegisterYGoogleAbrenSesion(t *testing.T) {
	c := newTestClient(t)

	email, password := credenciales()
	rec := c.expect(http.StatusCreated, http.MethodPost, "/v1/auth/register", map[string]any{
		"email": email, "username": "tester", "password": password, "client": "mobile", "remember_me": true,
	})
	if tk := decode[httpapi.AuthResponse](t, rec).RefreshToken; tk == nil || *tk == "" {
		t.Error("register móvil: sin refresh_token en el body")
	}

	gmail, _ := credencialesGmail()
	rec = c.google(claimsDe(gmail))
	if rec.Code != http.StatusOK {
		t.Fatalf("google: status = %d; body: %s", rec.Code, rec.Body)
	}
	if ck := cookieRefresh(rec); ck == nil || ck.Value == "" {
		t.Error("google web: sin cookie refresh_token")
	}
}

func TestSesion_ResetDeContrasenaRevocaTodas(t *testing.T) {
	c := newTestClient(t)
	email, password := c.registrar()

	var cookies []string
	for range 2 {
		rec := c.expect(http.StatusOK, http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": password})
		cookies = append(cookies, cookieRefresh(rec).Value)
	}

	c.recuperar(email, c.pedirPin(email, 1))
	if n := sesionesVivas(t, email); n != 0 {
		t.Errorf("sesiones vivas tras el reset = %d, quería 0", n)
	}
	for i, ck := range cookies {
		if rec := c.conCookie(http.MethodPost, rutaRefresh, ck, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("sesión %d tras el reset: status = %d, quería 401", i+1, rec.Code)
		}
	}
}

// Una sesión abierta con otra versión de la contraseña no se renueva, aunque
// no esté revocada (el login que leyó el hash viejo durante un reset).
func TestSesion_PwdAtDistintoRechaza(t *testing.T) {
	c := newTestClient(t)
	email, ck := c.loginWeb(true)

	if _, err := testPool(t).Exec(t.Context(), `UPDATE users SET password_changed_at = now() WHERE email = $1`, email); err != nil {
		t.Fatalf("cambiando password_changed_at: %v", err)
	}
	c.refreshCookie(http.StatusUnauthorized, ck.Value)
}

func TestSesion_RefreshTieneLimitePorIP(t *testing.T) {
	c := newTestClient(t, limiteSesionReal())

	for i := range middleware.RateLimitSesionRafaga {
		if rec := c.conCookie(http.MethodPost, rutaRefresh, "", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("refresh %d: status = %d, quería 401", i+1, rec.Code)
		}
	}
	if rec := c.conCookie(http.MethodPost, rutaRefresh, "", nil); rec.Code != http.StatusTooManyRequests {
		t.Errorf("refresh tras la ráfaga: status = %d, quería 429", rec.Code)
	}
}

// El vínculo con Google que quita el password revoca las sesiones anteriores
// de la cuenta; solo queda viva la que abre el propio login con Google.
func TestSesion_VinculoGoogleQueQuitaElPasswordRevoca(t *testing.T) {
	c := newTestClient(t)
	email, password := c.registrarGmail()

	rec := c.expect(http.StatusOK, http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": password})
	vieja := cookieRefresh(rec).Value

	if rec := c.google(claimsDe(email)); rec.Code != http.StatusOK {
		t.Fatalf("google: status = %d; body: %s", rec.Code, rec.Body)
	}
	if n := sesionesVivas(t, email); n != 1 {
		t.Errorf("sesiones vivas tras el vínculo = %d, quería 1 (la de Google)", n)
	}
	c.refreshCookie(http.StatusUnauthorized, vieja)
}
