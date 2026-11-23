package riesgo

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeStore devuelve filas fijas y guarda los parámetros recibidos para
// comprobar que el servicio pasa los filtros tal cual.
type fakeStore struct {
	celdas     []sqlc.ListCeldasCalorRow
	zonas      []sqlc.ListZonasRiesgoRow
	colonias   []sqlc.ListColoniasHistoricoRow
	categorias []sqlc.ListConteoCategoriasRow
	err        error

	argCeldas      *sqlc.ListCeldasCalorParams
	llamadasCeldas []sqlc.ListCeldasCalorParams
	argZonas       *sqlc.ListZonasRiesgoParams
	argColonias    *sqlc.ListColoniasHistoricoParams
	argCategorias  *sqlc.ListConteoCategoriasParams

	catalogo    []sqlc.ListCategoriasCatalogoRow
	datasets    []sqlc.DatasetHistorico
	exportar    []sqlc.ListExportarHistoricoRow
	errImportar error
	errEliminar error
	argImportar *ImportarTxParams
	argExportar *sqlc.ListExportarHistoricoParams
	eliminado   uuid.UUID

	// LeerMismaFoto: cuántas veces se abrió, si hay una abierta, cuántas
	// consultas del resumen corrieron fuera de ella y el error al abrirla.
	fotos       int
	enFoto      bool
	fueraDeFoto int
	errFoto     error
}

func (f *fakeStore) LeerMismaFoto(ctx context.Context, fn func(consultas) error) error {
	if f.errFoto != nil {
		return f.errFoto
	}
	f.fotos++
	f.enFoto = true
	defer func() { f.enFoto = false }()
	return fn(f)
}

func (f *fakeStore) contarFoto() {
	if !f.enFoto {
		f.fueraDeFoto++
	}
}

func (f *fakeStore) ListCeldasCalor(ctx context.Context, arg sqlc.ListCeldasCalorParams) ([]sqlc.ListCeldasCalorRow, error) {
	f.argCeldas = &arg
	f.llamadasCeldas = append(f.llamadasCeldas, arg)
	return f.celdas, f.err
}

func (f *fakeStore) ListZonasRiesgo(ctx context.Context, arg sqlc.ListZonasRiesgoParams) ([]sqlc.ListZonasRiesgoRow, error) {
	f.argZonas = &arg
	f.contarFoto()
	return f.zonas, f.err
}

func (f *fakeStore) ListColoniasHistorico(ctx context.Context, arg sqlc.ListColoniasHistoricoParams) ([]sqlc.ListColoniasHistoricoRow, error) {
	f.argColonias = &arg
	f.contarFoto()
	return f.colonias, f.err
}

func (f *fakeStore) ListConteoCategorias(ctx context.Context, arg sqlc.ListConteoCategoriasParams) ([]sqlc.ListConteoCategoriasRow, error) {
	f.argCategorias = &arg
	f.contarFoto()
	return f.categorias, f.err
}

func (f *fakeStore) ListCategoriasCatalogo(ctx context.Context) ([]sqlc.ListCategoriasCatalogoRow, error) {
	return f.catalogo, f.err
}

func (f *fakeStore) ListDatasetsHistoricos(ctx context.Context) ([]sqlc.DatasetHistorico, error) {
	return f.datasets, f.err
}

func (f *fakeStore) ListExportarHistorico(ctx context.Context, arg sqlc.ListExportarHistoricoParams) ([]sqlc.ListExportarHistoricoRow, error) {
	f.argExportar = &arg
	return f.exportar, f.err
}

// ImportarDataset devuelve errImportar si está puesto (como Postgres: un
// *pgconn.PgError 23505 para un sha256 repetido).
func (f *fakeStore) ImportarDataset(ctx context.Context, arg ImportarTxParams) (sqlc.DatasetHistorico, error) {
	f.argImportar = &arg
	if f.errImportar != nil {
		return sqlc.DatasetHistorico{}, f.errImportar
	}
	actor := arg.ActorID
	return sqlc.DatasetHistorico{ID: uuid.New(), Nombre: arg.Nombre, Archivo: arg.Archivo, Sha256: arg.Sha256, Filas: len(arg.Filas), CargadoPor: &actor}, nil
}

// EliminarDataset devuelve pgx.ErrNoRows si errEliminar lo pide, como DeleteDatasetHistorico.
func (f *fakeStore) EliminarDataset(ctx context.Context, id, actorID uuid.UUID) error {
	f.eliminado = id
	return f.errEliminar
}

var errUnique = &pgconn.PgError{Code: "23505"}
var areaTijuana = Area{MinLat: 32.4, MaxLat: 32.6, MinLng: -117.2, MaxLng: -116.8}

func fecha(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestTamanoCelda(t *testing.T) {
	casos := []struct {
		zoom float64
		want float64
	}{
		{0, CeldaMaxM},  // muy alejado: tope superior
		{10, 4126},      // 132030 / 1024 × 32
		{13, 515.7},     // 132030 / 8192 × 32
		{16, 64.47},     // 132030 / 65536 × 32
		{22, CeldaMinM}, // muy cerca: tope inferior
	}
	for _, c := range casos {
		if got := TamanoCelda(c.zoom); math.Abs(got-c.want) > 1 {
			t.Errorf("TamanoCelda(%v) = %v, want ≈ %v", c.zoom, got, c.want)
		}
	}
}

func TestScoreYNivel(t *testing.T) {
	casos := []struct {
		peso, max float64
		score     float64
		nivel     Nivel
	}{
		{10, 10, 1, NivelAlto},
		{6.6, 10, 0.66, NivelAlto},
		{6.5, 10, 0.65, NivelMedio},
		{3.3, 10, 0.33, NivelMedio},
		{3, 10, 0.3, NivelBajo},
		{0, 10, 0, NivelBajo},
		{5, 0, 0, NivelBajo}, // sin máximo no hay score
	}
	for _, c := range casos {
		s := Score(c.peso, c.max)
		if math.Abs(s-c.score) > 1e-9 {
			t.Errorf("Score(%v, %v) = %v, want %v", c.peso, c.max, s, c.score)
		}
		if n := NivelDe(s); n != c.nivel {
			t.Errorf("NivelDe(%v) = %v, want %v", s, n, c.nivel)
		}
	}
}

func TestService_ObtenerMapaCalor(t *testing.T) {
	f := &fakeStore{celdas: []sqlc.ListCeldasCalorRow{
		{Latitude: 32.5, Longitude: -117, Peso: 2.5, Reportes: 3, CeldaX: 500000, CeldaY: 3600000},
		{Latitude: 32.51, Longitude: -117.01, Peso: 7, Reportes: 7, CeldaX: 505000, CeldaY: 3600000},
	}}
	svc := NewService(f, zonaPrueba)
	filtro := Filtro{IncidenteIDs: []int{1, 6}, Desde: fecha("2025-01-01T00:00:00Z"), Hasta: fecha("2026-01-01T00:00:00Z")}

	mapa, err := svc.ObtenerMapaCalor(t.Context(), areaTijuana, 13, filtro)
	if err != nil {
		t.Fatalf("ObtenerMapaCalor() error = %v", err)
	}
	if mapa.PesoMaximo != 7 {
		t.Errorf("PesoMaximo = %v, want 7", mapa.PesoMaximo)
	}
	if len(mapa.Celdas) != 2 || mapa.Celdas[0].Reportes != 3 {
		t.Errorf("Celdas = %+v, want las 2 filas del store", mapa.Celdas)
	}
	if mapa.TamanoCeldaM != TamanoCelda(13) {
		t.Errorf("TamanoCeldaM = %v, want %v", mapa.TamanoCeldaM, TamanoCelda(13))
	}

	want := sqlc.ListCeldasCalorParams{
		TamanoCeldaM: TamanoCelda(13),
		MinLng:       -117.2, MinLat: 32.4, MaxLng: -116.8, MaxLat: 32.6,
		MaxInclusivo: true, // un solo tramo: incluye todo el área
		IncidenteIds: []int{1, 6},
		Desde:        filtro.Desde,
		Hasta:        filtro.Hasta,
	}
	if !reflect.DeepEqual(*f.argCeldas, want) {
		t.Errorf("ListCeldasCalor() arg = %+v, want %+v", *f.argCeldas, want)
	}
}

func TestPartirArea(t *testing.T) {
	if got := PartirArea(areaTijuana); !reflect.DeepEqual(got, []Area{areaTijuana}) {
		t.Errorf("PartirArea(Tijuana) = %+v, want el área sin partir", got)
	}

	// Con el mapa muy alejado: el mundo entero en 4 tramos de 90°, contiguos.
	mundo := Area{MinLat: -85, MaxLat: 85, MinLng: -180, MaxLng: 180}
	tramos := PartirArea(mundo)
	if len(tramos) != 4 {
		t.Fatalf("PartirArea(mundo) = %d tramos, want 4", len(tramos))
	}
	for i, tr := range tramos {
		if tr.MaxLng-tr.MinLng > AnchoMaxTramoGrados || tr.MinLat != -85 || tr.MaxLat != 85 {
			t.Errorf("tramo %d = %+v, want ≤ %v° y la latitud completa", i, tr, AnchoMaxTramoGrados)
		}
		if i > 0 && tr.MinLng != tramos[i-1].MaxLng {
			t.Errorf("tramo %d empieza en %v, want %v (contiguo)", i, tr.MinLng, tramos[i-1].MaxLng)
		}
	}
	if tramos[0].MinLng != -180 || tramos[3].MaxLng != 180 {
		t.Errorf("tramos = %+v, want de -180 a 180", tramos)
	}

	// Exactamente 180° ya da "Antipodal edge" en PostGIS: debe partirse.
	if n := len(PartirArea(Area{MinLng: -180, MaxLng: 0})); n != 2 {
		t.Errorf("PartirArea(180°) = %d tramos, want 2", n)
	}
}

func TestService_ObtenerMapaCalor_AreaAncha(t *testing.T) {
	// El store devuelve las mismas celdas en cada tramo: deben juntarse por su clave.
	f := &fakeStore{celdas: []sqlc.ListCeldasCalorRow{
		{Latitude: 32.5, Longitude: -117, Peso: 2.5, Reportes: 3, CeldaX: 500000, CeldaY: 3600000},
		{Latitude: 32.51, Longitude: -117.01, Peso: 7, Reportes: 7, CeldaX: 505000, CeldaY: 3600000},
	}}
	mapa, err := NewService(f, zonaPrueba).ObtenerMapaCalor(t.Context(), Area{MinLat: -85, MaxLat: 85, MinLng: -180, MaxLng: 180}, 0, Filtro{})
	if err != nil {
		t.Fatalf("ObtenerMapaCalor() error = %v", err)
	}
	if len(f.llamadasCeldas) != 4 {
		t.Fatalf("ListCeldasCalor llamado %d veces, want 4 (un tramo de 90° cada una)", len(f.llamadasCeldas))
	}
	// Los tramos no se solapan: solo el último incluye su borde derecho.
	for i, arg := range f.llamadasCeldas {
		if want := i == 3; arg.MaxInclusivo != want {
			t.Errorf("tramo %d: MaxInclusivo = %v, want %v", i, arg.MaxInclusivo, want)
		}
	}
	if len(mapa.Celdas) != 2 || mapa.Celdas[0].Reportes != 12 || mapa.Celdas[1].Peso != 28 {
		t.Errorf("Celdas = %+v, want 2 celdas con los 4 tramos sumados", mapa.Celdas)
	}
	if mapa.PesoMaximo != 28 {
		t.Errorf("PesoMaximo = %v, want 28", mapa.PesoMaximo)
	}
}

// Una celda partida entre dos tramos: su punto es el promedio de los reportes
// de las dos partes (por número de reportes), no el de la primera.
func TestService_ObtenerMapaCalor_CeldaEntreTramos(t *testing.T) {
	f := &fakeStoreTramos{filas: [][]sqlc.ListCeldasCalorRow{
		{{Latitude: 32.0, Longitude: -117.10, Peso: 1, Reportes: 1, CeldaX: 1, CeldaY: 2}},
		{{Latitude: 32.3, Longitude: -116.80, Peso: 3, Reportes: 3, CeldaX: 1, CeldaY: 2}},
	}}
	mapa, err := NewService(f, zonaPrueba).ObtenerMapaCalor(t.Context(), Area{MinLat: 0, MaxLat: 60, MinLng: -180, MaxLng: 0}, 0, Filtro{})
	if err != nil {
		t.Fatalf("ObtenerMapaCalor() error = %v", err)
	}
	if len(mapa.Celdas) != 1 {
		t.Fatalf("Celdas = %+v, want 1 (la misma clave en los dos tramos)", mapa.Celdas)
	}
	c := mapa.Celdas[0]
	if c.Reportes != 4 || c.Peso != 4 || math.Abs(c.Latitude-32.225) > 1e-9 || math.Abs(c.Longitude+116.875) > 1e-9 {
		t.Errorf("Celda = %+v, want 4 reportes, peso 4, en (32.225, -116.875)", c)
	}
}

// fakeStoreTramos devuelve una lista distinta de celdas en cada llamada (una
// por tramo).
type fakeStoreTramos struct {
	fakeStore
	filas [][]sqlc.ListCeldasCalorRow
}

func (f *fakeStoreTramos) ListCeldasCalor(ctx context.Context, arg sqlc.ListCeldasCalorParams) ([]sqlc.ListCeldasCalorRow, error) {
	i := len(f.llamadasCeldas)
	f.llamadasCeldas = append(f.llamadasCeldas, arg)
	if i < len(f.filas) {
		return f.filas[i], nil
	}
	return nil, nil
}

func TestService_ObtenerMapaCalor_SinCeldas(t *testing.T) {
	svc := NewService(&fakeStore{}, zonaPrueba)
	mapa, err := svc.ObtenerMapaCalor(t.Context(), areaTijuana, 12, Filtro{IncidenteIDs: []int{}})
	if err != nil {
		t.Fatalf("ObtenerMapaCalor() error = %v", err)
	}
	if mapa.Celdas == nil || len(mapa.Celdas) != 0 || mapa.PesoMaximo != 0 {
		t.Errorf("ObtenerMapaCalor() = %+v, want celdas vacías (no nil) y peso 0", mapa)
	}
}

func TestService_IncidentesVaciosEsSinFiltro(t *testing.T) {
	f := &fakeStore{}
	svc := NewService(f, zonaPrueba)
	if _, err := svc.ObtenerMapaCalor(t.Context(), areaTijuana, 12, Filtro{IncidenteIDs: []int{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ObtenerResumen(t.Context(), Filtro{IncidenteIDs: []int{}}); err != nil {
		t.Fatal(err)
	}
	if f.argCeldas.IncidenteIds != nil || f.argZonas.IncidenteIds != nil ||
		f.argColonias.IncidenteIds != nil || f.argCategorias.IncidenteIds != nil {
		t.Errorf("IncidenteIds = [] llegó como arreglo vacío, want nil (sin filtro)")
	}
}

func TestService_Validaciones(t *testing.T) {
	svc := NewService(&fakeStore{}, zonaPrueba)
	ctx := t.Context()

	invertida := Area{MinLat: 32.6, MaxLat: 32.4, MinLng: -117.2, MaxLng: -116.8}
	if _, err := svc.ObtenerMapaCalor(ctx, invertida, 12, Filtro{}); !errors.Is(err, ErrAreaInvalida) {
		t.Errorf("ObtenerMapaCalor(lat invertida) = %v, want ErrAreaInvalida", err)
	}
	invertida = Area{MinLat: 32.4, MaxLat: 32.6, MinLng: -116.8, MaxLng: -117.2}
	if _, err := svc.ObtenerMapaCalor(ctx, invertida, 12, Filtro{}); !errors.Is(err, ErrAreaInvalida) {
		t.Errorf("ObtenerMapaCalor(lng invertida) = %v, want ErrAreaInvalida", err)
	}

	periodo := Filtro{Desde: fecha("2026-02-01T00:00:00Z"), Hasta: fecha("2026-01-01T00:00:00Z")}
	if _, err := svc.ObtenerMapaCalor(ctx, areaTijuana, 12, periodo); !errors.Is(err, ErrPeriodoInvalido) {
		t.Errorf("ObtenerMapaCalor(periodo invertido) = %v, want ErrPeriodoInvalido", err)
	}
	if _, err := svc.ObtenerResumen(ctx, periodo); !errors.Is(err, ErrPeriodoInvalido) {
		t.Errorf("ObtenerResumen(periodo invertido) = %v, want ErrPeriodoInvalido", err)
	}

	// incidentes.id es int4: un id fuera de rango es un 400, sin llegar a la base.
	for _, id := range []int{math.MaxInt32 + 1, math.MinInt32 - 1} {
		f := &fakeStore{}
		fuera := Filtro{IncidenteIDs: []int{1, id}}
		if _, err := NewService(f, zonaPrueba).ObtenerMapaCalor(ctx, areaTijuana, 12, fuera); !errors.Is(err, ErrIncidenteInvalido) {
			t.Errorf("ObtenerMapaCalor(incidente %d) = %v, want ErrIncidenteInvalido", id, err)
		}
		if _, err := NewService(f, zonaPrueba).ObtenerResumen(ctx, fuera); !errors.Is(err, ErrIncidenteInvalido) {
			t.Errorf("ObtenerResumen(incidente %d) = %v, want ErrIncidenteInvalido", id, err)
		}
		if f.argCeldas != nil || f.fotos != 0 {
			t.Errorf("incidente %d: se consultó la base, want rechazarlo antes", id)
		}
	}
	if _, err := svc.ObtenerResumen(ctx, Filtro{IncidenteIDs: []int{math.MaxInt32, math.MinInt32}}); err != nil {
		t.Errorf("ObtenerResumen(límites de int4) = %v, want nil", err)
	}

	mismoDia := Filtro{Desde: fecha("2026-01-01T00:00:00Z"), Hasta: fecha("2026-01-01T00:00:00Z")}
	if _, err := svc.ObtenerResumen(ctx, mismoDia); err != nil {
		t.Errorf("ObtenerResumen(desde == hasta) = %v, want nil", err)
	}
}

func TestService_ObtenerResumen(t *testing.T) {
	f := &fakeStore{
		zonas: []sqlc.ListZonasRiesgoRow{
			{Latitude: 32.5, Longitude: -117, RadioM: 300, Colonia: "Cañón del Sáinz", Reportes: 12, Peso: 12},
			{Latitude: 32.4, Longitude: -116.9, RadioM: 200, Colonia: "", Reportes: 6, Peso: 5},
			{Latitude: 32.3, Longitude: -116.8, RadioM: 150, Colonia: "Obrera", Reportes: 4, Peso: 2},
		},
		colonias: []sqlc.ListColoniasHistoricoRow{
			{Colonia: "Bustamante", Reportes: 8, Peso: 8},
			{Colonia: "La Morita", Reportes: 6, Peso: 6},
		},
		categorias: []sqlc.ListConteoCategoriasRow{
			{IncidenteID: 1, Total: 350},
			{IncidenteID: 2, Total: 27},
		},
	}
	svc := NewService(f, zonaPrueba)
	ahora := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	svc.ahora = func() time.Time { return ahora }

	filtro := Filtro{IncidenteIDs: []int{1}, Desde: fecha("2026-01-01T00:00:00Z")}
	r, err := svc.ObtenerResumen(t.Context(), filtro)
	if err != nil {
		t.Fatalf("ObtenerResumen() error = %v", err)
	}

	if !r.GeneradoEn.Equal(ahora) || r.VersionModelo != VersionModelo {
		t.Errorf("GeneradoEn, VersionModelo = %v, %v, want %v, %v", r.GeneradoEn, r.VersionModelo, ahora, VersionModelo)
	}
	if r.TotalHistoricos != 377 {
		t.Errorf("TotalHistoricos = %v, want 377 (suma de categorías)", r.TotalHistoricos)
	}

	wantZonas := []struct {
		nombre string
		nivel  Nivel
	}{
		{"Cañón del Sáinz", NivelAlto},     // 12/12
		{NombreZonaSinColonia, NivelMedio}, // 5/12 = 0.42
		{"Obrera", NivelBajo},              // 2/12 = 0.17
	}
	if len(r.Zonas) != len(wantZonas) {
		t.Fatalf("len(Zonas) = %d, want %d", len(r.Zonas), len(wantZonas))
	}
	for i, w := range wantZonas {
		if r.Zonas[i].Nombre != w.nombre || r.Zonas[i].Nivel != w.nivel {
			t.Errorf("Zonas[%d] = %q %v, want %q %v", i, r.Zonas[i].Nombre, r.Zonas[i].Nivel, w.nombre, w.nivel)
		}
	}
	if r.ZonasAltoRiesgo != 1 {
		t.Errorf("ZonasAltoRiesgo = %v, want 1", r.ZonasAltoRiesgo)
	}
	if r.Zonas[0].RadioM != 300 || r.Zonas[0].Reportes != 12 {
		t.Errorf("Zonas[0] = %+v, want radio 300 y 12 reportes", r.Zonas[0])
	}

	if len(r.Colonias) != 2 || r.Colonias[0].Score != 1 || r.Colonias[1].Nivel != NivelAlto {
		t.Errorf("Colonias = %+v, want Bustamante 1.0 y La Morita 0.75 (Alto)", r.Colonias)
	}
	if len(r.Categorias) != 2 || r.Categorias[0] != (ConteoCategoria{IncidenteID: 1, Total: 350}) {
		t.Errorf("Categorias = %+v", r.Categorias)
	}

	if f.fotos != 1 || f.fueraDeFoto != 0 {
		t.Errorf("fotos = %d, consultas fuera de la foto = %d, want 1 y 0 (las tres en la misma)", f.fotos, f.fueraDeFoto)
	}

	// Parámetros del modelo y filtros, tal cual.
	if a := f.argZonas; a.EpsM != EpsM || a.MinPuntos != MinPuntos || a.Limite != LimiteZonas ||
		!reflect.DeepEqual(a.IncidenteIds, []int{1}) || a.Desde != filtro.Desde || a.Hasta != nil {
		t.Errorf("ListZonasRiesgo() arg = %+v", *a)
	}
	if a := f.argColonias; a.Limite != LimiteColonias || a.Desde != filtro.Desde {
		t.Errorf("ListColoniasHistorico() arg = %+v", *a)
	}
	if a := f.argCategorias; !reflect.DeepEqual(a.IncidenteIds, []int{1}) || a.Desde != filtro.Desde {
		t.Errorf("ListConteoCategorias() arg = %+v", *a)
	}
}

func TestService_ObtenerResumen_Vacio(t *testing.T) {
	r, err := NewService(&fakeStore{}, zonaPrueba).ObtenerResumen(t.Context(), Filtro{})
	if err != nil {
		t.Fatalf("ObtenerResumen() error = %v", err)
	}
	if r.Zonas == nil || r.Colonias == nil || r.Categorias == nil {
		t.Errorf("ObtenerResumen() = %+v, want listas vacías (no nil) para que el JSON lleve []", r)
	}
	if r.TotalHistoricos != 0 || r.ZonasAltoRiesgo != 0 {
		t.Errorf("totales = %d, %d, want 0, 0", r.TotalHistoricos, r.ZonasAltoRiesgo)
	}
}

func TestService_ErrorDelStore(t *testing.T) {
	boom := errors.New("conexión perdida")
	svc := NewService(&fakeStore{err: boom}, zonaPrueba)
	if _, err := svc.ObtenerMapaCalor(t.Context(), areaTijuana, 12, Filtro{}); !errors.Is(err, boom) {
		t.Errorf("ObtenerMapaCalor() = %v, want el error del store", err)
	}
	if _, err := svc.ObtenerResumen(t.Context(), Filtro{}); !errors.Is(err, boom) {
		t.Errorf("ObtenerResumen() = %v, want el error del store", err)
	}
	// Si no se puede abrir la transacción, el error llega igual.
	if _, err := NewService(&fakeStore{errFoto: boom}, zonaPrueba).ObtenerResumen(t.Context(), Filtro{}); !errors.Is(err, boom) {
		t.Errorf("ObtenerResumen() = %v, want el error al abrir la foto", err)
	}
}

var catalogoSQL = []sqlc.ListCategoriasCatalogoRow{{ID: 5, Nombre: "deslave", Etiqueta: "Deslave"}, {ID: 6, Nombre: "inundacion", Etiqueta: "Inundación"}}

const csvValido = "Colonia,Latitud,Longitud,Mes/año del Incidente,Categoria\nPopotla,32.29,-117.04,2023-08,Inundación\n"

func TestService_ImportarDataset(t *testing.T) {
	f := &fakeStore{catalogo: catalogoSQL}
	s := NewService(f, zonaPrueba)
	actor := uuid.New()

	d, err := s.ImportarDataset(context.Background(), actor, "", "lluvias.csv", []byte(csvValido))
	if err != nil {
		t.Fatalf("ImportarDataset: %v", err)
	}
	if d.Nombre != "lluvias.csv" || d.Archivo != "lluvias.csv" || d.Filas != 1 || d.CargadoPor == nil || *d.CargadoPor != actor {
		t.Errorf("dataset = %+v", d)
	}
	arg := f.argImportar
	if arg.ActorID != actor || len(arg.Sha256) != 64 || len(arg.Filas) != 1 || arg.Filas[0].IncidenteID != 6 {
		t.Errorf("parámetros = %+v", arg)
	}
}

func TestService_ImportarDataset_Errores(t *testing.T) {
	ctx := context.Background()

	dup := NewService(&fakeStore{catalogo: catalogoSQL, errImportar: errUnique}, zonaPrueba)
	if _, err := dup.ImportarDataset(ctx, uuid.New(), "", "a.csv", []byte(csvValido)); !errors.Is(err, ErrDatasetDuplicado) {
		t.Errorf("sha256 repetido: err = %v, quería ErrDatasetDuplicado", err)
	}

	f := &fakeStore{catalogo: catalogoSQL}
	_, err := NewService(f, zonaPrueba).ImportarDataset(ctx, uuid.New(), "", "a.csv", []byte("Latitud,Longitud,Fecha,Categoria\n32,-117,2023-01,Otro\n"))
	var inv *FilasInvalidasError
	if !errors.As(err, &inv) || inv.Total != 1 {
		t.Errorf("fila inválida: err = %v, quería *FilasInvalidasError", err)
	}
	if f.argImportar != nil {
		t.Error("con filas inválidas no debe guardar nada")
	}

	if _, err := NewService(&fakeStore{catalogo: catalogoSQL}, zonaPrueba).ImportarDataset(ctx, uuid.New(), "", "a.csv", []byte("hola")); !errors.Is(err, ErrFormatoInvalido) {
		t.Errorf("sin encabezados: err = %v, quería ErrFormatoInvalido", err)
	}
}

func TestService_EliminarDataset(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	f := &fakeStore{}
	if err := NewService(f, zonaPrueba).EliminarDataset(ctx, id, uuid.New()); err != nil || f.eliminado != id {
		t.Errorf("err = %v, eliminado = %v", err, f.eliminado)
	}
	if err := NewService(&fakeStore{errEliminar: pgx.ErrNoRows}, zonaPrueba).EliminarDataset(ctx, id, uuid.New()); !errors.Is(err, ErrDatasetNoEncontrado) {
		t.Errorf("err = %v, quería ErrDatasetNoEncontrado", err)
	}
}

func TestService_Exportar(t *testing.T) {
	f := &fakeStore{exportar: []sqlc.ListExportarHistoricoRow{{
		Origen: "dataset", Dataset: "Lluvias", Colonia: "Popotla", Latitude: 32.29, Longitude: -117.04,
		CreatedAt: time.Date(2023, 8, 1, 19, 0, 0, 0, time.UTC), Categoria: "Inundación", PersonasAfectadas: ptr(21),
	}}}
	s := NewService(f, zonaPrueba)
	s.ahora = func() time.Time { return time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC) }
	filtro := Filtro{IncidenteIDs: []int{6}, Desde: fecha("2020-01-01T00:00:00Z")}

	a, err := s.Exportar(context.Background(), filtro, FormatoCSV)
	if err != nil {
		t.Fatalf("Exportar: %v", err)
	}
	want := "\xEF\xBB\xBFColonia,Latitud,Longitud,Mes/año del Incidente,Categoria,Personas afectadas (total),Fallecidos,Origen,Dataset\n" +
		"Popotla,32.29,-117.04,2023-08-01,Inundación,21,,dataset,Lluvias\n"
	if a.Nombre != "riesgo-historico-2026-11-19.csv" || a.ContentType != "text/csv" || string(a.Contenido) != want {
		t.Errorf("archivo = %s %s\n%q", a.Nombre, a.ContentType, a.Contenido)
	}
	if !reflect.DeepEqual(f.argExportar.IncidenteIds, []int{6}) || f.argExportar.Desde != filtro.Desde || f.argExportar.Hasta != nil {
		t.Errorf("filtro = %+v", f.argExportar)
	}

	x, err := s.Exportar(context.Background(), Filtro{}, FormatoXLSX)
	if err != nil || x.Nombre != "riesgo-historico-2026-11-19.xlsx" || !bytes.HasPrefix(x.Contenido, []byte("PK")) {
		t.Errorf("xlsx: err = %v, nombre = %s", err, x.Nombre)
	}

	if _, err := s.Exportar(context.Background(), Filtro{Desde: fecha("2026-02-01T00:00:00Z"), Hasta: fecha("2026-01-01T00:00:00Z")}, FormatoCSV); !errors.Is(err, ErrPeriodoInvalido) {
		t.Errorf("periodo invertido: err = %v", err)
	}
}
