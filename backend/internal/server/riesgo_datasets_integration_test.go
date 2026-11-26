package server_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/google/uuid"
)

// importar manda el archivo en el campo file (y nombre si no es vacío) a
// POST /v1/riesgo/datasets.
func (c *testClient) importar(archivo string, contenido []byte, nombre string) *httptest.ResponseRecorder {
	c.t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if nombre != "" {
		if err := w.WriteField("nombre", nombre); err != nil {
			c.t.Fatal(err)
		}
	}
	fw, err := w.CreateFormFile("file", archivo)
	if err != nil {
		c.t.Fatalf("CreateFormFile() = %v", err)
	}
	fw.Write(contenido)
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/riesgo/datasets", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec
}

func (c *testClient) importarEsperando(want int, archivo string, contenido []byte, nombre string) *httptest.ResponseRecorder {
	c.t.Helper()
	rec := c.importar(archivo, contenido, nombre)
	if rec.Code != want {
		c.t.Fatalf("POST /v1/riesgo/datasets = %d, quería %d: %s", rec.Code, want, rec.Body.String())
	}
	return rec
}

func TestDatasetsHistoricos(t *testing.T) {
	admin := newTestClient(t)
	admin.loginComo("admin")
	// Categoría propia del test: filtrando por ella el resultado no depende de
	// lo que hayan creado otros tests. El archivo la nombra en mayúsculas.
	inc := admin.crearIncidente(map[string]any{"nombre": nombreIncidente(), "color": "#3366FF", "esta_activo": true})
	resumen := func(c *testClient) httpapi.ResumenRiesgo {
		return decode[httpapi.ResumenRiesgo](t, c.expect(http.StatusOK, http.MethodGet,
			fmt.Sprintf("/v1/riesgo/resumen?incidente_id=%d", inc.Id), nil))
	}

	// 5 incidentes a menos de 100 m (una zona) y uno lejos; uno sin colonia.
	cat := strings.ToUpper(inc.Nombre)
	archivo := "Colonia;Latitud;Longitud;Mes/año del Incidente;Categoria;Personas afectadas (total);Fallecidos\n" +
		"Colonia Dataset;32,4500;-116,9000;02/2017;" + cat + ";1;1\n" +
		"Colonia Dataset;32,4503;-116,9002;01/2023;" + cat + ";2;\n" +
		"Colonia Dataset;32,4498;-116,8997;08/2023;" + cat + ";21;\n" +
		"Colonia Dataset;32,4502;-116,9004;02/1998;" + cat + ";0;\n" +
		";32,4499;-116,9003;12/2010;" + cat + ";;\n" +
		"Lejos;32,5300;-117,0300;09/2026;" + cat + ";160;\n"

	var d httpapi.DatasetHistorico
	t.Run("importar", func(t *testing.T) {
		tc := *admin
		tc.t = t
		d = decode[httpapi.DatasetHistorico](t, tc.importarEsperando(http.StatusCreated, "lluvias.csv", []byte(archivo), ""))
		if d.Nombre != "lluvias.csv" || d.Archivo != "lluvias.csv" || d.Filas != 6 || d.CargadoPor == nil {
			t.Errorf("dataset = %+v", d)
		}
		if n := contarHistorial(t, "riesgo.importar_dataset", "dataset_historico", d.Id.String()); n != 1 {
			t.Errorf("eventos de importar_dataset = %d, quería 1", n)
		}

		res := resumen(&tc)
		if res.TotalHistoricos != 6 || res.VersionModelo != 2 {
			t.Errorf("total_historicos = %d, version_modelo = %d, quería 6 y 2", res.TotalHistoricos, res.VersionModelo)
		}
		if len(res.Zonas) != 1 || res.Zonas[0].Reportes != 5 || res.Zonas[0].Nombre != "Colonia Dataset" {
			t.Errorf("zonas = %+v, quería una de 5 en Colonia Dataset", res.Zonas)
		}
		// Por peso: Colonia Dataset (4 incidentes, ≈ 8.9) antes que Lejos (1
		// incidente con 160 afectados, 1 + ln 161 ≈ 6.1).
		if len(res.Colonias) != 2 || res.Colonias[0].Nombre != "Colonia Dataset" || res.Colonias[0].Reportes != 4 {
			t.Errorf("colonias = %+v", res.Colonias)
		}

		mapa := decode[httpapi.MapaCalor](t, tc.expect(http.StatusOK, http.MethodGet,
			fmt.Sprintf("/v1/riesgo/mapa-calor?min_lat=32.44&max_lat=32.46&min_lng=-116.91&max_lng=-116.89&zoom=13&incidente_id=%d", inc.Id), nil))
		total := 0
		for _, c := range mapa.Celdas {
			total += c.Reportes
		}
		if total != 5 {
			t.Errorf("reportes en las celdas = %d, quería 5", total)
		}

		// El periodo filtra por el mes del incidente.
		desde := decode[httpapi.ResumenRiesgo](t, tc.expect(http.StatusOK, http.MethodGet,
			fmt.Sprintf("/v1/riesgo/resumen?incidente_id=%d&desde=2023-01-01T00:00:00Z", inc.Id), nil))
		if desde.TotalHistoricos != 3 {
			t.Errorf("desde 2023: total_historicos = %d, quería 3", desde.TotalHistoricos)
		}
	})

	t.Run("listar", func(t *testing.T) {
		tc := *admin
		tc.t = t
		ds := decode[[]httpapi.DatasetHistorico](t, tc.expect(http.StatusOK, http.MethodGet, "/v1/riesgo/datasets", nil))
		if !slices.ContainsFunc(ds, func(x httpapi.DatasetHistorico) bool { return x.Id == d.Id && x.Filas == 6 }) {
			t.Errorf("el dataset importado no aparece en la lista")
		}
	})

	t.Run("el mismo archivo no se importa dos veces", func(t *testing.T) {
		tc := *admin
		tc.t = t
		tc.importarEsperando(http.StatusConflict, "otra-vez.csv", []byte(archivo), "Otro nombre")
	})

	t.Run("filas inválidas", func(t *testing.T) {
		tc := *admin
		tc.t = t
		// La fila 4 está en San Diego: coordenadas válidas, fuera de Baja California.
		malo := "Latitud,Longitud,Fecha,Categoria\n32.45,-116.9,2023-01," + inc.Nombre + "\n32.45,-116.9,,Otro\n" +
			"32.7157,-117.1611,2023-01," + inc.Nombre + "\n"
		res := decode[httpapi.ErrorImportacion](t, tc.importarEsperando(http.StatusUnprocessableEntity, "malo.csv", []byte(malo), ""))
		want := []httpapi.ErrorFilaImportacion{
			{Fila: 3, Columna: "Mes/año del Incidente", Mensaje: "falta la fecha"},
			{Fila: 3, Columna: "Categoria", Mensaje: `la categoría "Otro" no existe en el catálogo`},
			{Fila: 4, Columna: "", Mensaje: "la ubicación está fuera de la zona permitida (Baja California)"},
		}
		if res.TotalErrores != 3 || !slices.Equal(res.Errores, want) {
			t.Errorf("errores = %+v (total %d)", res.Errores, res.TotalErrores)
		}
		// No se importó nada, ni siquiera la fila buena.
		if r := resumen(&tc); r.TotalHistoricos != 6 {
			t.Errorf("total_historicos = %d tras un archivo inválido, quería 6", r.TotalHistoricos)
		}
	})

	t.Run("archivo sin el formato", func(t *testing.T) {
		tc := *admin
		tc.t = t
		rec := tc.importarEsperando(http.StatusBadRequest, "x.csv", []byte("Colonia,Lat\nCentro,32\n"), "")
		if msg := decode[httpapi.ErrorResponse](t, rec).Message; !strings.Contains(msg, "faltan columnas del formato") {
			t.Errorf("message = %q", msg)
		}
		tc.importarEsperando(http.StatusRequestEntityTooLarge, "grande.csv", bytes.Repeat([]byte("a"), 1<<20), "")
	})

	t.Run("exportar", func(t *testing.T) {
		tc := *admin
		tc.t = t
		// Un reporte marcado a mano también se exporta: el riesgo junta las dos fuentes.
		r := decode[httpapi.Reporte](t, tc.expect(http.StatusCreated, http.MethodPost, "/v1/reportes", map[string]any{
			"incidente_id": inc.Id, "latitude": 32.4501, "longitude": -116.9001, "direccion": "Calle 1 · Colonia Reporte",
		}))
		tc.expect(http.StatusOK, http.MethodPut, "/v1/reportes/"+r.Id.String()+"/historico", nil)

		rec := tc.expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/v1/riesgo/exportar?incidente_id=%d", inc.Id), nil)
		if ct, cd := rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"); ct != "text/csv" ||
			!strings.HasPrefix(cd, `attachment; filename="riesgo-historico-`) {
			t.Errorf("Content-Type = %q, Content-Disposition = %q", ct, cd)
		}
		filas, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(rec.Body.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(filas) != 8 || filas[0][0] != "Colonia" || filas[0][8] != "Dataset" {
			t.Fatalf("CSV = %v, quería encabezado + 7 filas", filas)
		}
		// Ordenado por fecha: el primero es el de 1998.
		if got := strings.Join(filas[1], "|"); got != "Colonia Dataset|32.4502|-116.9004|1998-02-01|"+inc.Etiqueta+"|0||dataset|lluvias.csv" {
			t.Errorf("primera fila = %s", got)
		}
		origenes := map[string]int{}
		for _, f := range filas[1:] {
			origenes[f[7]]++
		}
		if origenes["dataset"] != 6 || origenes["reporte"] != 1 {
			t.Errorf("origenes = %v, quería 6 de dataset y 1 de reporte", origenes)
		}

		x := tc.expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/v1/riesgo/exportar?formato=xlsx&incidente_id=%d", inc.Id), nil)
		if ct := x.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || !bytes.HasPrefix(x.Body.Bytes(), []byte("PK")) {
			t.Errorf("xlsx: Content-Type = %q", ct)
		}
		tc.expect(http.StatusBadRequest, http.MethodGet, "/v1/riesgo/exportar?formato=pdf", nil)

		// Lo exportado se vuelve a importar (es otro archivo: lleva el reporte).
		re := decode[httpapi.DatasetHistorico](t, tc.importarEsperando(http.StatusCreated, "exportado.xlsx", x.Body.Bytes(), "Reimportado"))
		if re.Filas != 7 {
			t.Errorf("reimportado: filas = %d, quería 7", re.Filas)
		}
		tc.expect(http.StatusNoContent, http.MethodDelete, "/v1/riesgo/datasets/"+re.Id.String(), nil)
		tc.expect(http.StatusOK, http.MethodDelete, "/v1/reportes/"+r.Id.String()+"/historico", nil)
	})

	t.Run("roles", func(t *testing.T) {
		ciudadano := newTestClient(t)
		ciudadano.login()
		ciudadano.expect(http.StatusForbidden, http.MethodGet, "/v1/riesgo/datasets", nil)
		ciudadano.expect(http.StatusForbidden, http.MethodGet, "/v1/riesgo/exportar", nil)
		ciudadano.expect(http.StatusForbidden, http.MethodDelete, "/v1/riesgo/datasets/"+d.Id.String(), nil)
		if rec := ciudadano.importar("a.csv", []byte(archivo), ""); rec.Code != http.StatusForbidden {
			t.Errorf("ciudadano importando = %d, quería 403", rec.Code)
		}
		for _, rol := range []string{"moderador", "analista", "proteccion_civil", "rescatista"} {
			c := newTestClient(t)
			c.loginComo(rol)
			c.expect(http.StatusOK, http.MethodGet, "/v1/riesgo/datasets", nil)
			c.expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/v1/riesgo/exportar?incidente_id=%d", inc.Id), nil)
		}
		analista := newTestClient(t)
		analista.loginComo("analista")
		otro := "Latitud,Longitud,Fecha,Categoria\n32.45,-116.9,2023-01," + inc.Nombre + "\n"
		od := decode[httpapi.DatasetHistorico](t, analista.importarEsperando(http.StatusCreated, "analista.csv", []byte(otro), ""))
		analista.expect(http.StatusNoContent, http.MethodDelete, "/v1/riesgo/datasets/"+od.Id.String(), nil)
	})

	t.Run("eliminar", func(t *testing.T) {
		tc := *admin
		tc.t = t
		tc.expect(http.StatusNoContent, http.MethodDelete, "/v1/riesgo/datasets/"+d.Id.String(), nil)
		if n := contarHistorial(t, "riesgo.eliminar_dataset", "dataset_historico", d.Id.String()); n != 1 {
			t.Errorf("eventos de eliminar_dataset = %d, quería 1", n)
		}
		if r := resumen(&tc); r.TotalHistoricos != 0 {
			t.Errorf("total_historicos = %d tras borrar el dataset, quería 0", r.TotalHistoricos)
		}
		tc.expect(http.StatusNotFound, http.MethodDelete, "/v1/riesgo/datasets/"+d.Id.String(), nil)
		tc.expect(http.StatusNotFound, http.MethodDelete, "/v1/riesgo/datasets/"+uuid.NewString(), nil)
		tc.expect(http.StatusBadRequest, http.MethodDelete, "/v1/riesgo/datasets/no-es-uuid", nil)
		// Borrado, el mismo archivo se puede volver a importar.
		again := decode[httpapi.DatasetHistorico](t, tc.importarEsperando(http.StatusCreated, "lluvias.csv", []byte(archivo), ""))
		tc.expect(http.StatusNoContent, http.MethodDelete, "/v1/riesgo/datasets/"+again.Id.String(), nil)
	})
}
