package reporte

import "testing"

func TestPoligonoBC_Contiene(t *testing.T) {
	tests := []struct {
		nombre   string
		lat, lon float64
		want     bool
	}{
		{"Tijuana", 32.5149, -117.0382, true},
		{"Rosarito", 32.3661, -117.0618, true},
		{"Tecate", 32.5768, -116.6250, true},
		{"Ensenada", 31.8667, -116.5964, true},
		{"Mexicali", 32.6245, -115.4523, true},
		{"San Felipe", 31.0275, -114.8353, true},
		{"San Quintín", 30.5600, -115.9378, true},
		{"Isla Cedros", 28.1800, -115.2200, true},
		{"Isla Ángel de la Guarda", 29.3333, -113.4167, true},
		{"Bahía de los Ángeles", 28.9500, -113.5600, true},
		// Entran a propósito (ver PoligonoBC).
		{"Calexico (margen norte)", 32.6789, -115.4989, true},
		{"San Luis Río Colorado", 32.4560, -114.7720, true},
		{"Golfo de Santa Clara", 31.6840, -114.4980, true},
		{"Isla Guadalupe (centro)", 29.0300, -118.2800, true},
		{"Isla Guadalupe (punta norte)", 29.1900, -118.3500, true},
		{"Isla Guadalupe (punta sur)", 28.8800, -118.2900, true},

		{"San Diego centro", 32.7157, -117.1611, false},
		{"El Centro", 32.7920, -115.5631, false},
		{"Caborca", 30.7161, -112.1580, false},
		{"Hermosillo", 29.0729, -110.9559, false},
		{"Puerto Peñasco", 31.3172, -113.5370, false},
		{"La Paz", 24.1426, -110.3128, false},
		{"Santa Rosalía", 27.3389, -112.2669, false},
		{"Pacífico al oeste de Isla Guadalupe", 29.0000, -119.2000, false},
		{"Los Ángeles", 34.0522, -118.2437, false},
		{"Ciudad de México", 19.4326, -99.1332, false},
	}
	for _, tt := range tests {
		if got := PoligonoBC.Contiene(tt.lat, tt.lon); got != tt.want {
			t.Errorf("%s (%v, %v): Contiene() = %v, want %v", tt.nombre, tt.lat, tt.lon, got, tt.want)
		}
	}
}

func TestParsePoligono(t *testing.T) {
	p, err := ParsePoligono("27.9547,-115.5611; 27.9896,-112.4533;32.7773, -114.6650;32.5715,-117.3546;28.7,-118.7")
	if err != nil {
		t.Fatalf("ParsePoligono() error = %v", err)
	}
	if len(p) != len(PoligonoBC) {
		t.Fatalf("ParsePoligono() = %v, want %v", p, PoligonoBC)
	}
	for i := range p {
		if p[i] != PoligonoBC[i] {
			t.Fatalf("ParsePoligono() = %v, want %v", p, PoligonoBC)
		}
	}

	for _, raw := range []string{
		"1,1;2,2",        // menos de 3 puntos
		"1,1;2,2;3",      // sin coma
		"1,1;2,2;a,3",    // no numérico
		"1,1;2,2;91,3",   // lat fuera de rango
		"1,1;2,2;3,-181", // lon fuera de rango
		"1,1;2,2;NaN,3",  // NaN
		"1,1;2,2;3,3;",   // ; al final
		// Los puntos de BC en el orden original: SW, SE, NW, NE (moño).
		"27.9547,-115.5611;27.9896,-112.4533;32.5215,-117.3546;32.7273,-114.6650",
		"0,0;0,2;2,2;0,1;2,0", // un vértice cae sobre un lado no contiguo
		"0,0;1,1;2,2",         // triángulo colineal: sin área
		"0,0;0,0;1,1",         // punto repetido: sin área
		// Colineales en decimal, pero en float64 el cordón deja un residuo.
		"0.1,0.1;0.2,0.2;0.3,0.3",
		"32.1,-117.1;32.2,-117.2;32.3,-117.3",
	} {
		if _, err := ParsePoligono(raw); err == nil {
			t.Errorf("ParsePoligono(%q) error = nil, want error", raw)
		}
	}
}

func TestParsePoligono_Concavo(t *testing.T) {
	// Una "L": cóncava pero sin cruces, debe aceptarse.
	if _, err := ParsePoligono("0,0;0,2;1,2;1,1;2,1;2,0"); err != nil {
		t.Fatalf("ParsePoligono() error = %v, want nil", err)
	}
}
