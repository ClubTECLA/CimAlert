package riesgo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/google/uuid"
)

// consultas declara solo las consultas que usa el servicio, con las firmas
// exactas de sqlc; *sqlc.Queries la cumple sin adaptadores.
type consultas interface {
	ListCeldasCalor(ctx context.Context, arg sqlc.ListCeldasCalorParams) ([]sqlc.ListCeldasCalorRow, error)
	ListZonasRiesgo(ctx context.Context, arg sqlc.ListZonasRiesgoParams) ([]sqlc.ListZonasRiesgoRow, error)
	ListColoniasHistorico(ctx context.Context, arg sqlc.ListColoniasHistoricoParams) ([]sqlc.ListColoniasHistoricoRow, error)
	ListConteoCategorias(ctx context.Context, arg sqlc.ListConteoCategoriasParams) ([]sqlc.ListConteoCategoriasRow, error)
	ListCategoriasCatalogo(ctx context.Context) ([]sqlc.ListCategoriasCatalogoRow, error)
	ListDatasetsHistoricos(ctx context.Context) ([]sqlc.DatasetHistorico, error)
	ListExportarHistorico(ctx context.Context, arg sqlc.ListExportarHistoricoParams) ([]sqlc.ListExportarHistoricoRow, error)
}

// store agrega a las consultas la lectura en una sola foto de la base (ver
// Store.LeerMismaFoto) y las operaciones de datasets, que escriben en una
// transacción.
type store interface {
	consultas
	LeerMismaFoto(ctx context.Context, fn func(consultas) error) error
	ImportarDataset(ctx context.Context, arg ImportarTxParams) (sqlc.DatasetHistorico, error)
	EliminarDataset(ctx context.Context, id, actorID uuid.UUID) error
}

type Service struct {
	store store
	zona  Zona
	ahora func() time.Time
}

// NewService crea el servicio de riesgo con la store y la zona donde se
// aceptan las filas de los datasets importados.
func NewService(store store, zona Zona) *Service {
	return &Service{store: store, zona: zona, ahora: time.Now}
}

// ObtenerMapaCalor agrupa el peso de los reportes históricos del área en
// celdas cuyo tamaño depende del zoom.
func (s *Service) ObtenerMapaCalor(ctx context.Context, area Area, zoom float64, filtro Filtro) (MapaCalor, error) {
	if area.MinLat > area.MaxLat || area.MinLng > area.MaxLng {
		return MapaCalor{}, ErrAreaInvalida
	}
	if err := validarFiltro(filtro); err != nil {
		return MapaCalor{}, err
	}

	tamano := TamanoCelda(zoom)
	mapa := MapaCalor{TamanoCeldaM: tamano, Celdas: []CeldaCalor{}}
	// Una celda puede salir en dos tramos vecinos (sus reportes caen a ambos
	// lados del borde): se junta por su clave, que es el mismo nodo de la
	// cuadrícula en los dos, y su punto queda en el promedio de los reportes
	// de ambas partes. Cada reporte cuenta en un solo tramo (ver
	// MaxInclusivo), así que sumar no lo duplica.
	indice := map[[2]float64]int{}
	tramos := PartirArea(area)
	for i, tramo := range tramos {
		filas, err := s.store.ListCeldasCalor(ctx, sqlc.ListCeldasCalorParams{
			TamanoCeldaM: tamano,
			MinLng:       tramo.MinLng,
			MinLat:       tramo.MinLat,
			MaxLng:       tramo.MaxLng,
			MaxLat:       tramo.MaxLat,
			// Los tramos no se solapan: cada uno excluye su borde derecho,
			// salvo el último, que cierra el área pedida.
			MaxInclusivo: i == len(tramos)-1,
			IncidenteIds: idsONil(filtro.IncidenteIDs),
			Desde:        filtro.Desde,
			Hasta:        filtro.Hasta,
		})
		if err != nil {
			return MapaCalor{}, err
		}
		for _, f := range filas {
			clave := [2]float64{f.CeldaX, f.CeldaY}
			if i, ok := indice[clave]; ok {
				c := &mapa.Celdas[i]
				n, m := float64(c.Reportes), float64(f.Reportes)
				c.Latitude = (c.Latitude*n + f.Latitude*m) / (n + m)
				c.Longitude = (c.Longitude*n + f.Longitude*m) / (n + m)
				c.Peso += f.Peso
				c.Reportes += f.Reportes
				continue
			}
			indice[clave] = len(mapa.Celdas)
			mapa.Celdas = append(mapa.Celdas, CeldaCalor{
				Latitude:  f.Latitude,
				Longitude: f.Longitude,
				Peso:      f.Peso,
				Reportes:  f.Reportes,
			})
		}
	}
	for _, c := range mapa.Celdas {
		mapa.PesoMaximo = max(mapa.PesoMaximo, c.Peso)
	}
	return mapa, nil
}

// ObtenerResumen arma las zonas, las colonias y el conteo por categoría de
// toda la ciudad.
func (s *Service) ObtenerResumen(ctx context.Context, filtro Filtro) (Resumen, error) {
	if err := validarFiltro(filtro); err != nil {
		return Resumen{}, err
	}
	ids := idsONil(filtro.IncidenteIDs)

	// Las tres consultas leen la misma foto de la base: si no, un reporte que
	// pasa a histórico entre una y otra dejaría, por ejemplo, una zona de 4
	// reportes junto a total_historicos = 3.
	var (
		filasZonas      []sqlc.ListZonasRiesgoRow
		filasColonias   []sqlc.ListColoniasHistoricoRow
		filasCategorias []sqlc.ListConteoCategoriasRow
	)
	err := s.store.LeerMismaFoto(ctx, func(q consultas) error {
		var err error
		if filasZonas, err = q.ListZonasRiesgo(ctx, sqlc.ListZonasRiesgoParams{
			Limite:       LimiteZonas,
			IncidenteIds: ids,
			Desde:        filtro.Desde,
			Hasta:        filtro.Hasta,
			EpsM:         EpsM,
			MinPuntos:    MinPuntos,
		}); err != nil {
			return err
		}
		if filasColonias, err = q.ListColoniasHistorico(ctx, sqlc.ListColoniasHistoricoParams{
			IncidenteIds: ids,
			Desde:        filtro.Desde,
			Hasta:        filtro.Hasta,
			Limite:       LimiteColonias,
		}); err != nil {
			return err
		}
		filasCategorias, err = q.ListConteoCategorias(ctx, sqlc.ListConteoCategoriasParams{
			IncidenteIds: ids,
			Desde:        filtro.Desde,
			Hasta:        filtro.Hasta,
		})
		return err
	})
	if err != nil {
		return Resumen{}, err
	}

	r := Resumen{
		GeneradoEn:    s.ahora(),
		VersionModelo: VersionModelo,
		Zonas:         make([]ZonaRiesgo, 0, len(filasZonas)),
		Colonias:      make([]ColoniaRiesgo, 0, len(filasColonias)),
		Categorias:    make([]ConteoCategoria, 0, len(filasCategorias)),
	}

	maxZona := 0.0
	for _, z := range filasZonas {
		if z.Peso > maxZona {
			maxZona = z.Peso
		}
	}
	for _, z := range filasZonas {
		score := Score(z.Peso, maxZona)
		zona := ZonaRiesgo{
			Nombre:    NombreZona(z.Colonia),
			Latitude:  z.Latitude,
			Longitude: z.Longitude,
			RadioM:    z.RadioM,
			Score:     score,
			Nivel:     NivelDe(score),
			Reportes:  z.Reportes,
		}
		if zona.Nivel == NivelAlto {
			r.ZonasAltoRiesgo++
		}
		r.Zonas = append(r.Zonas, zona)
	}

	maxColonia := 0.0
	for _, c := range filasColonias {
		if c.Peso > maxColonia {
			maxColonia = c.Peso
		}
	}
	for _, c := range filasColonias {
		score := Score(c.Peso, maxColonia)
		r.Colonias = append(r.Colonias, ColoniaRiesgo{
			Nombre:   c.Colonia,
			Reportes: c.Reportes,
			Score:    score,
			Nivel:    NivelDe(score),
		})
	}

	for _, c := range filasCategorias {
		r.Categorias = append(r.Categorias, ConteoCategoria{IncidenteID: c.IncidenteID, Total: c.Total})
		r.TotalHistoricos += c.Total
	}
	return r, nil
}

// ImportarDataset valida el archivo completo y, si todas sus filas son
// válidas, lo guarda como un dataset nuevo. Si alguna fila es inválida
// devuelve *FilasInvalidasError y no guarda nada.
func (s *Service) ImportarDataset(ctx context.Context, actorID uuid.UUID, nombre, archivo string, contenido []byte) (Dataset, error) {
	filasCatalogo, err := s.store.ListCategoriasCatalogo(ctx)
	if err != nil {
		return Dataset{}, err
	}
	catalogo := make([]Categoria, len(filasCatalogo))
	for i, c := range filasCatalogo {
		catalogo[i] = Categoria{ID: c.ID, Nombre: c.Nombre, Etiqueta: c.Etiqueta}
	}

	filas, err := LeerDataset(contenido, catalogo, s.zona, s.ahora())
	if err != nil {
		return Dataset{}, err
	}

	suma := sha256.Sum256(contenido)
	d, err := s.store.ImportarDataset(ctx, ImportarTxParams{
		Nombre:  NombreDataset(nombre, archivo),
		Archivo: NombreDataset(archivo, ""),
		Sha256:  hex.EncodeToString(suma[:]),
		ActorID: actorID,
		Filas:   filas,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return Dataset{}, ErrDatasetDuplicado
		}
		return Dataset{}, err
	}
	return toDataset(d), nil
}

// ListarDatasets devuelve los datasets importados, del más reciente al más antiguo.
func (s *Service) ListarDatasets(ctx context.Context) ([]Dataset, error) {
	filas, err := s.store.ListDatasetsHistoricos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Dataset, len(filas))
	for i, d := range filas {
		out[i] = toDataset(d)
	}
	return out, nil
}

// EliminarDataset borra el dataset y todos sus incidentes.
func (s *Service) EliminarDataset(ctx context.Context, id, actorID uuid.UUID) error {
	if err := s.store.EliminarDataset(ctx, id, actorID); err != nil {
		if db.IsNotFound(err) {
			return ErrDatasetNoEncontrado
		}
		return err
	}
	return nil
}

// Exportar arma el archivo con los incidentes históricos de las dos fuentes
// que cumplen el filtro.
func (s *Service) Exportar(ctx context.Context, filtro Filtro, formato Formato) (Archivo, error) {
	if err := validarFiltro(filtro); err != nil {
		return Archivo{}, err
	}
	filas, err := s.store.ListExportarHistorico(ctx, sqlc.ListExportarHistoricoParams{
		IncidenteIds: idsONil(filtro.IncidenteIDs),
		Desde:        filtro.Desde,
		Hasta:        filtro.Hasta,
	})
	if err != nil {
		return Archivo{}, err
	}
	exportadas := make([]FilaExportada, len(filas))
	for i, f := range filas {
		exportadas[i] = FilaExportada{
			Colonia:           f.Colonia,
			Latitude:          f.Latitude,
			Longitude:         f.Longitude,
			Fecha:             f.CreatedAt,
			Categoria:         f.Categoria,
			PersonasAfectadas: f.PersonasAfectadas,
			Fallecidos:        f.Fallecidos,
			Origen:            f.Origen,
			Dataset:           f.Dataset,
		}
	}

	nombre := fmt.Sprintf("riesgo-historico-%s", s.ahora().In(zonaTijuana).Format("2006-01-02"))
	if formato == FormatoXLSX {
		contenido, err := EscribirXLSX(exportadas)
		if err != nil {
			return Archivo{}, err
		}
		return Archivo{Nombre: nombre + ".xlsx", ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Contenido: contenido}, nil
	}
	contenido, err := EscribirCSV(exportadas)
	if err != nil {
		return Archivo{}, err
	}
	return Archivo{Nombre: nombre + ".csv", ContentType: "text/csv", Contenido: contenido}, nil
}

func toDataset(d sqlc.DatasetHistorico) Dataset {
	return Dataset{
		ID:         d.ID,
		Nombre:     d.Nombre,
		Archivo:    d.Archivo,
		Filas:      d.Filas,
		CargadoPor: d.CargadoPor,
		CreatedAt:  d.CreatedAt,
	}
}

func validarFiltro(f Filtro) error {
	if f.Desde != nil && f.Hasta != nil && f.Desde.After(*f.Hasta) {
		return ErrPeriodoInvalido
	}
	for _, id := range f.IncidenteIDs {
		// incidentes.id es integer (int4): un id fuera de rango no puede
		// existir, y pgx no lo sabría codificar (sería un 500 en vez de un 400).
		if id < math.MinInt32 || id > math.MaxInt32 {
			return ErrIncidenteInvalido
		}
	}
	return nil
}

// idsONil convierte una lista vacía en nil: en las consultas, NULL significa
// "sin filtro", mientras que un arreglo vacío no dejaría pasar nada.
func idsONil(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	return ids
}
