package riesgo

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// Límites de la importación. El body de la petición ya llega limitado a 1 MiB
// (middleware.BodyLimit); un .xlsx es un zip, así que además se limita lo que
// puede ocupar descomprimido.
const (
	MaxFilasDataset      = 50000
	MaxErroresDevueltos  = 100
	maxColoniaRunas      = 200
	maxNombreDatasetRuna = 200
	maxPersonas          = 1000000
	xlsxMaxDescomprimido = 64 << 20
	xlsxMaxXML           = 32 << 20
	// xlsxMaxColumnas por renglón (ver leerHoja). El formato usa 7 columnas
	// y los archivos reales llegan a 14.
	xlsxMaxColumnas = 100
)

// Columnas del formato de importación, con el nombre que se muestra en los
// errores (el de la hoja "Formato para importación").
const (
	ColColonia    = "Colonia"
	ColLatitud    = "Latitud"
	ColLongitud   = "Longitud"
	ColFecha      = "Mes/año del Incidente"
	ColCategoria  = "Categoria"
	ColPersonas   = "Personas afectadas (total)"
	ColFallecidos = "Fallecidos"
)

// columnasObligatorias, en el orden en que se reportan si faltan.
var columnasObligatorias = []string{ColLatitud, ColLongitud, ColFecha, ColCategoria}

// alias de cada columna, ya normalizados (ver normalizar).
var aliasColumnas = map[string]string{
	"colonia":                  ColColonia,
	"latitud":                  ColLatitud,
	"latitude":                 ColLatitud,
	"lat":                      ColLatitud,
	"longitud":                 ColLongitud,
	"longitude":                ColLongitud,
	"lng":                      ColLongitud,
	"lon":                      ColLongitud,
	"mes ano del incidente":    ColFecha,
	"mes ano":                  ColFecha,
	"fecha":                    ColFecha,
	"fecha del incidente":      ColFecha,
	"categoria":                ColCategoria,
	"personas afectadas total": ColPersonas,
	"personas afectadas":       ColPersonas,
	"fallecidos":               ColFallecidos,
}

// aliasCategorias: nombres de categoría que no están en el catálogo pero
// equivalen a una que sí (por su nombre en el catálogo).
var aliasCategorias = map[string]string{
	"derrumbe": "deslave",
}

// Categoria del catálogo, para reconocer la de cada fila.
type Categoria struct {
	ID       int
	Nombre   string
	Etiqueta string
}

// FilaDataset es un incidente válido, listo para guardar.
type FilaDataset struct {
	Fila              int
	IncidenteID       int
	Colonia           *string
	Fecha             time.Time // día 1 del mes, en UTC
	Latitude          float64
	Longitude         float64
	PersonasAfectadas *int
	Fallecidos        *int
}

// ErrorFila es un problema de una fila del archivo. Columna vacía = toda la fila.
type ErrorFila struct {
	Fila    int
	Columna string
	Mensaje string
}

// FilasInvalidasError dice que el archivo tiene filas inválidas: no se
// importa nada. Errores trae los primeros MaxErroresDevueltos; Total, todos.
type FilasInvalidasError struct {
	Errores []ErrorFila
	Total   int
}

func (e *FilasInvalidasError) Error() string {
	return fmt.Sprintf("el archivo tiene %d errores; no se importó nada", e.Total)
}

// formatoInvalidoError es un problema con el archivo entero (no es .xlsx ni
// .csv, o le faltan encabezados). Es ErrFormatoInvalido con un mensaje propio.
type formatoInvalidoError struct{ msg string }

func (e *formatoInvalidoError) Error() string   { return e.msg }
func (e *formatoInvalidoError) Is(t error) bool { return t == ErrFormatoInvalido }

func formatoInvalido(format string, args ...any) error {
	return &formatoInvalidoError{msg: fmt.Sprintf(format, args...)}
}

// filaCruda es un renglón del archivo como texto, con su número (1 = primero).
type filaCruda struct {
	num    int
	celdas []string
}

// tabla es el contenido del archivo: los encabezados y los renglones que siguen.
type tabla struct {
	columnas map[string]int // columna del formato → índice
	filas    []filaCruda
	// En un .xlsx una fecha llega como número de serie de Excel, contado
	// desde 1900 o, en libros con el sistema de fechas 1904 (Excel de Mac
	// antiguo), desde 1904.
	esXLSX    bool
	fecha1904 bool
}

// Zona es el área donde se aceptan las filas de un dataset. La cumple
// reporte.Poligono: server le pasa la misma zona que a los reportes, así que
// el riesgo no recibe puntos que un reporte no podría tener.
type Zona interface {
	Contiene(lat, lon float64) bool
}

// LeerDataset lee un .xlsx o un .csv con el formato de importación y valida
// cada fila contra el catálogo y la zona. ahora acota las fechas (no se
// aceptan meses futuros). Si alguna fila es inválida devuelve
// *FilasInvalidasError con todas las que fallaron.
func LeerDataset(contenido []byte, catalogo []Categoria, zona Zona, ahora time.Time) ([]FilaDataset, error) {
	t, err := leerTabla(contenido)
	if err != nil {
		return nil, err
	}

	categorias := indiceCategorias(catalogo)
	limite := time.Date(ahora.Year(), ahora.Month(), 1, 0, 0, 0, 0, time.UTC)
	var (
		validas []FilaDataset
		errs    []ErrorFila
		total   int
		datos   int
	)
	agregar := func(e ErrorFila) {
		total++
		if len(errs) < MaxErroresDevueltos {
			errs = append(errs, e)
		}
	}

	for _, fc := range t.filas {
		celda := func(col string) string {
			i, ok := t.columnas[col]
			if !ok || i >= len(fc.celdas) {
				return ""
			}
			return strings.TrimSpace(fc.celdas[i])
		}
		// Un texto del CSV puede traer el ' de textoSeguro; el .xlsx no lo lleva.
		texto := func(col string) string {
			if t.esXLSX {
				return celda(col)
			}
			return sinProteccion(celda(col))
		}
		if filaVacia(t, fc) {
			continue
		}
		if datos++; datos > MaxFilasDataset {
			return nil, formatoInvalido("el archivo tiene más de %d filas", MaxFilasDataset)
		}

		f := FilaDataset{Fila: fc.num}
		antes := total

		if v := texto(ColColonia); v != "" {
			if utf8.RuneCountInString(v) > maxColoniaRunas {
				agregar(ErrorFila{fc.num, ColColonia, fmt.Sprintf("la colonia tiene más de %d caracteres", maxColoniaRunas)})
			} else {
				f.Colonia = &v
			}
		}

		latOK, lngOK := false, false
		if f.Latitude, latOK = leerCoordenada(celda(ColLatitud), 90); !latOK {
			agregar(ErrorFila{fc.num, ColLatitud, "la latitud debe ser un número entre -90 y 90"})
		}
		if f.Longitude, lngOK = leerCoordenada(celda(ColLongitud), 180); !lngOK {
			agregar(ErrorFila{fc.num, ColLongitud, "la longitud debe ser un número entre -180 y 180"})
		}
		// Columna vacía: el problema es la pareja, no una de las dos. Suele
		// ser la longitud sin el signo menos (117 en vez de -117) o un punto
		// de otro estado.
		if latOK && lngOK && !zona.Contiene(f.Latitude, f.Longitude) {
			agregar(ErrorFila{fc.num, "", "la ubicación está fuera de la zona permitida (Baja California)"})
		}

		var ok bool

		switch fecha, err := leerFecha(celda(ColFecha), t.esXLSX, t.fecha1904); {
		case err != nil:
			agregar(ErrorFila{fc.num, ColFecha, err.Error()})
		case fecha.After(limite):
			agregar(ErrorFila{fc.num, ColFecha, "la fecha es posterior al mes actual"})
		default:
			f.Fecha = fecha
		}

		switch v := texto(ColCategoria); {
		case v == "":
			agregar(ErrorFila{fc.num, ColCategoria, "falta la categoría"})
		default:
			id, encontrada := categorias[normalizar(v)]
			switch {
			case !encontrada:
				agregar(ErrorFila{fc.num, ColCategoria, fmt.Sprintf("la categoría %q no existe en el catálogo", v)})
			case id == categoriaAmbigua:
				agregar(ErrorFila{fc.num, ColCategoria, fmt.Sprintf("la categoría %q coincide con más de una categoría del catálogo", v)})
			default:
				f.IncidenteID = id
			}
		}

		if f.PersonasAfectadas, ok = leerConteo(celda(ColPersonas)); !ok {
			agregar(ErrorFila{fc.num, ColPersonas, "las personas afectadas deben ser un número entero de 0 o más"})
		}
		if f.Fallecidos, ok = leerConteo(celda(ColFallecidos)); !ok {
			agregar(ErrorFila{fc.num, ColFallecidos, "los fallecidos deben ser un número entero de 0 o más"})
		}

		if total == antes {
			validas = append(validas, f)
		}
	}

	if total > 0 {
		return nil, &FilasInvalidasError{Errores: errs, Total: total}
	}
	if len(validas) == 0 {
		return nil, formatoInvalido("el archivo no tiene filas de datos")
	}
	return validas, nil
}

// NombreDataset es el nombre pedido o, si viene vacío, el del archivo.
func NombreDataset(pedido, archivo string) string {
	n := strings.TrimSpace(pedido)
	if n == "" {
		n = strings.TrimSpace(archivo)
	}
	if n == "" {
		n = "Dataset sin nombre"
	}
	if r := []rune(n); len(r) > maxNombreDatasetRuna {
		n = string(r[:maxNombreDatasetRuna])
	}
	return n
}

// filaVacia: un renglón sin nada en las columnas del formato se salta (las
// hojas suelen traer renglones vacíos al final, o fórmulas en otras columnas).
func filaVacia(t tabla, fc filaCruda) bool {
	for _, i := range t.columnas {
		if i < len(fc.celdas) && strings.TrimSpace(fc.celdas[i]) != "" {
			return false
		}
	}
	return true
}

// categoriaAmbigua marca en indiceCategorias un texto que normalizado
// coincide con dos categorías distintas (los ids de la base empiezan en 1).
const categoriaAmbigua = -1

// indiceCategorias reconoce cada categoría por su nombre y su etiqueta
// normalizados, más los alias. La etiqueta no es única en la base: si un texto
// lleva a dos categorías queda como categoriaAmbigua y esa fila se rechaza, en
// vez de quedarse con una de las dos sin avisar.
func indiceCategorias(catalogo []Categoria) map[string]int {
	idx := make(map[string]int, len(catalogo)*2+len(aliasCategorias))
	agregar := func(clave string, id int) {
		if previo, existe := idx[clave]; existe && previo != id {
			idx[clave] = categoriaAmbigua
			return
		}
		idx[clave] = id
	}
	for _, c := range catalogo {
		agregar(normalizar(c.Nombre), c.ID)
		agregar(normalizar(c.Etiqueta), c.ID)
	}
	for alias, nombre := range aliasCategorias {
		if id, ok := idx[normalizar(nombre)]; ok && id != categoriaAmbigua {
			if _, existe := idx[alias]; !existe {
				idx[alias] = id
			}
		}
	}
	return idx
}

// normalizar pasa a minúsculas, quita acentos y deja letras y números
// separados por un espacio: "Mes/año del Incidente" → "mes ano del incidente".
func normalizar(s string) string {
	var b strings.Builder
	espacio := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r): // marca de acento, ya separada por NFD
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if espacio && b.Len() > 0 {
				b.WriteByte(' ')
			}
			espacio = false
			b.WriteRune(r)
		default:
			espacio = true
		}
	}
	return b.String()
}

// leerNumero acepta punto o coma decimal ("32.5" o "32,5").
func leerNumero(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	if !strings.Contains(s, ".") {
		s = strings.Replace(s, ",", ".", 1)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func leerCoordenada(s string, limite float64) (float64, bool) {
	v, ok := leerNumero(s)
	return v, ok && v >= -limite && v <= limite
}

// leerConteo: vacío = sin dato (nil); si no, un entero de 0 a maxPersonas
// ("3.0" vale, "1.5" no).
func leerConteo(s string) (*int, bool) {
	if s == "" {
		return nil, true
	}
	v, ok := leerNumero(s)
	if !ok || v < 0 || v > maxPersonas || v != math.Trunc(v) {
		return nil, false
	}
	n := int(v)
	return &n, true
}

// formatosFecha que se aceptan como texto; solo se guarda el mes.
var formatosFecha = []string{
	"2006-01", "2006-01-02", "2006/01/02", "2006-01-02 15:04:05", time.RFC3339,
	"01/2006", "1/2006", "01-2006", "1-2006",
	"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006",
}

// leerFecha devuelve el día 1 del mes de la fecha. En un .xlsx una celda con
// fecha llega como su número de serie de Excel.
func leerFecha(s string, esXLSX, fecha1904 bool) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("falta la fecha")
	}
	if esXLSX {
		if serie, err := strconv.ParseFloat(s, 64); err == nil {
			t, err := excelize.ExcelDateToTime(serie, fecha1904)
			if err != nil || t.Year() < 1900 {
				return time.Time{}, errors.New("la fecha no es válida")
			}
			return primerDia(t), nil
		}
	}
	for _, f := range formatosFecha {
		if t, err := time.Parse(f, s); err == nil {
			if t.Year() < 1900 {
				return time.Time{}, errors.New("la fecha es anterior a 1900")
			}
			return primerDia(t), nil
		}
	}
	return time.Time{}, fmt.Errorf("la fecha %q no tiene un formato válido (AAAA-MM, MM/AAAA o DD/MM/AAAA)", s)
}

func primerDia(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// leerTabla distingue el formato por el contenido: un .xlsx es un zip.
func leerTabla(contenido []byte) (tabla, error) {
	if bytes.HasPrefix(contenido, []byte("PK\x03\x04")) {
		return leerXLSX(contenido)
	}
	return leerCSV(contenido)
}

func leerXLSX(contenido []byte) (tabla, error) {
	f, err := excelize.OpenReader(bytes.NewReader(contenido), excelize.Options{
		UnzipSizeLimit:    xlsxMaxDescomprimido,
		UnzipXMLSizeLimit: xlsxMaxXML,
	})
	if err != nil {
		return tabla{}, formatoInvalido("el archivo no es un .xlsx válido")
	}
	defer f.Close()

	var faltanPrimera []string
	for i, hoja := range f.GetSheetList() {
		renglones, err := leerHoja(f, hoja)
		if err != nil {
			return tabla{}, err
		}
		t, faltan := armarTabla(renglones, 1)
		if faltan == nil {
			t.esXLSX = true
			if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
				t.fecha1904 = *props.Date1904
			}
			return t, nil
		}
		if i == 0 {
			faltanPrimera = faltan
		}
	}
	return tabla{}, formatoInvalido("ninguna hoja tiene los encabezados del formato; en la primera faltan: %s", strings.Join(faltanPrimera, ", "))
}

// leerHoja lee una hoja renglón por renglón (lo mismo que GetRows, pero sin
// juntar la hoja entera antes de revisarla). excelize rellena cada renglón con
// celdas vacías hasta su última celda con dato: con un valor en la columna XFD
// (la 16,384), un .xlsx de 12 KB con 1,000 renglones pedía más de 1 GB. Por eso:
//   - un renglón con más de xlsxMaxColumnas rechaza el archivo en cuanto se lee;
//   - de los renglones de datos solo se guarda hasta la última columna del formato;
//   - el límite de filas se aplica mientras se lee;
//   - si el encabezado (el primer renglón no vacío) no es del formato, la hoja
//     no se sigue leyendo: armarTabla dice qué le falta.
func leerHoja(f *excelize.File, hoja string) ([][]string, error) {
	rows, err := f.Rows(hoja)
	if err != nil {
		return nil, formatoInvalido("no se pudo leer la hoja %q", hoja)
	}
	defer rows.Close()

	var (
		renglones  [][]string
		encabezado tabla // solo sus columnas, para filaVacia
		hasta      = -1  // columnas que se guardan de cada renglón; -1 = aún sin encabezado
		datos      int
	)
	for rows.Next() {
		// RawCellValue: los números sin el formato de la celda (las fechas como
		// número de serie y las coordenadas con todos sus decimales).
		r, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, formatoInvalido("no se pudo leer la hoja %q", hoja)
		}
		if len(r) > xlsxMaxColumnas {
			return nil, formatoInvalido("la hoja %q tiene un renglón con más de %d columnas", hoja, xlsxMaxColumnas)
		}
		if hasta < 0 {
			renglones = append(renglones, r)
			if strings.TrimSpace(strings.Join(r, "")) == "" {
				continue
			}
			columnas, faltan := columnasDelEncabezado(r)
			if faltan != nil {
				break
			}
			encabezado.columnas = columnas
			for _, i := range columnas {
				hasta = max(hasta, i+1)
			}
			continue
		}
		r = slices.Clone(r[:min(len(r), hasta)])
		// Mismo límite y misma regla que LeerDataset: solo cuenta un renglón
		// con algo en las columnas del formato (no en una columna de más, como
		// unas notas entre ellas). Aquí se corta antes de guardarlos.
		if !filaVacia(encabezado, filaCruda{celdas: r}) {
			if datos++; datos > MaxFilasDataset {
				return nil, formatoInvalido("el archivo tiene más de %d filas", MaxFilasDataset)
			}
		}
		renglones = append(renglones, r)
	}
	if err := rows.Error(); err != nil {
		return nil, formatoInvalido("no se pudo leer la hoja %q", hoja)
	}
	return renglones, nil
}

func leerCSV(contenido []byte) (tabla, error) {
	contenido = bytes.TrimPrefix(contenido, []byte("\xEF\xBB\xBF")) // BOM de UTF-8
	if !utf8.Valid(contenido) {
		// Excel en Windows guarda el CSV en Windows-1252 por defecto. Coincide
		// con Latin-1 en las letras del español, pero no en 0x80–0x9F (comillas
		// tipográficas “ ”, guiones largos, €).
		var err error
		if contenido, err = charmap.Windows1252.NewDecoder().Bytes(contenido); err != nil {
			return tabla{}, formatoInvalido("el archivo no es un .xlsx ni un .csv")
		}
	}
	if bytes.IndexByte(contenido, 0) >= 0 {
		return tabla{}, formatoInvalido("el archivo no es un .xlsx ni un .csv")
	}

	r := csv.NewReader(bytes.NewReader(contenido))
	r.Comma = separadorCSV(contenido)
	r.FieldsPerRecord = -1 // las filas pueden traer más o menos columnas
	var renglones [][]string
	var numeros []int
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return tabla{}, formatoInvalido("el CSV no es válido: %v", err)
		}
		linea, _ := r.FieldPos(0)
		renglones = append(renglones, rec)
		numeros = append(numeros, linea)
	}
	t, faltan := armarTabla(renglones, 1)
	if faltan != nil {
		return tabla{}, formatoInvalido("faltan columnas del formato: %s", strings.Join(faltan, ", "))
	}
	// En un CSV un campo entre comillas puede ocupar varias líneas: se usa la
	// línea real de cada renglón.
	for i := range t.filas {
		t.filas[i].num = numeros[t.filas[i].num-1]
	}
	return t, nil
}

// armarTabla toma como encabezado el primer renglón no vacío. Si le faltan
// columnas obligatorias devuelve cuáles. primero es el número del primer renglón.
func armarTabla(renglones [][]string, primero int) (tabla, []string) {
	inicio := -1
	for i, r := range renglones {
		if strings.TrimSpace(strings.Join(r, "")) != "" {
			inicio = i
			break
		}
	}
	if inicio < 0 {
		return tabla{}, columnasObligatorias
	}

	columnas, faltan := columnasDelEncabezado(renglones[inicio])
	if faltan != nil {
		return tabla{}, faltan
	}

	t := tabla{columnas: columnas}
	for i := inicio + 1; i < len(renglones); i++ {
		t.filas = append(t.filas, filaCruda{num: primero + i, celdas: renglones[i]})
	}
	return t, nil
}

// columnasDelEncabezado reconoce las columnas del formato en un encabezado
// (columna del formato → índice; si una se repite, cuenta la primera) y
// devuelve las obligatorias que faltan.
func columnasDelEncabezado(encabezado []string) (map[string]int, []string) {
	columnas := map[string]int{}
	for i, enc := range encabezado {
		col, ok := aliasColumnas[normalizar(enc)]
		if !ok {
			continue // columna de más
		}
		if _, repetida := columnas[col]; !repetida {
			columnas[col] = i
		}
	}
	var faltan []string
	for _, col := range columnasObligatorias {
		if _, ok := columnas[col]; !ok {
			faltan = append(faltan, col)
		}
	}
	return columnas, faltan
}

// separadorCSV elige entre coma, punto y coma y tabulador según cuál aparece
// más en la primera línea (Excel en español guarda con punto y coma).
func separadorCSV(contenido []byte) rune {
	linea, _, _ := bytes.Cut(contenido, []byte("\n"))
	mejor, cuantos := ',', bytes.Count(linea, []byte(","))
	for _, sep := range []rune{';', '\t'} {
		if n := bytes.Count(linea, []byte(string(sep))); n > cuantos {
			mejor, cuantos = sep, n
		}
	}
	return mejor
}
