package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolationOn(t *testing.T) {
	const nombre = "auth_providers_provider_provider_id_key"

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "unique con el nombre dado", err: &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: nombre}, want: true},
		{name: "unique con otro nombre", err: &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "users_email_key"}, want: false},
		{name: "foreign key", err: &pgconn.PgError{Code: pgForeignKeyViolation, ConstraintName: nombre}, want: false},
		{name: "envuelto", err: fmt.Errorf("x: %w", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: nombre}), want: true},
		{name: "otro error", err: errors.New("x"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolationOn(tt.err, nombre); got != tt.want {
				t.Errorf("IsUniqueViolationOn() = %v, quería %v", got, tt.want)
			}
		})
	}
}
