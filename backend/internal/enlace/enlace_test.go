package enlace

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func firmadorFijo(ahora time.Time) *Firmador {
	f := New("secreto-de-prueba", time.Hour)
	f.ahora = func() time.Time { return ahora }
	return f
}

func TestURL_FormaYExpiracion(t *testing.T) {
	ahora := time.Unix(1_700_000_000, 0)
	url := firmadorFijo(ahora).URL("reportes/abc/foto.jpg")

	if !strings.HasPrefix(url, "/multimedia/reportes/abc/foto.jpg?md5=") {
		t.Fatalf("URL = %q, prefijo inesperado", url)
	}
	if !strings.HasSuffix(url, "&expires=1700003600") {
		t.Errorf("URL = %q, debe expirar una hora después", url)
	}
}

func TestURL_CambiaConArchivoSecretoYExpiracion(t *testing.T) {
	ahora := time.Unix(1_700_000_000, 0)
	base := firmadorFijo(ahora).URL("a.jpg")

	otroArchivo := firmadorFijo(ahora).URL("b.jpg")
	otroSecreto := New("otro-secreto", time.Hour)
	otroSecreto.ahora = func() time.Time { return ahora }
	otraHora := firmadorFijo(ahora.Add(time.Minute)).URL("a.jpg")

	for nombre, url := range map[string]string{
		"otro archivo": otroArchivo,
		"otro secreto": otroSecreto.URL("a.jpg"),
		"otra hora":    otraHora,
	} {
		if firmaDe(url) == firmaDe(base) {
			t.Errorf("%s: la firma no cambió", nombre)
		}
	}
}

func firmaDe(url string) string {
	_, resto, _ := strings.Cut(url, "md5=")
	firma, _, _ := strings.Cut(resto, "&")
	return firma
}

// La firma debe coincidir con la que calcula openssl, que es como la documenta nginx.
func TestFirma_CoincideConOpenssl(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl no está instalado")
	}
	cadena := "1700003600/multimedia/reportes/abc/foto.jpg secreto-de-prueba"
	out, err := exec.Command("sh", "-c", `printf '%s' "$1" | openssl md5 -binary | openssl base64 | tr +/ -_ | tr -d =`, "sh", cadena).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := firma("1700003600", "/multimedia/reportes/abc/foto.jpg", []byte("secreto-de-prueba")), strings.TrimSpace(string(out)); got != want {
		t.Errorf("firma = %q, openssl = %q", got, want)
	}
}
