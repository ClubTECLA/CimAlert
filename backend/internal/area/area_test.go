package area

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/google/uuid"
)

func poligono(t *testing.T, anillo [][][]float64) httpapi.Poligono {
	t.Helper()
	return httpapi.Poligono{
		Type:        httpapi.PoligonoType("Polygon"),
		Coordinates: anillo,
	}
}

func cuadrado() [][][]float64 {
	return [][][]float64{{
		{-117.05, 32.50}, {-117.02, 32.50}, {-117.02, 32.53}, {-117.05, 32.53}, {-117.05, 32.50},
	}}
}

func TestValidarPoligono(t *testing.T) {
	tests := []struct {
		name      string
		poligono  httpapi.Poligono
		quiereErr string
	}{
		{name: "válido", poligono: poligono(t, cuadrado())},
		{
			name:      "otra geometría",
			poligono:  httpapi.Poligono{Type: httpapi.PoligonoType("LineString"), Coordinates: cuadrado()},
			quiereErr: "Polygon",
		},
		{
			name:      "sin anillos",
			poligono:  poligono(t, [][][]float64{}),
			quiereErr: "un solo anillo",
		},
		{
			name:      "menos de tres vértices",
			poligono:  poligono(t, [][][]float64{{{-117, 32}, {-116, 32}, {-117, 32}}}),
			quiereErr: "tres vértices",
		},
		{
			name:      "anillo abierto",
			poligono:  poligono(t, [][][]float64{{{-117.05, 32.50}, {-117.02, 32.50}, {-117.02, 32.53}, {-117.05, 32.53}}}),
			quiereErr: "cerrar",
		},
		{
			name:      "coordenadas fuera de rango",
			poligono:  poligono(t, [][][]float64{{{-181, 32}, {-117, 32}, {-117, 33}, {-181, 32}}}),
			quiereErr: "válidos",
		},
		{
			name:      "vértice con elevación",
			poligono:  poligono(t, [][][]float64{{{-117, 32, 10}, {-116, 32}, {-116, 33}, {-117, 32, 10}}}),
			quiereErr: "válidos",
		},
		{
			name:      "sin área",
			poligono:  poligono(t, [][][]float64{{{-117, 32}, {-116, 32}, {-115, 32}, {-117, 32}}}),
			quiereErr: "no tiene área",
		},
		{
			// Área firmada no nula (3): no la detiene el shoelace, solo la comprobación de lados.
			name:      "anillo que se cruza, lóbulos desiguales",
			poligono:  poligono(t, [][][]float64{{{-117, 32}, {-115, 34}, {-117, 35}, {-116, 32}, {-117, 32}}}),
			quiereErr: "se cruza",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validarPoligono(tt.poligono)
			if tt.quiereErr == "" {
				if err != nil {
					t.Fatalf("validarPoligono: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.quiereErr) {
				t.Fatalf("err = %v, quería que contuviera %q", err, tt.quiereErr)
			}
		})
	}
}

// fakeStore implementa store guardando el GeoJSON que recibe, para comprobar
// que el handler serializa el polígono del contrato sin transformarlo.
type fakeStore struct {
	reportes []sqlc.ListReportesEnAreaRow
	centros  []sqlc.ListCentrosEnAreaRow
	err      error

	lastPoligono string
}

func (f *fakeStore) ListReportesEnArea(_ context.Context, poligono string) ([]sqlc.ListReportesEnAreaRow, error) {
	f.lastPoligono = poligono
	return f.reportes, f.err
}

func (f *fakeStore) ListCentrosEnArea(_ context.Context, poligono string) ([]sqlc.ListCentrosEnAreaRow, error) {
	f.lastPoligono = poligono
	return f.centros, f.err
}

func TestBuscarArea(t *testing.T) {
	reporteID, centroID := uuid.New(), uuid.New()
	creado := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{
		reportes: []sqlc.ListReportesEnAreaRow{{
			ID: reporteID, IncidenteID: 2, EstadoActual: sqlc.Estado("Verificado"),
			Avistamientos: 1, EsOficial: true, Latitude: 32.5149, Longitude: -117.0382,
			Direccion: nil, IncidenteEtiqueta: "Socavón", CreatedAt: creado,
		}},
		centros: []sqlc.ListCentrosEnAreaRow{{
			ID: centroID, TipoCentro: 1, Nombre: "Refugio", Activo: true,
			Latitude: 32.515, Longitude: -117.038, CreatedAt: creado,
		}},
	}
	handler := NewHandler(NewService(store))

	resp, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{
		Body: &httpapi.BuscarAreaRequest{Poligono: poligono(t, cuadrado())},
	})
	if err != nil {
		t.Fatalf("BuscarArea: %v", err)
	}
	body, ok := resp.(httpapi.BuscarArea200JSONResponse)
	if !ok {
		t.Fatalf("respuesta = %T, quería BuscarArea200JSONResponse", resp)
	}

	if len(body.Reportes) != 1 || body.Reportes[0].Id != reporteID {
		t.Fatalf("reportes = %+v", body.Reportes)
	}
	if body.Reportes[0].Titulo != "Socavón" || body.Reportes[0].EstadoActual != httpapi.EstadoReporte("Verificado") {
		t.Errorf("reporte = %+v", body.Reportes[0])
	}
	if len(body.Centros) != 1 || body.Centros[0].Id != centroID || body.Centros[0].Nombre != "Refugio" {
		t.Fatalf("centros = %+v", body.Centros)
	}

	// El store recibe el GeoJSON íntegro: PostGIS lo convierte a geography en la consulta.
	var enviado struct {
		Type        string        `json:"type"`
		Coordinates [][][]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(store.lastPoligono), &enviado); err != nil {
		t.Fatalf("GeoJSON enviado: %v", err)
	}
	if enviado.Type != "Polygon" || len(enviado.Coordinates) != 1 || len(enviado.Coordinates[0]) != 5 {
		t.Errorf("GeoJSON enviado = %+v", enviado)
	}
}

func TestBuscarAreaRechazaPoligonoInvalido(t *testing.T) {
	store := &fakeStore{}
	handler := NewHandler(NewService(store))

	resp, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{
		Body: &httpapi.BuscarAreaRequest{Poligono: poligono(t, [][][]float64{{{-117, 32}, {-116, 32}, {-117, 32}}})},
	})
	if err != nil {
		t.Fatalf("BuscarArea: %v", err)
	}
	if _, ok := resp.(httpapi.BuscarArea400JSONResponse); !ok {
		t.Fatalf("respuesta = %T, quería BuscarArea400JSONResponse", resp)
	}
	if store.lastPoligono != "" {
		t.Errorf("se consultó la base con un polígono inválido: %s", store.lastPoligono)
	}
}

func TestServicePropagaErrores(t *testing.T) {
	quiere := errors.New("base caída")
	store := &fakeStore{err: quiere}
	svc := NewService(store)

	if _, _, err := svc.Buscar(t.Context(), "{}"); !errors.Is(err, quiere) {
		t.Fatalf("err = %v, quería %v", err, quiere)
	}
}
