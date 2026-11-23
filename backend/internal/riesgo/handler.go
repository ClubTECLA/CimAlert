package riesgo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"

	"github.com/ClubTECLA/cimalert/backend/internal/httpapi"
	"github.com/ClubTECLA/cimalert/backend/internal/httperr"
	"github.com/ClubTECLA/cimalert/backend/internal/middleware"
)

// Campos del multipart de la importación.
const (
	campoArchivo = "file"
	campoNombre  = "nombre"
	// maxCampoNombre corta el campo nombre al leerlo; el contrato permite 200
	// caracteres (hasta 4 bytes cada uno en UTF-8).
	maxCampoNombre = 1 << 10
)

// Handler implementa las operaciones de riesgo del contrato; el servidor lo
// embebe junto con los de las demás funcionalidades.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func normalizarArea(a Area, zoom float64) (Area, float64, error) {
	for _, v := range []float64{a.MinLat, a.MaxLat, a.MinLng, a.MaxLng, zoom} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Area{}, 0, ErrAreaInvalida
		}
	}
	a.MinLat, a.MaxLat = max(a.MinLat, -90), min(a.MaxLat, 90)
	a.MinLng, a.MaxLng = max(a.MinLng, -180), min(a.MaxLng, 180)
	if a.MinLat > a.MaxLat || a.MinLng > a.MaxLng {
		return Area{}, 0, ErrAreaInvalida
	}
	return a, min(max(zoom, 0), 24), nil
}

// ObtenerMapaCalor devuelve el peso de los reportes del área visible,
// agrupado en celdas según el zoom.
func (h *Handler) ObtenerMapaCalor(ctx context.Context, request httpapi.ObtenerMapaCalorRequestObject) (httpapi.ObtenerMapaCalorResponseObject, error) {
	p := request.Params
	a, z, err := normalizarArea(Area{MinLat: p.MinLat, MaxLat: p.MaxLat, MinLng: p.MinLng, MaxLng: p.MaxLng}, p.Zoom)
	if err != nil {
		return nil, err
	}
	mapa, err := h.svc.ObtenerMapaCalor(ctx,
		a,
		z,
		Filtro{IncidenteIDs: deref(p.IncidenteId), Desde: p.Desde, Hasta: p.Hasta},
	)
	if err != nil {
		return nil, err
	}

	celdas := make([]httpapi.CeldaCalor, 0, len(mapa.Celdas))
	for _, c := range mapa.Celdas {
		celdas = append(celdas, httpapi.CeldaCalor{
			Latitude:  c.Latitude,
			Longitude: c.Longitude,
			Peso:      c.Peso,
			Reportes:  c.Reportes,
		})
	}
	return httpapi.ObtenerMapaCalor200JSONResponse{
		TamanoCeldaM: mapa.TamanoCeldaM,
		PesoMaximo:   mapa.PesoMaximo,
		Celdas:       celdas,
	}, nil
}

// ObtenerResumenRiesgo devuelve las zonas de mayor riesgo, las colonias con
// más incidentes y el conteo por tipo de incidente.
func (h *Handler) ObtenerResumenRiesgo(ctx context.Context, request httpapi.ObtenerResumenRiesgoRequestObject) (httpapi.ObtenerResumenRiesgoResponseObject, error) {
	p := request.Params
	r, err := h.svc.ObtenerResumen(ctx, Filtro{
		IncidenteIDs: deref(p.IncidenteId),
		Desde:        p.Desde,
		Hasta:        p.Hasta,
	})
	if err != nil {
		return nil, err
	}

	zonas := make([]httpapi.ZonaRiesgo, 0, len(r.Zonas))
	for _, z := range r.Zonas {
		zonas = append(zonas, httpapi.ZonaRiesgo{
			Nombre:    z.Nombre,
			Latitude:  z.Latitude,
			Longitude: z.Longitude,
			RadioM:    z.RadioM,
			Score:     z.Score,
			Nivel:     httpapi.NivelRiesgo(z.Nivel),
			Reportes:  z.Reportes,
		})
	}
	colonias := make([]httpapi.ColoniaRiesgo, 0, len(r.Colonias))
	for _, c := range r.Colonias {
		colonias = append(colonias, httpapi.ColoniaRiesgo{
			Nombre:   c.Nombre,
			Reportes: c.Reportes,
			Score:    c.Score,
			Nivel:    httpapi.NivelRiesgo(c.Nivel),
		})
	}
	categorias := make([]httpapi.ConteoCategoria, 0, len(r.Categorias))
	for _, c := range r.Categorias {
		categorias = append(categorias, httpapi.ConteoCategoria{IncidenteId: c.IncidenteID, Total: c.Total})
	}

	return httpapi.ObtenerResumenRiesgo200JSONResponse{
		GeneradoEn:      r.GeneradoEn,
		VersionModelo:   r.VersionModelo,
		TotalHistoricos: r.TotalHistoricos,
		ZonasAltoRiesgo: r.ZonasAltoRiesgo,
		Zonas:           zonas,
		Colonias:        colonias,
		Categorias:      categorias,
	}, nil
}

// ListarDatasetsHistoricos devuelve los datasets importados.
func (h *Handler) ListarDatasetsHistoricos(ctx context.Context, _ httpapi.ListarDatasetsHistoricosRequestObject) (httpapi.ListarDatasetsHistoricosResponseObject, error) {
	ds, err := h.svc.ListarDatasets(ctx)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListarDatasetsHistoricos200JSONResponse, len(ds))
	for i, d := range ds {
		out[i] = toDatasetHTTP(d)
	}
	return out, nil
}

// ImportarDatasetHistorico lee el multipart (file y, opcional, nombre) e
// importa el archivo. Si hay filas inválidas responde 422 con cada error.
func (h *Handler) ImportarDatasetHistorico(ctx context.Context, request httpapi.ImportarDatasetHistoricoRequestObject) (httpapi.ImportarDatasetHistoricoResponseObject, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, httperr.ErrNoAutenticado
	}

	var (
		contenido []byte
		archivo   string
		nombre    string
		hayFile   bool
	)
	for {
		parte, err := request.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return httpapi.ImportarDatasetHistorico400JSONResponse{Message: "petición multipart inválida"}, nil
		}
		switch parte.FormName() {
		case campoArchivo:
			// El body ya llegó limitado por middleware.BodyLimit.
			if contenido, err = io.ReadAll(parte); err != nil {
				return httpapi.ImportarDatasetHistorico400JSONResponse{Message: "petición multipart inválida"}, nil
			}
			archivo, hayFile = filepath.Base(parte.FileName()), true
		case campoNombre:
			b, err := io.ReadAll(io.LimitReader(parte, maxCampoNombre))
			if err != nil {
				return httpapi.ImportarDatasetHistorico400JSONResponse{Message: "petición multipart inválida"}, nil
			}
			nombre = string(bytes.ToValidUTF8(b, nil))
		}
		_ = parte.Close()
	}
	if !hayFile || len(contenido) == 0 {
		return httpapi.ImportarDatasetHistorico400JSONResponse{Message: "falta el campo file"}, nil
	}
	if archivo == "." || archivo == "/" {
		archivo = ""
	}

	d, err := h.svc.ImportarDataset(ctx, userID, nombre, archivo, contenido)
	var filasInvalidas *FilasInvalidasError
	switch {
	case errors.As(err, &filasInvalidas):
		errs := make([]httpapi.ErrorFilaImportacion, len(filasInvalidas.Errores))
		for i, e := range filasInvalidas.Errores {
			errs[i] = httpapi.ErrorFilaImportacion{Fila: e.Fila, Columna: e.Columna, Mensaje: e.Mensaje}
		}
		return httpapi.ImportarDatasetHistorico422JSONResponse{
			Message:      filasInvalidas.Error(),
			TotalErrores: filasInvalidas.Total,
			Errores:      errs,
		}, nil
	case errors.Is(err, ErrFormatoInvalido):
		// El mensaje dice qué está mal (p. ej. qué encabezados faltan).
		return httpapi.ImportarDatasetHistorico400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}
	return httpapi.ImportarDatasetHistorico201JSONResponse(toDatasetHTTP(d)), nil
}

// EliminarDatasetHistorico borra el dataset y sus incidentes.
func (h *Handler) EliminarDatasetHistorico(ctx context.Context, request httpapi.EliminarDatasetHistoricoRequestObject) (httpapi.EliminarDatasetHistoricoResponseObject, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, httperr.ErrNoAutenticado
	}
	if err := h.svc.EliminarDataset(ctx, request.DatasetId, userID); err != nil {
		return nil, err
	}
	return httpapi.EliminarDatasetHistorico204Response{}, nil
}

// ExportarRiesgo descarga los incidentes históricos en CSV o XLSX.
func (h *Handler) ExportarRiesgo(ctx context.Context, request httpapi.ExportarRiesgoRequestObject) (httpapi.ExportarRiesgoResponseObject, error) {
	p := request.Params
	formato := FormatoCSV
	if p.Formato != nil && *p.Formato == httpapi.Xlsx {
		formato = FormatoXLSX
	}
	a, err := h.svc.Exportar(ctx, Filtro{IncidenteIDs: deref(p.IncidenteId), Desde: p.Desde, Hasta: p.Hasta}, formato)
	if err != nil {
		return nil, err
	}
	headers := httpapi.ExportarRiesgo200ResponseHeaders{
		ContentDisposition: fmt.Sprintf("attachment; filename=%q", a.Nombre),
	}
	if formato == FormatoXLSX {
		return httpapi.ExportarRiesgo200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
			Body: bytes.NewReader(a.Contenido), Headers: headers, ContentLength: int64(len(a.Contenido)),
		}, nil
	}
	return httpapi.ExportarRiesgo200TextcsvResponse{
		Body: bytes.NewReader(a.Contenido), Headers: headers, ContentLength: int64(len(a.Contenido)),
	}, nil
}

func toDatasetHTTP(d Dataset) httpapi.DatasetHistorico {
	return httpapi.DatasetHistorico{
		Id:         d.ID,
		Nombre:     d.Nombre,
		Archivo:    d.Archivo,
		Filas:      d.Filas,
		CargadoPor: d.CargadoPor,
		CreatedAt:  d.CreatedAt,
	}
}

func deref(ids *[]int) []int {
	if ids == nil {
		return nil
	}
	return *ids
}
