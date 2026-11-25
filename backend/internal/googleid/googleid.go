// Package googleid verifica los ID tokens que emite Google para el inicio de
// sesión. Es el único paquete que importa go-oidc.
package googleid

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// CertsURL es donde Google publica las claves con que firma sus ID tokens.
const CertsURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	issuerGoogle = "https://accounts.google.com"
	// timeoutSonda limita la petición propia a CertsURL tras un fallo.
	timeoutSonda = 5 * time.Second
)

var (
	// ErrTokenInvalido: el token no es de Google, no es para esta app, venció
	// o está mal formado.
	ErrTokenInvalido = errors.New("google id token invalid")
	// ErrClavesNoDisponibles: no se pudieron obtener las claves de Google, así
	// que no se sabe si el token es válido.
	ErrClavesNoDisponibles = errors.New("google keys unavailable")
)

// Claims son los datos del token que usa el inicio de sesión.
type Claims struct {
	Sub, Email, Name, HD string
	// Aud es el primer valor de aud que está en la lista configurada.
	Aud           string
	EmailVerified bool
}

// Verificador verifica ID tokens de Google contra una lista de client IDs.
type Verificador struct {
	clientIDs  []string
	httpClient *http.Client
	certsURL   string
	verifier   *oidc.IDTokenVerifier
}

// New arma el verificador sin hacer ninguna petición. server.go pasa
// CertsURL; los tests pasan la URL de un servidor httptest.
func New(clientIDs []string, httpClient *http.Client, certsURL string) *Verificador {
	keySet := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), httpClient), certsURL)
	return &Verificador{
		clientIDs:  clientIDs,
		httpClient: httpClient,
		certsURL:   certsURL,
		// El aud se revisa aquí contra varios client IDs (web, Android, iOS),
		// no en go-oidc, que solo acepta uno. go-oidc acepta también el iss
		// "accounts.google.com" sin esquema (oidc/verify.go:243).
		verifier: oidc.NewVerifier(issuerGoogle, keySet, &oidc.Config{SkipClientIDCheck: true}),
	}
}

// Verificar revisa firma, exp, iss, aud y azp, y devuelve los claims. Los
// errores son ErrTokenInvalido o ErrClavesNoDisponibles.
func (v *Verificador) Verificar(ctx context.Context, raw string) (Claims, error) {
	idToken, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		// go-oidc devuelve este error solo después de comprobar la firma, así
		// que no se puede falsificar, y no hace falta la petición de abajo.
		var vencido *oidc.TokenExpiredError
		if errors.As(err, &vencido) {
			return Claims{}, ErrTokenInvalido
		}
		// go-oidc envuelve los errores de las claves con %v
		// (oidc/verify.go:336), así que la causa no se puede leer del error.
		// Una petición propia distingue "Google no responde" de "token malo".
		// Solo ocurre tras un fallo, y el límite por IP la acota.
		if !v.clavesDisponibles(ctx) {
			return Claims{}, ErrClavesNoDisponibles
		}
		return Claims{}, ErrTokenInvalido
	}

	aud := ""
	for _, a := range idToken.Audience {
		if slices.Contains(v.clientIDs, a) {
			aud = a
			break
		}
	}
	if aud == "" {
		return Claims{}, ErrTokenInvalido
	}

	var c struct {
		Email         string    `json:"email"`
		EmailVerified boolFlexy `json:"email_verified"`
		Name          string    `json:"name"`
		HD            string    `json:"hd"`
		AZP           string    `json:"azp"`
	}
	if err := idToken.Claims(&c); err != nil {
		return Claims{}, ErrTokenInvalido
	}
	if c.AZP != "" && !slices.Contains(v.clientIDs, c.AZP) {
		return Claims{}, ErrTokenInvalido
	}

	return Claims{
		Sub:           idToken.Subject,
		Email:         c.Email,
		Name:          c.Name,
		HD:            c.HD,
		Aud:           aud,
		EmailVerified: bool(c.EmailVerified),
	}, nil
}

// clavesDisponibles hace un GET a certsURL y reporta si respondió 2xx.
func (v *Verificador) clavesDisponibles(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, timeoutSonda)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return false
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// boolFlexy acepta email_verified como booleano o como el string "true",
// porque Google lo ha mandado de las dos formas.
type boolFlexy bool

func (b *boolFlexy) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case bool:
		*b = boolFlexy(x)
	case string:
		*b = x == "true"
	default:
		*b = false
	}
	return nil
}
