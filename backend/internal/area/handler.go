package area

import (
	"context"

	"github.com/ClubTECLA/cimalert/backend/internal/centro"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/reporte"
)

// Handler implementa la operación buscarArea del contrato. El servidor lo
// embebe junto a los de las demás funcionalidades.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) BuscarArea(ctx context.Context, request httpapi.BuscarAreaRequestObject) (httpapi.BuscarAreaResponseObject, error) {
	if request.Body == nil {
		return httpapi.BuscarArea400JSONResponse{Message: "Falta el polígono."}, nil
	}

	reportes, centros, err := h.svc.Buscar(ctx,
		string(request.Body.Poligono.Type),
		request.Body.Poligono.Coordinates,
	)
	if err != nil {
		return nil, err // ErrAreaInvalida sale como 400 por ReglasHTTP
	}

	// make(..., 0, n) para que el JSON sea [] y no null cuando no hay resultados.
	resp := httpapi.BuscarArea200JSONResponse{
		Reportes: make([]httpapi.ReporteResumen, 0, len(reportes)),
		Centros:  make([]httpapi.Centro, 0, len(centros)),
	}
	for _, r := range reportes {
		resp.Reportes = append(resp.Reportes, reporte.ToReporteResumen(r))
	}
	for _, c := range centros {
		resp.Centros = append(resp.Centros, centro.ToCentro(c))
	}
	return resp, nil
}
