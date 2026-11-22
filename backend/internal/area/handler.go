package area

import (
	"context"
	"encoding/json"
	"errors"
	"math"

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

// validarPoligono rechaza lo que PostGIS no podría convertir o no tendría
// sentido como área. El validador del contrato ya exige la forma de los datos;
// aquí se comprueban las reglas geométricas que no caben en un schema JSON.
func validarPoligono(poligono httpapi.Poligono) error {
	if string(poligono.Type) != "Polygon" {
		return errors.New("La geometría debe ser un Polygon.")
	}
	if len(poligono.Coordinates) != 1 {
		return errors.New("El polígono debe tener un solo anillo.")
	}
	anillo := poligono.Coordinates[0]
	if len(anillo) < 4 {
		return errors.New("El polígono necesita al menos tres vértices.")
	}

	primero, ultimo := anillo[0], anillo[len(anillo)-1]
	for _, vertice := range anillo {
		if len(vertice) != 2 || vertice[0] < -180 || vertice[0] > 180 || vertice[1] < -90 || vertice[1] > 90 {
			return errors.New("Las coordenadas deben ser pares [longitud, latitud] válidos.")
		}
	}
	if primero[0] != ultimo[0] || primero[1] != ultimo[1] {
		return errors.New("El anillo debe cerrar repitiendo el primer vértice.")
	}

	// Shoelace: un anillo sin área no filtraría nada y probablemente es un
	// doble clic del usuario, no un área dibujada.
	var area float64
	for i := 0; i < len(anillo)-1; i++ {
		area += anillo[i][0]*anillo[i+1][1] - anillo[i+1][0]*anillo[i][1]
	}
	if math.Abs(area) < 1e-12 {
		return errors.New("El polígono no tiene área.")
	}
	return nil
}

func (h *Handler) BuscarArea(ctx context.Context, request httpapi.BuscarAreaRequestObject) (httpapi.BuscarAreaResponseObject, error) {
	if request.Body == nil {
		return httpapi.BuscarArea400JSONResponse{Message: "Falta el polígono."}, nil
	}
	if err := validarPoligono(request.Body.Poligono); err != nil {
		return httpapi.BuscarArea400JSONResponse{Message: err.Error()}, nil
	}

	geojson, err := json.Marshal(request.Body.Poligono)
	if err != nil {
		return nil, err
	}
	reportes, centros, err := h.svc.Buscar(ctx, string(geojson))
	if err != nil {
		return nil, err
	}

	out := httpapi.BuscarArea200JSONResponse{
		Reportes: make([]httpapi.ReporteResumen, 0, len(reportes)),
		Centros:  make([]httpapi.Centro, 0, len(centros)),
	}
	for _, r := range reportes {
		out.Reportes = append(out.Reportes, httpapi.ReporteResumen{
			Id:            r.ID,
			IncidenteId:   r.IncidenteID,
			Titulo:        reporte.Titulo(r.IncidenteEtiqueta, r.Direccion),
			Direccion:     r.Direccion,
			CreatedAt:     r.CreatedAt,
			EstadoActual:  httpapi.EstadoReporte(r.EstadoActual),
			Avistamientos: r.Avistamientos,
			EsOficial:     r.EsOficial,
			Latitude:      r.Latitude,
			Longitude:     r.Longitude,
		})
	}
	for _, c := range centros {
		out.Centros = append(out.Centros, httpapi.Centro{
			Activo:      c.Activo,
			Cantidad:    c.Cantidad,
			Capacidad:   c.Capacidad,
			CreatedAt:   c.CreatedAt,
			Descripcion: c.Descripcion,
			Id:          c.ID,
			ImagePath:   c.ImagePath,
			Latitude:    c.Latitude,
			Longitude:   c.Longitude,
			Nombre:      c.Nombre,
			TipoCentro:  c.TipoCentro,
		})
	}
	return out, nil
}
