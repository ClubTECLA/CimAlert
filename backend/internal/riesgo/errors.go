package riesgo

import (
	"errors"
	"net/http"

	"github.com/ClubTECLA/cimalert/backend/internal/httperr"
)

var (
	ErrAreaInvalida      = errors.New("área inválida")
	ErrPeriodoInvalido   = errors.New("periodo inválido")
	ErrIncidenteInvalido = errors.New("incidente_id fuera de rango")
	// ErrFormatoInvalido: el archivo no es .xlsx ni .csv, o no tiene el
	// formato de importación. El mensaje concreto lo pone formatoInvalido.
	ErrFormatoInvalido     = errors.New("el archivo no tiene el formato de importación")
	ErrDatasetDuplicado    = errors.New("dataset duplicado")
	ErrDatasetNoEncontrado = errors.New("dataset no encontrado")
)

// ReglasHTTP dice con qué status y mensaje responde la API cada error de
// arriba. Al agregar un Err…, agrégale aquí su regla.
var ReglasHTTP = []httperr.Regla{
	{Err: ErrAreaInvalida, Status: http.StatusBadRequest, Message: "el área es inválida: el mínimo no puede ser mayor que el máximo"},
	{Err: ErrPeriodoInvalido, Status: http.StatusBadRequest, Message: "el periodo es inválido: desde no puede ser posterior a hasta"},
	{Err: ErrIncidenteInvalido, Status: http.StatusBadRequest, Message: "incidente_id inválido: fuera del rango de los ids"},
	// El handler responde el mensaje concreto (qué falta en el archivo); esta
	// regla solo evita un 500 si el error llegara a la tabla.
	{Err: ErrFormatoInvalido, Status: http.StatusBadRequest, Message: "el archivo no tiene el formato de importación"},
	{Err: ErrDatasetDuplicado, Status: http.StatusConflict, Message: "ese archivo ya se importó"},
	{Err: ErrDatasetNoEncontrado, Status: http.StatusNotFound, Message: "dataset no encontrado"},
}
