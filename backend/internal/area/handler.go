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
	if anilloSeCruza(anillo) {
		return errors.New("El anillo se cruza a sí mismo; los vértices deben ir en orden alrededor del borde.")
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

// anilloSeCruza busca dos lados no contiguos que se toquen o se crucen. El último vértice
// repite al primero, así que se descarta para recorrer los lados en ciclo.
func anilloSeCruza(anillo [][]float64) bool {
	n := len(anillo) - 1
	for i := 0; i < n; i++ {
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue // contiguos a través del vértice de cierre
			}
			if segmentosSeTocan(anillo[i], anillo[(i+1)%n], anillo[j], anillo[(j+1)%n]) {
				return true
			}
		}
	}
	return false
}

func segmentosSeTocan(a, b, c, d []float64) bool {
	o1, o2 := orientacion(a, b, c), orientacion(a, b, d)
	o3, o4 := orientacion(c, d, a), orientacion(c, d, b)
	if o1 != o2 && o3 != o4 {
		return true
	}
	return o1 == 0 && enRango(a, b, c) || o2 == 0 && enRango(a, b, d) ||
		o3 == 0 && enRango(c, d, a) || o4 == 0 && enRango(c, d, b)
}

func orientacion(a, b, c []float64) int {
	v := (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func enRango(a, b, c []float64) bool {
	return min(a[0], b[0]) <= c[0] && c[0] <= max(a[0], b[0]) &&
		min(a[1], b[1]) <= c[1] && c[1] <= max(a[1], b[1])
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
