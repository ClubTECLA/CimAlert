package area

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/centro"
	"github.com/ClubTECLA/cimalert/backend/internal/geo"
	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/reporte"
	"github.com/google/uuid"
)

func poligono(anillo [][][]float64) httpapi.Poligono {
	return httpapi.Poligono{Type: httpapi.PoligonoType("Polygon"), Coordinates: anillo}
}

func cuadrado() [][][]float64 {
	return [][][]float64{{
		{-117.05, 32.50}, {-117.02, 32.50}, {-117.02, 32.53}, {-117.05, 32.53}, {-117.05, 32.50},
	}}
}

type fakeReportes struct {
	out      []reporte.Resumen
	err      error
	llamado  bool
	lastPoli geo.Poligono
}

func (f *fakeReportes) ListarEnArea(_ context.Context, p geo.Poligono) ([]reporte.Resumen, error) {
	f.llamado, f.lastPoli = true, p
	return f.out, f.err
}

type fakeCentros struct {
	out      []centro.Centro
	err      error
	llamado  bool
	lastPoli geo.Poligono
}

func (f *fakeCentros) ListarEnArea(_ context.Context, p geo.Poligono) ([]centro.Centro, error) {
	f.llamado, f.lastPoli = true, p
	return f.out, f.err
}

func TestBuscarArea(t *testing.T) {
	reporteID, centroID := uuid.New(), uuid.New()
	creado := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	reportes := &fakeReportes{out: []reporte.Resumen{{
		ID: reporteID, IncidenteID: 2, Titulo: "Socavón", EstadoActual: "Verificado",
		Avistamientos: 1, EsOficial: true, Latitude: 32.5149, Longitude: -117.0382, CreatedAt: creado,
	}}}
	centros := &fakeCentros{out: []centro.Centro{{
		ID: centroID, TipoCentro: 1, Nombre: "Refugio", Activo: true,
		Latitude: 32.515, Longitude: -117.038, CreatedAt: creado,
	}}}
	handler := NewHandler(NewService(reportes, centros))

	resp, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{
		Body: &httpapi.BuscarAreaRequest{Poligono: poligono(cuadrado())},
	})
	if err != nil {
		t.Fatalf("BuscarArea: %v", err)
	}
	body, ok := resp.(httpapi.BuscarArea200JSONResponse)
	if !ok {
		t.Fatalf("respuesta = %T, quería BuscarArea200JSONResponse", resp)
	}
	if len(body.Reportes) != 1 || body.Reportes[0].Id != reporteID || body.Reportes[0].Titulo != "Socavón" {
		t.Fatalf("reportes = %+v", body.Reportes)
	}
	if len(body.Centros) != 1 || body.Centros[0].Id != centroID || body.Centros[0].Nombre != "Refugio" {
		t.Fatalf("centros = %+v", body.Centros)
	}

	// Ambos servicios reciben el mismo polígono ya validado, en {Lat, Lon} y sin
	// el punto de cierre.
	for name, p := range map[string]geo.Poligono{"reportes": reportes.lastPoli, "centros": centros.lastPoli} {
		if len(p) != 4 || p[0].Lat != 32.50 || p[0].Lon != -117.05 {
			t.Errorf("%s recibió %+v", name, p)
		}
	}
}

func TestBuscarArea_ListasVacias(t *testing.T) {
	handler := NewHandler(NewService(&fakeReportes{}, &fakeCentros{}))

	resp, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{
		Body: &httpapi.BuscarAreaRequest{Poligono: poligono(cuadrado())},
	})
	if err != nil {
		t.Fatalf("BuscarArea: %v", err)
	}
	body := resp.(httpapi.BuscarArea200JSONResponse)
	// Deben ser [] y no null al serializar.
	if body.Reportes == nil || body.Centros == nil {
		t.Fatalf("listas nil: reportes=%v centros=%v", body.Reportes, body.Centros)
	}
}

func TestBuscarArea_SinBody(t *testing.T) {
	handler := NewHandler(NewService(&fakeReportes{}, &fakeCentros{}))

	resp, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{})
	if err != nil {
		t.Fatalf("BuscarArea: %v", err)
	}
	if _, ok := resp.(httpapi.BuscarArea400JSONResponse); !ok {
		t.Fatalf("respuesta = %T, quería BuscarArea400JSONResponse", resp)
	}
}

func TestBuscarArea_PoligonoInvalido(t *testing.T) {
	reportes, centros := &fakeReportes{}, &fakeCentros{}
	handler := NewHandler(NewService(reportes, centros))

	_, err := handler.BuscarArea(t.Context(), httpapi.BuscarAreaRequestObject{
		Body: &httpapi.BuscarAreaRequest{Poligono: poligono([][][]float64{{{-117, 32}, {-116, 32}, {-117, 32}}})},
	})
	// El 400 lo pone ReglasHTTP a partir de ErrAreaInvalida.
	if !errors.Is(err, ErrAreaInvalida) {
		t.Fatalf("err = %v, quería %v", err, ErrAreaInvalida)
	}
	if reportes.llamado || centros.llamado {
		t.Error("se consultó con un polígono inválido")
	}
}

func TestService_PropagaErrores(t *testing.T) {
	quiere := errors.New("base caída")

	t.Run("falla reportes", func(t *testing.T) {
		centros := &fakeCentros{}
		svc := NewService(&fakeReportes{err: quiere}, centros)
		if _, _, err := svc.Buscar(t.Context(), "Polygon", cuadrado()); !errors.Is(err, quiere) {
			t.Fatalf("err = %v, quería %v", err, quiere)
		}
		if centros.llamado {
			t.Error("se llamó a centros tras fallar reportes")
		}
	})

	t.Run("falla centros", func(t *testing.T) {
		svc := NewService(&fakeReportes{}, &fakeCentros{err: quiere})
		if _, _, err := svc.Buscar(t.Context(), "Polygon", cuadrado()); !errors.Is(err, quiere) {
			t.Fatalf("err = %v, quería %v", err, quiere)
		}
	})
}
