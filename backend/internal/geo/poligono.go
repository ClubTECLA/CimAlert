package geo

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Punto es una coordenada en grados.
type Punto struct {
	Lat, Lon float64
}

// Poligono es la zona donde se permite crear reportes. Los vértices van en
// orden alrededor del borde; el último se une con el primero.
type Poligono []Punto

// PoligonoBC cubre a grandes rasgos Baja California y sus islas. El borde
// norte se subió 0.05° para no rechazar reportes pegados a la frontera, así
// que entra un poco de EE. UU. (San Ysidro, Imperial Beach, Calexico, Yuma).
// El borde este es recto y no sigue el río Colorado: entran San Luis Río
// Colorado y Golfo de Santa Clara (Sonora). El último punto saca el borde
// oeste al Pacífico para incluir Isla Guadalupe.
var PoligonoBC = Poligono{
	{27.9547, -115.5611},
	{27.9896, -112.4533},
	{32.7773, -114.6650},
	{32.5715, -117.3546},
	{28.7000, -118.7000},
}

// Contiene reporta si (lat, lon) cae dentro del polígono (ray casting). Un
// punto justo sobre el borde puede quedar de cualquier lado.
func (p Poligono) Contiene(lat, lon float64) bool {
	dentro := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		a, b := p[i], p[j]
		if (a.Lat > lat) != (b.Lat > lat) &&
			lon < (b.Lon-a.Lon)*(lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			dentro = !dentro
		}
	}
	return dentro
}

// ParsePoligono lee "lat,lon;lat,lon;..." con al menos 3 puntos.
func ParsePoligono(s string) (Poligono, error) {
	var p Poligono
	var exactos [][2]*big.Rat
	for _, par := range strings.Split(s, ";") {
		lat, lon, ok := strings.Cut(par, ",")
		if !ok {
			return nil, fmt.Errorf("punto %q: use lat,lon", par)
		}
		la, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
		if err != nil {
			return nil, fmt.Errorf("punto %q: %w", par, err)
		}
		lo, err := strconv.ParseFloat(strings.TrimSpace(lon), 64)
		if err != nil {
			return nil, fmt.Errorf("punto %q: %w", par, err)
		}
		// Escrito en positivo para que NaN (que ParseFloat acepta) también falle.
		if !(la >= -90 && la <= 90 && lo >= -180 && lo <= 180) {
			return nil, fmt.Errorf("punto %q fuera de rango", par)
		}
		p = append(p, Punto{la, lo})
		exactos = append(exactos, [2]*big.Rat{ratExacto(lat, la), ratExacto(lon, lo)})
	}
	if len(p) < 3 {
		return nil, fmt.Errorf("se necesitan al menos 3 puntos, hay %d", len(p))
	}
	// Con 3 puntos no hay lados no contiguos que revisar en cruce, así que un
	// triángulo plano (colineal o con un punto repetido) pasaría y rechazaría
	// todo reporte. El área se calcula exacta sobre el decimal escrito: en
	// float64 puntos colineales como 0.1;0.2;0.3 dejan un residuo distinto de 0.
	if areaExacta(exactos).Sign() == 0 {
		return nil, fmt.Errorf("el polígono no tiene área: hay puntos repetidos o en línea")
	}
	if i, j, ok := p.cruce(); ok {
		return nil, fmt.Errorf("los lados %d y %d se cruzan: los puntos deben ir en orden alrededor del borde", i+1, j+1)
	}
	return p, nil
}

// ratExacto lee el texto decimal tal cual; si big.Rat no acepta el formato
// (p. ej. hexadecimal de ParseFloat) usa el valor binario de f.
func ratExacto(s string, f float64) *big.Rat {
	if r, ok := new(big.Rat).SetString(strings.TrimSpace(s)); ok {
		return r
	}
	return new(big.Rat).SetFloat64(f)
}

// areaExacta es el doble del área con signo (fórmula del cordón) sin redondeo;
// cada punto es {lat, lon}. Solo importa si es 0.
func areaExacta(pts [][2]*big.Rat) *big.Rat {
	a := new(big.Rat)
	dLon, sLat := new(big.Rat), new(big.Rat)
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		dLon.Sub(pts[j][1], pts[i][1])
		sLat.Add(pts[j][0], pts[i][0])
		a.Add(a, dLon.Mul(dLon, sLat))
	}
	return a
}

// cruce busca dos lados no contiguos que se toquen o se crucen. El lado i
// va del punto i al i+1 (el último cierra con el primero).
func (p Poligono) cruce() (int, int, bool) {
	n := len(p)
	for i := 0; i < n; i++ {
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue // contiguos: comparten el primer punto
			}
			if segmentosSeTocan(p[i], p[(i+1)%n], p[j], p[(j+1)%n]) {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// segmentosSeTocan reporta si los segmentos ab y cd comparten algún punto.
func segmentosSeTocan(a, b, c, d Punto) bool {
	o1, o2 := orientacion(a, b, c), orientacion(a, b, d)
	o3, o4 := orientacion(c, d, a), orientacion(c, d, b)
	if o1 != o2 && o3 != o4 {
		return true
	}
	// Colineales: se tocan si alguno cae dentro del rango del otro.
	return o1 == 0 && enRango(a, b, c) || o2 == 0 && enRango(a, b, d) ||
		o3 == 0 && enRango(c, d, a) || o4 == 0 && enRango(c, d, b)
}

// orientacion es el signo del giro a→b→c: 1, -1, o 0 si son colineales.
func orientacion(a, b, c Punto) int {
	v := (b.Lon-a.Lon)*(c.Lat-a.Lat) - (b.Lat-a.Lat)*(c.Lon-a.Lon)
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// enRango reporta si c, colineal con ab, cae dentro del rectángulo de ab.
func enRango(a, b, c Punto) bool {
	return min(a.Lon, b.Lon) <= c.Lon && c.Lon <= max(a.Lon, b.Lon) &&
		min(a.Lat, b.Lat) <= c.Lat && c.Lat <= max(a.Lat, b.Lat)
}
