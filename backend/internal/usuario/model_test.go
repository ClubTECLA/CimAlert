package usuario

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
)

func TestEmailAutoritativo(t *testing.T) {
	tests := []struct {
		email, hd string
		want      bool
	}{
		{"ana@gmail.com", "", true},
		{"ana@example.com", "example.com", true},
		{"ana@example.com", "", false},
		{"ana@gmail.com.evil.com", "", false},
	}
	for _, tt := range tests {
		if got := emailAutoritativo(tt.email, tt.hd); got != tt.want {
			t.Errorf("emailAutoritativo(%q, %q) = %v, quería %v", tt.email, tt.hd, got, tt.want)
		}
	}
}

func TestDecidirVinculo(t *testing.T) {
	const sub = "sub-1"
	otro := "sub-2"
	mismo := sub

	// base pasa todas las reglas y conserva el password.
	base := candidatoVinculo{RolNombre: "ciudadano", RolDefault: "ciudadano"}

	tests := []struct {
		name               string
		cambiar            func(*candidatoVinculo)
		wantYaVinculado    bool
		wantQuitarPassword bool
		wantErr            error
	}{
		{name: "sin vínculo, password ya probado", cambiar: func(*candidatoVinculo) {}},
		{name: "sin vínculo, password nunca probado", cambiar: func(c *candidatoVinculo) { c.PasswordChangedAtNulo = true }, wantQuitarPassword: true},
		{name: "suspendido", cambiar: func(c *candidatoVinculo) { c.Suspendido = true }, wantErr: ErrUsuarioSuspendido},
		{name: "otro rol", cambiar: func(c *candidatoVinculo) { c.RolNombre = "admin" }, wantErr: ErrGoogleVinculoNoPermitido},
		{name: "ya vinculado al mismo sub", cambiar: func(c *candidatoVinculo) { c.GoogleProviderID = &mismo }, wantYaVinculado: true},
		{name: "vinculado a otro sub", cambiar: func(c *candidatoVinculo) { c.GoogleProviderID = &otro }, wantErr: ErrGoogleCuentaDistinta},
		// El orden de las reglas.
		{name: "suspendido gana sobre el rol", cambiar: func(c *candidatoVinculo) { c.Suspendido = true; c.RolNombre = "admin" }, wantErr: ErrUsuarioSuspendido},
		{name: "el rol gana sobre el vínculo existente", cambiar: func(c *candidatoVinculo) { c.RolNombre = "admin"; c.GoogleProviderID = &otro }, wantErr: ErrGoogleVinculoNoPermitido},
		{name: "ya vinculado no quita el password", cambiar: func(c *candidatoVinculo) { c.GoogleProviderID = &mismo; c.PasswordChangedAtNulo = true }, wantYaVinculado: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			tt.cambiar(&c)
			ya, quitar, err := decidirVinculo(c, sub)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, quería %v", err, tt.wantErr)
			}
			if ya != tt.wantYaVinculado || quitar != tt.wantQuitarPassword {
				t.Errorf("yaVinculado/quitarPassword = %v/%v, quería %v/%v", ya, quitar, tt.wantYaVinculado, tt.wantQuitarPassword)
			}
		})
	}
}

func TestUsernameDesdeGoogle(t *testing.T) {
	largo := strings.Repeat("á", 150)
	tests := []struct {
		name, nombre, email, want string
	}{
		{name: "con nombre", nombre: "  Ana López ", email: "ana@gmail.com", want: "Ana López"},
		{name: "sin nombre", nombre: "", email: "ana.lopez@gmail.com", want: "ana.lopez"},
		{name: "nombre de solo espacios", nombre: "   ", email: "ana@gmail.com", want: "ana"},
		{name: "nombre largo con acentos", nombre: largo, email: "ana@gmail.com", want: strings.Repeat("á", 100)},
		{name: "sin nombre ni parte local", nombre: "", email: "@gmail.com", want: "usuario"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usernameDesdeGoogle(tt.nombre, tt.email)
			if got != tt.want {
				t.Errorf("usernameDesdeGoogle() = %q, quería %q", got, tt.want)
			}
			if n := utf8.RuneCountInString(got); n < 1 || n > maxUsernameRunes {
				t.Errorf("largo = %d runas, quería entre 1 y %d", n, maxUsernameRunes)
			}
		})
	}
}

func TestEstaSuspendido(t *testing.T) {
	tests := []struct {
		state sqlc.NullUserState
		want  bool
	}{
		{sqlc.NullUserState{}, false},
		{sqlc.NullUserState{UserState: sqlc.UserStatePendiente, Valid: true}, false},
		{sqlc.NullUserState{UserState: sqlc.UserStateEnrevisin, Valid: true}, false},
		{sqlc.NullUserState{UserState: sqlc.UserStateVerificado, Valid: true}, false},
		{sqlc.NullUserState{UserState: sqlc.UserStateSuspendido, Valid: true}, true},
	}
	for _, tt := range tests {
		if got := estaSuspendido(tt.state); got != tt.want {
			t.Errorf("estaSuspendido(%+v) = %v, quería %v", tt.state, got, tt.want)
		}
	}
}
