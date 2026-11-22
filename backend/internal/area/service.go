package area

import (
	"context"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
)

// store son las dos consultas con PostGIS que necesita la búsqueda por área. Es
// estrecha a propósito: los tests unitarios la sustituyen por un fake.
type store interface {
	ListReportesEnArea(ctx context.Context, poligono string) ([]sqlc.ListReportesEnAreaRow, error)
	ListCentrosEnArea(ctx context.Context, poligono string) ([]sqlc.ListCentrosEnAreaRow, error)
}

// Store usa las consultas generadas por sqlc directamente: no hay operaciones
// de varias sentencias en esta búsqueda.
type Store struct {
	*sqlc.Queries
}

var _ store = (*Store)(nil)

type Service struct {
	store store
}

func NewService(s store) *Service {
	return &Service{store: s}
}

// Buscar devuelve reportes y centros cuyo centroide cae dentro del polígono. El
// polígono viaja como GeoJSON en texto: la conversión a geography y el índice
// GiST viven en la consulta SQL.
func (s *Service) Buscar(ctx context.Context, poligono string) ([]sqlc.ListReportesEnAreaRow, []sqlc.ListCentrosEnAreaRow, error) {
	reportes, err := s.store.ListReportesEnArea(ctx, poligono)
	if err != nil {
		return nil, nil, err
	}
	centros, err := s.store.ListCentrosEnArea(ctx, poligono)
	if err != nil {
		return nil, nil, err
	}
	return reportes, centros, nil
}
