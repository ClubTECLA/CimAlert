package riesgo

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"
	// La imagen de Alpine no trae la base de zonas horarias: se embebe para
	// que America/Tijuana tenga su horario de verano.
	_ "time/tzdata"

	"github.com/xuri/excelize/v2"
)

// Formato del archivo exportado.
type Formato string

const (
	FormatoCSV  Formato = "csv"
	FormatoXLSX Formato = "xlsx"
)

// Columnas que se agregan al exportar; al importar se ignoran.
const (
	ColOrigen  = "Origen"
	ColDataset = "Dataset"
)

// columnasExportadas: las del formato de importación, en su orden, más el origen.
var columnasExportadas = []string{
	ColColonia, ColLatitud, ColLongitud, ColFecha, ColCategoria, ColPersonas, ColFallecidos, ColOrigen, ColDataset,
}

// zonaTijuana para escribir la fecha del incidente en hora local. Con una
// zona fija de UTC-8, un reporte de las 00:30 en horario de verano saldría con
// el día anterior.
var zonaTijuana = func() *time.Location {
	loc, err := time.LoadLocation("America/Tijuana")
	if err != nil {
		panic(err) // no pasa: time/tzdata va embebido
	}
	return loc
}()

// FilaExportada es un incidente histórico de cualquiera de las dos fuentes.
type FilaExportada struct {
	Colonia           string
	Latitude          float64
	Longitude         float64
	Fecha             time.Time
	Categoria         string
	PersonasAfectadas *int
	Fallecidos        *int
	Origen            string // "reporte" o "dataset"
	Dataset           string // vacío si es un reporte
}

// Archivo exportado.
type Archivo struct {
	Nombre      string
	ContentType string
	Contenido   []byte
}

// EscribirCSV en UTF-8 con BOM, para que Excel respete los acentos al abrirlo.
// La fecha va como AAAA-MM-DD. Los textos pasan por textoSeguro.
func EscribirCSV(filas []FilaExportada) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	if err := w.Write(columnasExportadas); err != nil {
		return nil, err
	}
	for _, f := range filas {
		if err := w.Write([]string{
			textoSeguro(f.Colonia),
			strconv.FormatFloat(f.Latitude, 'f', -1, 64),
			strconv.FormatFloat(f.Longitude, 'f', -1, 64),
			f.Fecha.In(zonaTijuana).Format("2006-01-02"),
			textoSeguro(f.Categoria),
			conteoTexto(f.PersonasAfectadas),
			conteoTexto(f.Fallecidos),
			f.Origen,
			textoSeguro(f.Dataset),
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// EscribirXLSX en una hoja "Formato para importación" con la fecha como fecha
// de Excel (mm/aaaa), igual que el formato de importación. Los números quedan
// con 15 dígitos significativos, el límite de Excel (en una coordenada, menos
// de un micrómetro).
func EscribirXLSX(filas []FilaExportada) ([]byte, error) {
	const hoja = "Formato para importación"
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", hoja); err != nil {
		return nil, err
	}
	sw, err := f.NewStreamWriter(hoja)
	if err != nil {
		return nil, err
	}
	formatoFecha := "mm/yyyy"
	estiloFecha, err := f.NewStyle(&excelize.Style{CustomNumFmt: &formatoFecha})
	if err != nil {
		return nil, err
	}

	encabezado := make([]any, len(columnasExportadas))
	for i, c := range columnasExportadas {
		encabezado[i] = c
	}
	if err := sw.SetRow("A1", encabezado); err != nil {
		return nil, err
	}
	for i, fila := range filas {
		celda, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		fecha := fila.Fecha.In(zonaTijuana)
		if err := sw.SetRow(celda, []any{
			fila.Colonia,
			fila.Latitude,
			fila.Longitude,
			excelize.Cell{StyleID: estiloFecha, Value: time.Date(fecha.Year(), fecha.Month(), fecha.Day(), 0, 0, 0, 0, time.UTC)},
			fila.Categoria,
			conteoCelda(fila.PersonasAfectadas),
			conteoCelda(fila.Fallecidos),
			fila.Origen,
			fila.Dataset,
		}); err != nil {
			return nil, err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// inicioDeFormula son los caracteres con los que una hoja de cálculo toma una
// celda de CSV como fórmula.
const inicioDeFormula = "=+-@\t\r"

// textoSeguro antepone ' a un texto que una hoja de cálculo tomaría como
// fórmula: la colonia sale de la dirección que escribe el ciudadano, y
// "=HYPERLINK(...)" se ejecutaría al abrir el CSV. Al importar, sinProteccion
// lo quita. En el .xlsx no hace falta: los textos se guardan como texto.
func textoSeguro(s string) string {
	if s != "" && strings.ContainsRune(inicioDeFormula, rune(s[0])) {
		return "'" + s
	}
	return s
}

// sinProteccion deshace textoSeguro, para que exportar y volver a importar no
// cambie el texto.
func sinProteccion(s string) string {
	if len(s) >= 2 && s[0] == '\'' && strings.ContainsRune(inicioDeFormula, rune(s[1])) {
		return s[1:]
	}
	return s
}

func conteoTexto(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}

func conteoCelda(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}
