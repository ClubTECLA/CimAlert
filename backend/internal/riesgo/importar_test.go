package riesgo

import (
	"bytes"
	"errors"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// catalogoPrueba es el catálogo del baseline (nombre, etiqueta).
var catalogoPrueba = []Categoria{
	{1, "luz", "Luz"}, {2, "socavon", "Socavón"}, {3, "arbol", "Árbol"}, {4, "drenaje", "Drenaje"},
	{5, "deslave", "Deslave"}, {6, "inundacion", "Inundación"}, {7, "incendio", "Incendio"},
}

var ahoraPrueba = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

// cajaPrueba es una zona rectangular para los tests del paquete; la zona real
// (geo.PoligonoBC) se prueba en la integración.
type cajaPrueba struct{ minLat, maxLat, minLng, maxLng float64 }

func (c cajaPrueba) Contiene(lat, lon float64) bool {
	return lat >= c.minLat && lat <= c.maxLat && lon >= c.minLng && lon <= c.maxLng
}

// zonaPrueba cubre Tijuana y Rosarito, pero no San Diego.
var zonaPrueba = cajaPrueba{32.0, 32.6, -117.3, -116.5}

func ptr[T any](v T) *T { return &v }

// xlsxPrueba arma un .xlsx como la hoja "Formato para importación": otra hoja
// antes, fechas como fecha de Excel y un conteo con fórmulas a la derecha.
// excelize, como Excel, escribe 15 dígitos significativos: las coordenadas de
// los tests de .xlsx no llevan más.
func xlsxPrueba(t *testing.T, filas [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Categorías"); err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Categorías", "A1", "Categorías más reportadas")
	hoja := "Formato para importación"
	if _, err := f.NewSheet(hoja); err != nil {
		t.Fatal(err)
	}
	formato := "mmm-yyyy"
	estilo, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &formato})
	encabezado := []any{"Colonia", "Latitud", "Longitud", "Mes/año del Incidente", "Categoria", "Personas afectadas (total)", "Fallecidos"}
	if err := f.SetSheetRow(hoja, "A1", &encabezado); err != nil {
		t.Fatal(err)
	}
	for i, fila := range filas {
		celda, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := f.SetSheetRow(hoja, celda, &fila); err != nil {
			t.Fatal(err)
		}
		fechaCelda, _ := excelize.CoordinatesToCellName(4, i+2)
		_ = f.SetCellStyle(hoja, fechaCelda, fechaCelda, estilo)
	}
	// Conteo con fórmulas fuera de la tabla, como en la hoja original.
	_ = f.SetCellValue(hoja, "I2", "Inundación")
	_ = f.SetCellFormula(hoja, "I3", "COUNTIF(E:E,I2)")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLeerDataset_XLSXDelFormato(t *testing.T) {
	contenido := xlsxPrueba(t, [][]any{
		{"Reforma (Colonia)", 32.3831417021218, -117.077865600586, time.Date(2017, 2, 1, 0, 0, 0, 0, time.UTC), "Inundación", 1, 1},
		{"Campo Alfonso", 32.3056175998308, -117.044504284859, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), "Derrumbe", 2, nil},
		{},
		{"Mision Viejo", 32.2024658720783, -116.910565495491, time.Date(2017, 2, 1, 0, 0, 0, 0, time.UTC), "SOCAVON", 0, nil},
	})

	filas, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil {
		t.Fatalf("LeerDataset: %v", err)
	}
	want := []FilaDataset{
		{Fila: 2, IncidenteID: 6, Colonia: ptr("Reforma (Colonia)"), Fecha: time.Date(2017, 2, 1, 0, 0, 0, 0, time.UTC),
			Latitude: 32.3831417021218, Longitude: -117.077865600586, PersonasAfectadas: ptr(1), Fallecidos: ptr(1)},
		// Derrumbe cuenta como deslave.
		{Fila: 3, IncidenteID: 5, Colonia: ptr("Campo Alfonso"), Fecha: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Latitude: 32.3056175998308, Longitude: -117.044504284859, PersonasAfectadas: ptr(2)},
		// El renglón 4 está vacío y se salta, sin mover los números.
		{Fila: 5, IncidenteID: 2, Colonia: ptr("Mision Viejo"), Fecha: time.Date(2017, 2, 1, 0, 0, 0, 0, time.UTC),
			Latitude: 32.2024658720783, Longitude: -116.910565495491, PersonasAfectadas: ptr(0)},
	}
	if !reflect.DeepEqual(filas, want) {
		t.Errorf("filas =\n%+v\nquería\n%+v", filas, want)
	}
}

func TestLeerDataset_RechazaFilasInvalidas(t *testing.T) {
	contenido := xlsxPrueba(t, [][]any{
		{"Echeverría", 32.37, -117.05, time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC), "Inundación", 2, nil},
		{"Sin fecha", 32.35, -117.05, nil, "Inundación", 0, nil},
		{"El Escondido", 32.36, -116.95, time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), "Otro", 0, nil},
		{"Fuera", 95.0, -117.05, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "Árbol", 1.5, -1},
		{"San Diego", 32.7157, -117.1611, time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC), "Inundación", 0, nil},
	})

	_, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
	var inv *FilasInvalidasError
	if !errors.As(err, &inv) {
		t.Fatalf("err = %v, quería *FilasInvalidasError", err)
	}
	want := []ErrorFila{
		{3, ColFecha, "falta la fecha"},
		{4, ColCategoria, `la categoría "Otro" no existe en el catálogo`},
		{5, ColLatitud, "la latitud debe ser un número entre -90 y 90"},
		{5, ColFecha, "la fecha es posterior al mes actual"},
		{5, ColPersonas, "las personas afectadas deben ser un número entero de 0 o más"},
		{5, ColFallecidos, "los fallecidos deben ser un número entero de 0 o más"},
		// Las dos coordenadas son válidas, pero la pareja cae fuera de la zona.
		{6, "", "la ubicación está fuera de la zona permitida (Baja California)"},
	}
	if !reflect.DeepEqual(inv.Errores, want) || inv.Total != len(want) {
		t.Errorf("errores = %+v (total %d)\nquería %+v", inv.Errores, inv.Total, want)
	}
}

func TestLeerDataset_CSV(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido []byte
	}{
		{"coma, con BOM", []byte("\xEF\xBB\xBFColonia,Latitud,Longitud,Mes/año del Incidente,Categoria,Personas afectadas (total),Fallecidos\n" +
			"\"Centro, Playas\",32.3466,-117.0614,1998-02,inundacion,21,\n")},
		// Excel en español: punto y coma, coma decimal y MM/AAAA.
		{"punto y coma", []byte("COLONIA;LATITUD;LONGITUD;MES/AÑO DEL INCIDENTE;CATEGORÍA;PERSONAS AFECTADAS (TOTAL);FALLECIDOS\r\n" +
			"Centro, Playas;32,3466;-117,0614;02/1998;Inundación;21;\r\n")},
		// Windows-1252 (Latin-1): "año" con ñ = 0xF1, "Inundación" con ó = 0xF3.
		{"latin-1", []byte("Colonia,Latitud,Longitud,Mes/a\xf1o del Incidente,Categoria,Personas afectadas (total),Fallecidos\n" +
			"\"Centro, Playas\",32.3466,-117.0614,15/02/1998,Inundaci\xf3n,21,\n")},
	}
	want := []FilaDataset{{Fila: 2, IncidenteID: 6, Colonia: ptr("Centro, Playas"), Fecha: time.Date(1998, 2, 1, 0, 0, 0, 0, time.UTC),
		Latitude: 32.3466, Longitude: -117.0614, PersonasAfectadas: ptr(21)}}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			filas, err := LeerDataset(c.contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
			if err != nil {
				t.Fatalf("LeerDataset: %v", err)
			}
			if !reflect.DeepEqual(filas, want) {
				t.Errorf("filas = %+v, quería %+v", filas, want)
			}
		})
	}
}

func TestLeerDataset_ColumnasOpcionalesYDeMas(t *testing.T) {
	// Sin Colonia, Personas ni Fallecidos, en otro orden y con una columna de más.
	contenido := []byte("Categoria,Notas,Longitud,Latitud,Fecha\nDeslave,cualquier cosa,-117.05,32.37,2024-01-15\n")
	filas, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil {
		t.Fatalf("LeerDataset: %v", err)
	}
	want := []FilaDataset{{Fila: 2, IncidenteID: 5, Fecha: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Latitude: 32.37, Longitude: -117.05}}
	if !reflect.DeepEqual(filas, want) {
		t.Errorf("filas = %+v, quería %+v", filas, want)
	}
}

func TestLeerDataset_ArchivoInvalido(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido []byte
		mensaje   string
	}{
		{"faltan encabezados", []byte("Colonia,Lat\nCentro,32\n"), "faltan columnas del formato: Longitud, Mes/año del Incidente, Categoria"},
		{"solo encabezado", []byte("Latitud,Longitud,Fecha,Categoria\n\n,,,\n"), "el archivo no tiene filas de datos"},
		{"vacío", []byte(""), "faltan columnas del formato: Latitud, Longitud, Mes/año del Incidente, Categoria"},
		{"binario", []byte{0x89, 'P', 'N', 'G', 0, 0, 0}, "el archivo no es un .xlsx ni un .csv"},
		{"zip que no es xlsx", []byte("PK\x03\x04basura"), "el archivo no es un .xlsx válido"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := LeerDataset(c.contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
			if !errors.Is(err, ErrFormatoInvalido) || err.Error() != c.mensaje {
				t.Errorf("err = %v, quería ErrFormatoInvalido %q", err, c.mensaje)
			}
		})
	}
}

func TestLeerDataset_XLSXSinEncabezados(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Colonia")
	buf, _ := f.WriteToBuffer()
	_, err := LeerDataset(buf.Bytes(), catalogoPrueba, zonaPrueba, ahoraPrueba)
	if !errors.Is(err, ErrFormatoInvalido) || !strings.Contains(err.Error(), "ninguna hoja tiene los encabezados") {
		t.Errorf("err = %v", err)
	}
}

// xlsxConRenglones arma un .xlsx con el encabezado del formato y n renglones
// de datos; extra pone además un valor en esa columna de cada renglón (0 = no).
func xlsxConRenglones(t *testing.T, n, extra int) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.SetRow("A1", []any{"Latitud", "Longitud", "Fecha", "Categoria"}); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= n+1; i++ {
		fila := []any{32.5, -117.05, "2024-01", "Socavón"}
		if extra > 0 {
			fila = append(fila, make([]any, extra-len(fila)-1)...)
			fila = append(fila, "x")
		}
		celda, _ := excelize.CoordinatesToCellName(1, i)
		if err := sw.SetRow(celda, fila); err != nil {
			t.Fatal(err)
		}
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Un valor en la última columna de Excel (XFD) hace que excelize rellene cada
// renglón con 16,384 celdas: un archivo de pocos KB pedía más de 1 GB.
func TestLeerDataset_XLSXRenglonDemasiadoAncho(t *testing.T) {
	contenido := xlsxConRenglones(t, 1000, 16384)
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	antes := m.TotalAlloc
	_, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
	runtime.ReadMemStats(&m)

	if !errors.Is(err, ErrFormatoInvalido) || err.Error() != `la hoja "Sheet1" tiene un renglón con más de 100 columnas` {
		t.Errorf("err = %v", err)
	}
	asignado := m.TotalAlloc - antes
	t.Logf("archivo de %d KB, asignó %d KB", len(contenido)>>10, asignado>>10)
	if asignado > 32<<20 {
		t.Errorf("asignó %d MB; el tope de columnas debería cortar antes", asignado>>20)
	}
}

// Con el tope justo, un valor en la columna 100 (fuera del formato) se ignora.
func TestLeerDataset_XLSXColumnaDeMasDentroDelTope(t *testing.T) {
	filas, err := LeerDataset(xlsxConRenglones(t, 2, xlsxMaxColumnas), catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil || len(filas) != 2 {
		t.Fatalf("filas = %d, err = %v", len(filas), err)
	}
}

// El límite de filas corta mientras se lee la hoja, no después de juntarla.
func TestLeerDataset_XLSXDemasiadasFilas(t *testing.T) {
	_, err := LeerDataset(xlsxConRenglones(t, MaxFilasDataset+1, 0), catalogoPrueba, zonaPrueba, ahoraPrueba)
	if !errors.Is(err, ErrFormatoInvalido) || err.Error() != "el archivo tiene más de 50000 filas" {
		t.Errorf("err = %v", err)
	}
}

// Un renglón que solo tiene algo en una columna de más (unas notas entre las
// del formato) no cuenta para el límite, igual que en LeerDataset: 50,000
// incidentes y una nota suelta se importan.
func TestLeerDataset_XLSXNotaNoCuentaComoFila(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if err := sw.SetRow("A1", []any{"Latitud", "Notas", "Longitud", "Fecha", "Categoria"}); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= MaxFilasDataset+1; i++ {
		celda, _ := excelize.CoordinatesToCellName(1, i)
		if err := sw.SetRow(celda, []any{32.5, nil, -117.05, "2024-01", "Socavón"}); err != nil {
			t.Fatal(err)
		}
	}
	celda, _ := excelize.CoordinatesToCellName(1, MaxFilasDataset+2)
	if err := sw.SetRow(celda, []any{nil, "revisar con protección civil"}); err != nil {
		t.Fatal(err)
	}
	if err := sw.Flush(); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}

	filas, err := LeerDataset(buf.Bytes(), catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil || len(filas) != MaxFilasDataset {
		t.Fatalf("filas = %d, err = %v", len(filas), err)
	}
}

func TestLeerDataset_DevuelveSoloLosPrimerosErrores(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("Latitud,Longitud,Fecha,Categoria\n")
	for range MaxErroresDevueltos + 20 {
		b.WriteString("32.5,-117,2024-01,Otro\n")
	}
	_, err := LeerDataset(b.Bytes(), catalogoPrueba, zonaPrueba, ahoraPrueba)
	var inv *FilasInvalidasError
	if !errors.As(err, &inv) || len(inv.Errores) != MaxErroresDevueltos || inv.Total != MaxErroresDevueltos+20 {
		t.Fatalf("err = %v", err)
	}
	if inv.Errores[0].Fila != 2 {
		t.Errorf("primer error en la fila %d, quería 2", inv.Errores[0].Fila)
	}
}

// Lo exportado se puede volver a importar: mismas filas (Origen y Dataset se ignoran).
func TestExportarSeVuelveAImportar(t *testing.T) {
	tijuana := func(y int, m time.Month) time.Time {
		return time.Date(y, m, 1, 12, 0, 0, 0, zonaTijuana)
	}
	exportadas := []FilaExportada{
		{Colonia: "Centro, Playas", Latitude: 32.3465962005551, Longitude: -117.06138074398, Fecha: tijuana(1998, 2),
			Categoria: "Inundación", PersonasAfectadas: ptr(21), Origen: "dataset", Dataset: "Lluvias"},
		{Colonia: "", Latitude: 32.5149, Longitude: -117.0382, Fecha: time.Date(2026, 3, 15, 23, 30, 0, 0, time.UTC),
			Categoria: "Socavón", Origen: "reporte"},
		// Una dirección que Excel tomaría como fórmula, y un reporte de las
		// 00:30 del 1 de junio en Tijuana (horario de verano, UTC-7): con una
		// zona fija de UTC-8 saldría el 31 de mayo.
		{Colonia: `=HYPERLINK("http://x","clic")`, Latitude: 32.5149, Longitude: -117.0382,
			Fecha: time.Date(2026, 6, 1, 7, 30, 0, 0, time.UTC), Categoria: "Inundación", Origen: "dataset", Dataset: "-Lluvias"},
		// Textos que ya empiezan con ' o con un carácter de fórmula: el CSV
		// los distingue de la protección y el .xlsx no los toca.
		{Colonia: "-Norte", Latitude: 32.5149, Longitude: -117.0382, Fecha: tijuana(2020, 1), Categoria: "Socavón", Origen: "reporte"},
		{Colonia: "'-Norte", Latitude: 32.5149, Longitude: -117.0382, Fecha: tijuana(2020, 1), Categoria: "Socavón", Origen: "reporte"},
		{Colonia: "'Apóstrofo", Latitude: 32.5149, Longitude: -117.0382, Fecha: tijuana(2020, 1), Categoria: "Socavón", Origen: "reporte"},
	}
	want := []FilaDataset{
		{Fila: 2, IncidenteID: 6, Colonia: ptr("Centro, Playas"), Fecha: time.Date(1998, 2, 1, 0, 0, 0, 0, time.UTC),
			Latitude: 32.3465962005551, Longitude: -117.06138074398, PersonasAfectadas: ptr(21)},
		{Fila: 3, IncidenteID: 2, Fecha: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Latitude: 32.5149, Longitude: -117.0382},
		{Fila: 4, IncidenteID: 6, Colonia: ptr(`=HYPERLINK("http://x","clic")`), Fecha: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			Latitude: 32.5149, Longitude: -117.0382},
		{Fila: 5, IncidenteID: 2, Colonia: ptr("-Norte"), Fecha: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Latitude: 32.5149, Longitude: -117.0382},
		{Fila: 6, IncidenteID: 2, Colonia: ptr("'-Norte"), Fecha: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Latitude: 32.5149, Longitude: -117.0382},
		{Fila: 7, IncidenteID: 2, Colonia: ptr("'Apóstrofo"), Fecha: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Latitude: 32.5149, Longitude: -117.0382},
	}

	for nombre, escribir := range map[string]func([]FilaExportada) ([]byte, error){"csv": EscribirCSV, "xlsx": EscribirXLSX} {
		t.Run(nombre, func(t *testing.T) {
			contenido, err := escribir(exportadas)
			if err != nil {
				t.Fatal(err)
			}
			filas, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
			if err != nil {
				t.Fatalf("LeerDataset: %v", err)
			}
			if !reflect.DeepEqual(filas, want) {
				t.Errorf("filas =\n%+v\nquería\n%+v", filas, want)
			}
		})
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"Mes/año del Incidente":        "mes ano del incidente",
		"  Personas afectadas (total)": "personas afectadas total",
		"ÁRBOL":                        "arbol",
		"Socavón":                      "socavon",
	}
	for in, want := range casos {
		if got := normalizar(in); got != want {
			t.Errorf("normalizar(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestNombreDataset(t *testing.T) {
	if got := NombreDataset("  Lluvias 2023 ", "a.xlsx"); got != "Lluvias 2023" {
		t.Errorf("con nombre: %q", got)
	}
	if got := NombreDataset("", "a.xlsx"); got != "a.xlsx" {
		t.Errorf("sin nombre: %q", got)
	}
	if got := NombreDataset("", ""); got != "Dataset sin nombre" {
		t.Errorf("sin nada: %q", got)
	}
	if got := NombreDataset(strings.Repeat("ñ", 250), ""); len([]rune(got)) != 200 {
		t.Errorf("largo: %d runas", len([]rune(got)))
	}
}

func TestEscribirCSV_TextoQueSeriaFormula(t *testing.T) {
	contenido, err := EscribirCSV([]FilaExportada{{Colonia: "=1+1", Categoria: "@SUMA", Dataset: "'-x", Latitude: 32.5, Longitude: -117.05,
		Fecha: time.Date(2023, 1, 1, 20, 0, 0, 0, time.UTC), Origen: "dataset"}})
	if err != nil {
		t.Fatal(err)
	}
	// Las coordenadas negativas no se tocan: son números, no texto.
	if got := strings.Split(string(contenido), "\n")[1]; got != "'=1+1,32.5,-117.05,2023-01-01,'@SUMA,,,dataset,''-x" {
		t.Errorf("fila = %q", got)
	}
	for _, c := range []struct{ entra, sale string }{
		{"Centro", "Centro"}, {"'=1+1", "=1+1"}, {"'-Norte", "-Norte"}, {"'Apóstrofo", "'Apóstrofo"}, {"'", "'"},
		{"''-Norte", "'-Norte"}, {"''", "'"},
	} {
		if got := sinProteccion(c.entra); got != c.sale {
			t.Errorf("sinProteccion(%q) = %q, quería %q", c.entra, got, c.sale)
		}
	}
}

func TestLeerDataset_CSVWindows1252(t *testing.T) {
	// 0x93 y 0x94 son “ ” en Windows-1252; en Latin-1 serían caracteres de control.
	contenido := []byte("Colonia,Latitud,Longitud,Mes/a\xf1o del Incidente,Categoria\n" +
		"\x93El Mirador\x94,32.3466,-117.0614,02/1998,Inundaci\xf3n\n")
	filas, err := LeerDataset(contenido, catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil {
		t.Fatalf("LeerDataset: %v", err)
	}
	if len(filas) != 1 || filas[0].Colonia == nil || *filas[0].Colonia != "“El Mirador”" {
		t.Errorf("filas = %+v", filas)
	}
}

func TestLeerDataset_XLSXConFechas1904(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	fecha1904 := true
	if err := f.SetWorkbookProps(&excelize.WorkbookPropsOptions{Date1904: &fecha1904}); err != nil {
		t.Fatal(err)
	}
	_ = f.SetSheetRow("Sheet1", "A1", &[]any{"Latitud", "Longitud", "Mes/año del Incidente", "Categoria"})
	// 43497 en el sistema 1904 es el 2 de febrero de 2023 (en el de 1900 sería 2019).
	_ = f.SetSheetRow("Sheet1", "A2", &[]any{32.37, -117.05, 43497, "Inundación"})
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	filas, err := LeerDataset(buf.Bytes(), catalogoPrueba, zonaPrueba, ahoraPrueba)
	if err != nil {
		t.Fatalf("LeerDataset: %v", err)
	}
	if want := time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC); len(filas) != 1 || !filas[0].Fecha.Equal(want) {
		t.Errorf("filas = %+v, quería la fecha %v", filas, want)
	}
}

func TestLeerDataset_CategoriaAmbigua(t *testing.T) {
	// Dos categorías con la misma etiqueta: por la etiqueta no se sabe cuál es,
	// pero por su nombre (único en la base) sí.
	catalogo := append(slices.Clone(catalogoPrueba), Categoria{8, "inundacion_pluvial", "Inundación"})
	contenido := []byte("Latitud,Longitud,Mes/año del Incidente,Categoria\n" +
		"32.37,-117.05,2023-02,Inundación\n" +
		"32.37,-117.05,2023-02,inundacion_pluvial\n")
	_, err := LeerDataset(contenido, catalogo, zonaPrueba, ahoraPrueba)
	var inv *FilasInvalidasError
	if !errors.As(err, &inv) {
		t.Fatalf("err = %v, quería *FilasInvalidasError", err)
	}
	want := []ErrorFila{{2, ColCategoria, `la categoría "Inundación" coincide con más de una categoría del catálogo`}}
	if !reflect.DeepEqual(inv.Errores, want) {
		t.Errorf("errores = %+v\nquería %+v", inv.Errores, want)
	}
}
