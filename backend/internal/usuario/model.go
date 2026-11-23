package usuario

import (
	"time"

	"github.com/google/uuid"
)

// CreatePinTxParams son los datos para emitir un PIN de recuperación nuevo.
type CreatePinTxParams struct {
	UserID    uuid.UUID
	PinHash   []byte
	ExpiresAt time.Time
	// MaxPorHora es el tope de PIN emitidos en la última hora; al llegar a él no se crea ninguno.
	MaxPorHora int64
}

// ResetPasswordTxParams son los datos para cambiar la contraseña y cerrar
// el token de reset en una sola transacción.
type ResetPasswordTxParams struct {
	UserID uuid.UUID
	// PinID es la fila de recovery_pins que guarda el token de reset.
	PinID        uuid.UUID
	PasswordHash string
}

// ConRol es un usuario junto con el nombre de su rol, ya resuelto contra
// la tabla roles.
type ConRol struct {
	ID       uuid.UUID
	Email    *string
	Username string
	RolName  string
}
