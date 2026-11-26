package googleid

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	clientWeb     = "web.apps.googleusercontent.com"
	clientAndroid = "android.apps.googleusercontent.com"
	kidPrueba     = "clave-1"
)

// servidorClaves sirve un JWKS con la clave pública de prueba. Cuenta las
// peticiones y puede responder con otro status.
type servidorClaves struct {
	*httptest.Server
	peticiones atomic.Int32
	status     atomic.Int32
}

func nuevoServidorClaves(t *testing.T, pub *rsa.PublicKey) *servidorClaves {
	t.Helper()

	// n y e en base64url a mano, para no importar go-jose en los tests.
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"alg": "RS256",
		"use": "sig",
		"kid": kidPrueba,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}})
	if err != nil {
		t.Fatal(err)
	}

	s := &servidorClaves{}
	s.status.Store(http.StatusOK)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.peticiones.Add(1)
		if code := int(s.status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(s.Close)
	return s
}

func generarClave(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// claimsValidos son los claims de un token que Verificar acepta; cada caso
// cambia lo que necesita.
func claimsValidos() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            clientWeb,
		"azp":            clientWeb,
		"sub":            "1234567890",
		"email":          "ana@gmail.com",
		"email_verified": true,
		"name":           "Ana",
		"iat":            time.Now().Unix(),
		"exp":            time.Now().Add(time.Hour).Unix(),
	}
}

func firmar(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kidPrueba
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func nuevoVerificador(s *servidorClaves) *Verificador {
	return New([]string{clientWeb, clientAndroid}, &http.Client{Timeout: 5 * time.Second}, s.URL)
}

func TestVerificar(t *testing.T) {
	key := generarClave(t)
	otraClave := generarClave(t)
	s := nuevoServidorClaves(t, &key.PublicKey)

	tests := []struct {
		name    string
		key     *rsa.PrivateKey
		cambiar func(jwt.MapClaims)
		wantErr error
		want    Claims
	}{
		{
			name: "válido",
			want: Claims{Sub: "1234567890", Email: "ana@gmail.com", Name: "Ana", Aud: clientWeb, EmailVerified: true},
		},
		{
			// Falla si go-oidc deja de aceptar el iss sin esquema.
			name:    "válido con iss sin esquema",
			cambiar: func(c jwt.MapClaims) { c["iss"] = "accounts.google.com" },
			want:    Claims{Sub: "1234567890", Email: "ana@gmail.com", Name: "Ana", Aud: clientWeb, EmailVerified: true},
		},
		{
			name:    "aud como lista con un valor configurado",
			cambiar: func(c jwt.MapClaims) { c["aud"] = []string{"otra-app", clientAndroid}; c["azp"] = clientAndroid },
			want:    Claims{Sub: "1234567890", Email: "ana@gmail.com", Name: "Ana", Aud: clientAndroid, EmailVerified: true},
		},
		{
			name:    "email_verified como string",
			cambiar: func(c jwt.MapClaims) { c["email_verified"] = "true" },
			want:    Claims{Sub: "1234567890", Email: "ana@gmail.com", Name: "Ana", Aud: clientWeb, EmailVerified: true},
		},
		{
			// El claim se reporta; quien llama decide.
			name:    "email_verified en false sí verifica",
			cambiar: func(c jwt.MapClaims) { c["email_verified"] = false },
			want:    Claims{Sub: "1234567890", Email: "ana@gmail.com", Name: "Ana", Aud: clientWeb},
		},
		{
			name:    "hd se reporta",
			cambiar: func(c jwt.MapClaims) { c["email"] = "ana@empresa.mx"; c["hd"] = "empresa.mx" },
			want:    Claims{Sub: "1234567890", Email: "ana@empresa.mx", Name: "Ana", HD: "empresa.mx", Aud: clientWeb, EmailVerified: true},
		},
		{
			name:    "vencido",
			cambiar: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
			wantErr: ErrTokenInvalido,
		},
		{
			name:    "aud de otra app",
			cambiar: func(c jwt.MapClaims) { c["aud"] = "otra-app" },
			wantErr: ErrTokenInvalido,
		},
		{
			name:    "azp de otra app",
			cambiar: func(c jwt.MapClaims) { c["azp"] = "otra-app" },
			wantErr: ErrTokenInvalido,
		},
		{
			name:    "otro emisor",
			cambiar: func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" },
			wantErr: ErrTokenInvalido,
		},
		{
			name:    "firmado con otra clave",
			key:     otraClave,
			wantErr: ErrTokenInvalido,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := claimsValidos()
			if tt.cambiar != nil {
				tt.cambiar(claims)
			}
			firma := key
			if tt.key != nil {
				firma = tt.key
			}

			got, err := nuevoVerificador(s).Verificar(t.Context(), firmar(t, firma, claims))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, quería %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("claims = %+v, quería %+v", got, tt.want)
			}
		})
	}
	t.Run("token mal formado", func(t *testing.T) {
		if _, err := nuevoVerificador(s).Verificar(t.Context(), "no-es-un-jwt"); !errors.Is(err, ErrTokenInvalido) {
			t.Errorf("error = %v, quería ErrTokenInvalido", err)
		}
	})
}

func TestVerificar_ClavesNoDisponibles(t *testing.T) {
	key := generarClave(t)

	t.Run("las claves responden 500", func(t *testing.T) {
		s := nuevoServidorClaves(t, &key.PublicKey)
		s.status.Store(http.StatusInternalServerError)
		_, err := nuevoVerificador(s).Verificar(t.Context(), firmar(t, key, claimsValidos()))
		if !errors.Is(err, ErrClavesNoDisponibles) {
			t.Errorf("error = %v, quería ErrClavesNoDisponibles", err)
		}
	})
	t.Run("el servidor de claves está cerrado", func(t *testing.T) {
		s := nuevoServidorClaves(t, &key.PublicKey)
		v := nuevoVerificador(s)
		s.Close()
		_, err := v.Verificar(t.Context(), firmar(t, key, claimsValidos()))
		if !errors.Is(err, ErrClavesNoDisponibles) {
			t.Errorf("error = %v, quería ErrClavesNoDisponibles", err)
		}
	})
}

// Con las claves ya en caché, un token vencido es ErrTokenInvalido aunque
// Google no responda, y no se hace ninguna petición extra: go-oidc reporta el
// vencimiento después de comprobar la firma con la caché.
func TestVerificar_VencidoNoConsultaLasClaves(t *testing.T) {
	key := generarClave(t)
	s := nuevoServidorClaves(t, &key.PublicKey)
	v := nuevoVerificador(s)

	if _, err := v.Verificar(t.Context(), firmar(t, key, claimsValidos())); err != nil {
		t.Fatalf("llenando la caché de claves: %v", err)
	}
	s.status.Store(http.StatusInternalServerError)
	antes := s.peticiones.Load()

	vencido := claimsValidos()
	vencido["exp"] = time.Now().Add(-time.Minute).Unix()
	if _, err := v.Verificar(t.Context(), firmar(t, key, vencido)); !errors.Is(err, ErrTokenInvalido) {
		t.Errorf("error = %v, quería ErrTokenInvalido", err)
	}
	if n := s.peticiones.Load() - antes; n != 0 {
		t.Errorf("se hicieron %d peticiones a las claves, quería 0", n)
	}
}
