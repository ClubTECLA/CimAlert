package riesgo

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"time"

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

// zonaTijuana para escribir la fecha del incidente en hora local (el día 1 del
// mes de un dataset se guarda a mediodía en Tijuana). La imagen de Alpine no
// trae la base de zonas horarias: sin ella se usa UTC-8, que solo mueve la hora
// y no cambia el día de un mediodía.
var zonaTijuana = func() *time.Location {
	if loc, err := time.LoadLocation("America/Tijuana"); err == nil {
		return loc
	}
	return time.FixedZone("PST", -8*60*60)
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
// La fecha va como AAAA-MM-DD.
func EscribirCSV(filas []FilaExportada) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	if err := w.Write(columnasExportadas); err != nil {
		return nil, err
	}
	for _, f := range filas {
		if err := w.Write([]string{
			f.Colonia,
			strconv.FormatFloat(f.Latitude, 'f', -1, 64),
			strconv.FormatFloat(f.Longitude, 'f', -1, 64),
			f.Fecha.In(zonaTijuana).Format("2006-01-02"),
			f.Categoria,
			conteoTexto(f.PersonasAfectadas),
			conteoTexto(f.Fallecidos),
			f.Origen,
			f.Dataset,
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
