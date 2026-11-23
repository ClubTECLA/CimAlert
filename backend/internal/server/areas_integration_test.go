package server_test

import (
	"net/http"
	"testing"

	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/google/uuid"
)

// areaCentro es un cuadrado pequeño alrededor del centro de Tijuana, donde los
// tests crean reportes y centros. El punto lejano queda fuera.
func areaCentro() map[string]any {
	return map[string]any{
		"type": "Polygon",
		"coordinates": [][][]float64{{
			{-117.05, 32.50}, {-117.02, 32.50}, {-117.02, 32.53}, {-117.05, 32.53}, {-117.05, 32.50},
		}},
	}
}

func TestBuscarArea(t *testing.T) {
	c := newTestClient(t)
	c.loginComo("admin")

	tipo := decode[httpapi.TiposCentro](t,
		c.expect(http.StatusCreated, http.MethodPost, "/v1/tipos-centro", map[string]any{"nombre": nombreTipoCentro()}))

	crearReporteEn := func(lat, lng float64) uuid.UUID {
		t.Helper()
		rec := c.expect(http.StatusCreated, http.MethodPost, "/v1/reportes", map[string]any{
			"incidente_id": incidenteSocavon, "latitude": lat, "longitude": lng,
		})
		return decode[httpapi.Reporte](t, rec).Id
	}
	crearCentroEn := func(lat, lng float64) uuid.UUID {
		t.Helper()
		rec := c.expect(http.StatusCreated, http.MethodPost, "/v1/centro", map[string]any{
			"tipo_centro": tipo.Id, "nombre": "Centro " + uuid.NewString()[:8],
			"latitude": lat, "longitude": lng,
		})
		return decode[httpapi.Centro](t, rec).Id
	}

	dentroReporte := crearReporteEn(32.5149, -117.0382)
	// Fuera del área dibujada, pero dentro de la zona permitida por el backend.
	fueraReporte := crearReporteEn(32.515, -117.0)
	dentroCentro := crearCentroEn(32.515, -117.038)
	fueraCentro := crearCentroEn(32.515, -117.0)

	resp := decode[httpapi.BuscarAreaResponse](t,
		c.expect(http.StatusOK, http.MethodPost, "/v1/areas/buscar", map[string]any{"poligono": areaCentro()}))

	reportes := make(map[uuid.UUID]bool, len(resp.Reportes))
	for _, r := range resp.Reportes {
		reportes[r.Id] = true
	}
	if !reportes[dentroReporte] || reportes[fueraReporte] {
		t.Errorf("reportes = %+v; quería %s dentro y no %s", resp.Reportes, dentroReporte, fueraReporte)
	}

	centros := make(map[uuid.UUID]bool, len(resp.Centros))
	for _, centro := range resp.Centros {
		centros[centro.Id] = true
	}
	if !centros[dentroCentro] || centros[fueraCentro] {
		t.Errorf("centros = %+v; quería %s dentro y no %s", resp.Centros, dentroCentro, fueraCentro)
	}
}

func TestBuscarAreaErrores(t *testing.T) {
	c := newTestClient(t)

	t.Run("sin token responde 401", func(t *testing.T) {
		tc := *c
		tc.t = t
		tc.expect(http.StatusUnauthorized, http.MethodPost, "/v1/areas/buscar", map[string]any{"poligono": areaCentro()})
	})

	c.login()
	abierto := map[string]any{
		"type": "Polygon",
		"coordinates": [][][]float64{{
			{-117.05, 32.50}, {-117.02, 32.50}, {-117.02, 32.53}, {-117.05, 32.53},
		}},
	}

	t.Run("anillo abierto responde 400", func(t *testing.T) {
		tc := *c
		tc.t = t
		tc.expect(http.StatusBadRequest, http.MethodPost, "/v1/areas/buscar", map[string]any{"poligono": abierto})
	})

	t.Run("sin body responde 400", func(t *testing.T) {
		tc := *c
		tc.t = t
		tc.expect(http.StatusBadRequest, http.MethodPost, "/v1/areas/buscar", nil)
	})
}
