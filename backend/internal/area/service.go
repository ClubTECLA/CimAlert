package area

import (
	"context"
	"fmt"

	"github.com/ClubTECLA/cimalert/backend/internal/centro"
	"github.com/ClubTECLA/cimalert/backend/internal/geo"
	"github.com/ClubTECLA/cimalert/backend/internal/reporte"
)

type reportes interface {
	ListarEnArea(ctx context.Context, p geo.Poligono) ([]reporte.Resumen, error)
}
type centros interface {
	ListarEnArea(ctx context.Context, p geo.Poligono) ([]centro.Centro, error)
}

type Service struct {
	reportes reportes
	centros  centros
}

func NewService(r reportes, c centros) *Service {
	return &Service{reportes: r, centros: c}
}

func (s *Service) Buscar(ctx context.Context, tipo string, anillos [][][]float64) ([]reporte.Resumen, []centro.Centro, error) {
	p, err := geo.DesdeGeoJSON(tipo, anillos) // valida una sola vez
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrAreaInvalida, err)
	}
	rs, err := s.reportes.ListarEnArea(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	cs, err := s.centros.ListarEnArea(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	return rs, cs, nil
}
