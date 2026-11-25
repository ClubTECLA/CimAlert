package server_test

// Tests de integración de POST /v1/auth/google, uno por CUJ del diseño
// (docs/designs/google_login.md), más las carreras y el límite por IP. El
// verificador de Google es verificadorFalso: cada test le pone los claims.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/googleid"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/server"
	"github.com/google/uuid"
)

const rutaGoogle = "/v1/auth/google"

// Mensajes fijos del contrato, por el nombre de su error en usuario.
const (
	msgErrTokenGoogleInvalido      = "token de Google inválido"
	msgErrEmailGoogleNoVerificado  = "email de Google no verificado"
	msgErrGoogleEmailNoAdmitido    = "este email de Google no se puede usar aquí; regístrate o inicia sesión con email y contraseña"
	msgErrUsuarioSuspendido        = "usuario suspendido"
	msgErrGoogleCuentaDistinta     = "la cuenta ya está vinculada a otra cuenta de Google"
	msgErrGoogleVinculoNoPermitido = "esta cuenta no puede vincularse con Google; inicia sesión con tu contraseña"
	msgErrGoogleNoConfigurado      = "login con Google no configurado"
)

// claimsDe son claims válidos de Google para email, con un sub nuevo.
func claimsDe(email string) googleid.Claims {
	return googleid.Claims{Sub: "sub-" + uuid.NewString(), Email: email, Name: "Usuario Google", Aud: "web-client", EmailVerified: true}
}

// google pone los claims en el verificador falso y manda el token.
func (c *testClient) google(claims googleid.Claims) *httptest.ResponseRecorder {
	c.t.Helper()

	c.verificador.poner(claims, nil)
	return c.do(http.MethodPost, rutaGoogle, map[string]any{"id_token": "token-falso"})
}

// googleOK es google exigiendo 200; devuelve la respuesta decodificada.
func (c *testClient) googleOK(claims googleid.Claims) httpapi.GoogleAuthResponse {
	c.t.Helper()

	rec := c.google(claims)
	if rec.Code != http.StatusOK {
		c.t.Fatalf("POST %s: status = %d, quería 200; body: %s", rutaGoogle, rec.Code, rec.Body)
	}
	return decode[httpapi.GoogleAuthResponse](c.t, rec)
}

// exigirError falla si rec no tiene ese status y mensaje.
func exigirError(t *testing.T, rec *httptest.ResponseRecorder, status int, mensaje string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("status = %d, quería %d; body: %s", rec.Code, status, rec.Body)
	}
	if got := decode[httpapi.ErrorResponse](t, rec).Message; got != mensaje {
		t.Errorf("message = %q, quería %q", got, mensaje)
	}
}

// meCon pide /v1/auth/me con token y devuelve el status. No usa c.token, así
// que sirve desde varias goroutines.
func (c *testClient) meCon(token string) int {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec.Code
}

// tokenDeLogin inicia sesión con email y password y devuelve el JWT.
func (c *testClient) tokenDeLogin(email, password string) string {
	c.t.Helper()

	return decode[httpapi.AuthResponse](c.t, c.expect(http.StatusOK, http.MethodPost, "/v1/auth/login", map[string]any{
		"email": email, "password": password,
	})).AccessToken
}

// contar corre un SELECT count(*) y devuelve el resultado.
func contar(t *testing.T, query string, args ...any) int {
	t.Helper()

	var n int
	if err := testPool(t).QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("contando (%s): %v", query, err)
	}
	return n
}

func contarProveedoresGoogle(t *testing.T, sub string) int {
	t.Helper()
	return contar(t, `SELECT count(*) FROM auth_providers WHERE provider = 'google' AND provider_id = $1`, sub)
}

func contarProveedores(t *testing.T, email, provider string) int {
	t.Helper()
	return contar(t, `SELECT count(*) FROM auth_providers ap JOIN users u ON u.id = ap.user_id WHERE u.email = $1 AND ap.provider = $2`, email, provider)
}

// passwordHash devuelve el password_hash del usuario con ese email.
func passwordHash(t *testing.T, email string) *string {
	t.Helper()

	var hash *string
	if err := testPool(t).QueryRow(t.Context(), `SELECT password_hash FROM users WHERE email = $1`, email).Scan(&hash); err != nil {
		t.Fatalf("leyendo password_hash: %v", err)
	}
	return hash
}

// ejecutar corre una sentencia SQL de preparación.
func ejecutar(t *testing.T, sql string, args ...any) {
	t.Helper()

	if _, err := testPool(t).Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("ejecutando (%s): %v", sql, err)
	}
}

func suspender(t *testing.T, email string) {
	t.Helper()
	ejecutar(t, `UPDATE users SET state = 'Suspendido' WHERE email = $1`, email)
}

func TestGoogle_CUJ1_PrimerInicioCreaLaCuenta(t *testing.T) {
	c := newTestClient(t)
	email, _ := credencialesGmail()
	claims := claimsDe(email)

	res := c.googleOK(claims)
	if !res.Created {
		t.Error("created = false, quería true")
	}
	if string(res.User.Email) != email {
		t.Errorf("email = %q, quería %q", res.User.Email, email)
	}
	if n := contarProveedoresGoogle(t, claims.Sub); n != 1 {
		t.Errorf("proveedores google = %d, quería 1", n)
	}
	if n := contarHistorial(t, "usuario.registrar_google", "usuario", res.User.Id.String()); n != 1 {
		t.Errorf("usuario.registrar_google: %d eventos, quería 1", n)
	}
	if c.meCon(res.AccessToken) != http.StatusOK {
		t.Error("el token nuevo no sirve en /v1/auth/me")
	}
}

func TestGoogle_CUJ2_UsuarioQueRegresa(t *testing.T) {
	c := newTestClient(t)
	email, _ := credencialesGmail()
	claims := claimsDe(email)
	primero := c.googleOK(claims)

	// Mismo sub; Google reporta otro email y otro nombre.
	otroEmail, _ := credencialesGmail()
	claims.Email = otroEmail
	claims.Name = "Otro Nombre"
	segundo := c.googleOK(claims)

	if segundo.Created {
		t.Error("created = true en el segundo inicio")
	}
	if segundo.User.Id != primero.User.Id {
		t.Errorf("user id = %v, quería %v", segundo.User.Id, primero.User.Id)
	}
	if n := contar(t, `SELECT count(*) FROM users WHERE id = $1 AND email = $2 AND username = 'Usuario Google'`, primero.User.Id, email); n != 1 {
		t.Error("el email o el username guardados cambiaron")
	}
}

// CUJ 3 y 4: la cuenta local nunca probó su buzón, así que al vincular se
// quita el password (que pudo poner otra persona) y se cierra todo lo abierto.
func TestGoogle_CUJ3y4_VinculoQuitaElPassword(t *testing.T) {
	c := newTestClient(t)
	email, password := c.registrarGmail()
	tokenViejo := c.tokenDeLogin(email, password)
	// La recuperación que dejó a medias quien pre-registró el email.
	tokenReset := c.tokenDeReset(email, c.pedirPin(email, 1))

	res := c.googleOK(claimsDe(email))
	if res.Created {
		t.Error("created = true al vincular una cuenta existente")
	}
	if c.meCon(tokenViejo) != http.StatusUnauthorized {
		t.Error("el JWT anterior al vínculo sigue sirviendo")
	}
	if c.meCon(res.AccessToken) != http.StatusOK {
		t.Error("el JWT nuevo no sirve")
	}
	if c.puedeEntrar(email, password) {
		t.Error("el password viejo sigue sirviendo")
	}
	if n := contarProveedores(t, email, "local"); n != 0 {
		t.Errorf("proveedores local = %d, quería 0", n)
	}
	esperar(t, "aviso de vínculo con Google", func() bool { return c.mailer.avisadoGoogle(email) })

	evento := ultimoHistorial(t, "usuario", res.User.Id.String())
	var datos map[string]any
	if err := json.Unmarshal(evento.Datos, &datos); err != nil {
		t.Fatalf("datos del historial: %v", err)
	}
	if evento.Accion != "usuario.vincular_google" || datos["password_removed"] != true {
		t.Errorf("historial = %s %s, quería usuario.vincular_google con password_removed true", evento.Accion, evento.Datos)
	}

	if rec := c.restablecer(tokenReset); rec.Code != http.StatusBadRequest {
		t.Errorf("reset con el token anterior: status = %d, quería 400", rec.Code)
	}
	if passwordHash(t, email) != nil {
		t.Error("password_hash no quedó en NULL")
	}
}

// CUJ 5: la cuenta ya probó su buzón con la recuperación; el vínculo no toca
// el password ni las sesiones.
func TestGoogle_CUJ5_VinculoConservaElPassword(t *testing.T) {
	c := newTestClient(t)
	email, _ := c.registrarGmail()
	c.recuperar(email, c.pedirPin(email, 1))
	// El JWT del registro ya lo revocó la recuperación.
	token := c.tokenDeLogin(email, passwordNuevo)

	c.googleOK(claimsDe(email))

	if !c.puedeEntrar(email, passwordNuevo) {
		t.Error("el password dejó de servir")
	}
	if c.meCon(token) != http.StatusOK {
		t.Error("el JWT anterior dejó de servir")
	}
	// Da tiempo a un aviso en segundo plano que no debería mandarse.
	time.Sleep(100 * time.Millisecond)
	if c.mailer.avisadoGoogle(email) {
		t.Error("se mandó el aviso de vínculo sin quitar el password")
	}
}

func TestGoogle_CUJ6_VinculoRechazado(t *testing.T) {
	c := newTestClient(t)

	t.Run("rol distinto de ciudadano", func(t *testing.T) {
		email, _ := c.registrarGmail()
		ejecutar(t, `UPDATE users SET rol_id = (SELECT id FROM roles WHERE nombre = 'admin') WHERE email = $1`, email)
		exigirError(t, c.google(claimsDe(email)), http.StatusConflict, msgErrGoogleVinculoNoPermitido)
	})
	t.Run("vinculada a otra cuenta de Google", func(t *testing.T) {
		email, _ := c.registrarGmail()
		c.googleOK(claimsDe(email))
		exigirError(t, c.google(claimsDe(email)), http.StatusConflict, msgErrGoogleCuentaDistinta)
	})
}

// CUJ 7: Google no es la autoridad del email, así que no prueba nada.
func TestGoogle_CUJ7_EmailNoAutoritativo(t *testing.T) {
	c := newTestClient(t)

	existente, _ := c.registrar()
	nuevo, _ := credenciales()
	claimsNuevo := claimsDe(nuevo)

	conCuenta := c.google(claimsDe(existente))
	sinCuenta := c.google(claimsNuevo)
	exigirError(t, conCuenta, http.StatusForbidden, msgErrGoogleEmailNoAdmitido)
	exigirError(t, sinCuenta, http.StatusForbidden, msgErrGoogleEmailNoAdmitido)
	// La respuesta no revela si hay una cuenta con ese email.
	if conCuenta.Body.String() != sinCuenta.Body.String() {
		t.Errorf("bodies distintos: %s / %s", conCuenta.Body, sinCuenta.Body)
	}
	if n := contar(t, `SELECT count(*) FROM users WHERE email = $1`, nuevo); n != 0 {
		t.Errorf("se crearon %d usuarios", n)
	}
	if n := contarProveedoresGoogle(t, claimsNuevo.Sub); n != 0 {
		t.Errorf("se crearon %d proveedores google", n)
	}

	// Con hd (Workspace) Google sí es la autoridad.
	conHD := claimsDe(existente)
	conHD.HD = "example.com"
	if res := c.googleOK(conHD); res.Created {
		t.Error("created = true al vincular con hd")
	}
}

func TestGoogle_CUJ8_Suspendido(t *testing.T) {
	c := newTestClient(t)

	t.Run("vinculada, por sub", func(t *testing.T) {
		email, _ := credencialesGmail()
		claims := claimsDe(email)
		c.googleOK(claims)
		suspender(t, email)
		exigirError(t, c.google(claims), http.StatusForbidden, msgErrUsuarioSuspendido)
	})
	t.Run("sin vincular, por email", func(t *testing.T) {
		email, password := c.registrarGmail()
		suspender(t, email)
		antes := passwordHash(t, email)
		claims := claimsDe(email)

		exigirError(t, c.google(claims), http.StatusForbidden, msgErrUsuarioSuspendido)
		if n := contarProveedoresGoogle(t, claims.Sub); n != 0 {
			t.Errorf("se crearon %d proveedores google", n)
		}
		if despues := passwordHash(t, email); despues == nil || *despues != *antes {
			t.Error("password_hash cambió")
		}

		t.Run("login", func(t *testing.T) {
			exigirError(t, c.do(http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": password}),
				http.StatusForbidden, msgErrUsuarioSuspendido)
			if rec := c.do(http.MethodPost, "/v1/auth/login", map[string]any{"email": email, "password": "incorrecta123"}); rec.Code != http.StatusUnauthorized {
				t.Errorf("login con password incorrecto: status = %d, quería 401", rec.Code)
			}
		})
	})
}

func TestGoogle_CUJ9_TokenInvalido(t *testing.T) {
	c := newTestClient(t)

	c.verificador.poner(googleid.Claims{}, googleid.ErrTokenInvalido)
	exigirError(t, c.do(http.MethodPost, rutaGoogle, map[string]any{"id_token": "x"}), http.StatusUnauthorized, msgErrTokenGoogleInvalido)

	email, _ := credencialesGmail()
	claims := claimsDe(email)
	claims.EmailVerified = false
	exigirError(t, c.google(claims), http.StatusUnauthorized, msgErrEmailGoogleNoVerificado)

	if rec := c.do(http.MethodPost, rutaGoogle, map[string]any{"id_token": ""}); rec.Code != http.StatusBadRequest {
		t.Errorf("id_token vacío: status = %d, quería 400", rec.Code)
	}
}

func TestGoogle_CUJ10_SinGoogle(t *testing.T) {
	c := newTestClient(t, sinGoogle())
	exigirError(t, c.do(http.MethodPost, rutaGoogle, map[string]any{"id_token": "x"}), http.StatusServiceUnavailable, msgErrGoogleNoConfigurado)
}

// enParalelo manda la misma petición de Google desde dos goroutines y
// devuelve las dos respuestas.
func (c *testClient) enParalelo(claims googleid.Claims) [2]*httptest.ResponseRecorder {
	c.verificador.poner(claims, nil)
	var (
		recs  [2]*httptest.ResponseRecorder
		wg    sync.WaitGroup
		listo = make(chan struct{})
	)
	for i := range recs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-listo
			req := httptest.NewRequest(http.MethodPost, rutaGoogle, strings.NewReader(`{"id_token":"token-falso"}`))
			req.Header.Set("Content-Type", "application/json")
			recs[i] = httptest.NewRecorder()
			c.router.ServeHTTP(recs[i], req)
		}()
	}
	close(listo)
	wg.Wait()
	return recs
}

func TestGoogle_CarreraDeAltas(t *testing.T) {
	c := newTestClient(t)
	for i := range 20 {
		email, _ := credencialesGmail()
		claims := claimsDe(email)

		for _, rec := range c.enParalelo(claims) {
			if rec.Code != http.StatusOK {
				t.Fatalf("iteración %d: status = %d; body: %s", i, rec.Code, rec.Body)
			}
		}
		if n := contar(t, `SELECT count(*) FROM users WHERE email = $1`, email); n != 1 {
			t.Fatalf("iteración %d: %d usuarios, quería 1", i, n)
		}
		if n := contarProveedoresGoogle(t, claims.Sub); n != 1 {
			t.Fatalf("iteración %d: %d proveedores google, quería 1", i, n)
		}
	}
}

func TestGoogle_CarreraDeVinculos(t *testing.T) {
	c := newTestClient(t)
	for i := range 20 {
		email, _ := c.registrarGmail()
		claims := claimsDe(email)

		recs := c.enParalelo(claims)
		for _, rec := range recs {
			if rec.Code != http.StatusOK {
				t.Fatalf("iteración %d: status = %d; body: %s", i, rec.Code, rec.Body)
			}
		}
		if n := contarProveedores(t, email, "google"); n != 1 {
			t.Fatalf("iteración %d: %d proveedores google, quería 1", i, n)
		}
		// Los dos JWT llevan el password_changed_at que dejó el vínculo.
		for j, rec := range recs {
			token := decode[httpapi.GoogleAuthResponse](t, rec).AccessToken
			if code := c.meCon(token); code != http.StatusOK {
				t.Errorf("iteración %d: el JWT %d da %d en /v1/auth/me", i, j, code)
			}
		}
	}
}

func TestGoogle_LimitePorIP(t *testing.T) {
	t.Run("cada ruta tiene su contador", func(t *testing.T) {
		c := newTestClient(t, conRouter(server.WithRateLimitSesion(2, time.Hour)))

		// Con los claims vacíos del verificador falso, Google responde 401.
		for i := 1; i <= 3; i++ {
			rec := c.do(http.MethodPost, rutaGoogle, map[string]any{"id_token": "x"})
			if quiere429 := i == 3; (rec.Code == http.StatusTooManyRequests) != quiere429 {
				t.Errorf("google %d: status = %d", i, rec.Code)
			}
		}
		// Un contador compartido daría 429 desde el primer login.
		for i, want := range []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusTooManyRequests} {
			rec := c.do(http.MethodPost, "/v1/auth/login", map[string]any{"email": "nadie@example.com", "password": "incorrecta123"})
			if rec.Code != want {
				t.Errorf("login %d: status = %d, quería %d", i+1, rec.Code, want)
			}
		}
	})
	t.Run("valores de producción", func(t *testing.T) {
		c := newTestClient(t, limiteSesionReal())

		// Un body inválido: el límite corre antes que el validador, así que
		// gasta un token sin llegar a bcrypt, que tardaría más que el intervalo.
		for i := 1; i <= 11; i++ {
			rec := c.do(http.MethodPost, "/v1/auth/login", map[string]any{})
			want := http.StatusBadRequest
			if i == 11 {
				want = http.StatusTooManyRequests
			}
			if rec.Code != want {
				t.Fatalf("login %d: status = %d, quería %d; body: %s", i, rec.Code, want, rec.Body)
			}
		}
	})
}
