package area

import (
	"errors"
	"net/http"

	"github.com/ClubTECLA/cimalert/backend/internal/httperr"
)

var (
	ErrAreaInvalida = errors.New("área inválida")
)

var ReglasHTTP = []httperr.Regla{
	{Err: ErrAreaInvalida, Status: http.StatusBadRequest},
}
