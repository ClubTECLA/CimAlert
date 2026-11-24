// Package enlace firma las URLs de los archivos de multimedia para que expiren.
package enlace

import (
	"crypto/md5" //nolint:gosec // el formato lo fija el secure_link_md5 de nginx
	"encoding/base64"
	"strconv"
	"time"
)

// prefijo es donde nginx sirve los archivos.
const prefijo = "/multimedia/"

// Firmador crea URLs de archivos que nginx (secure_link) acepta hasta que vencen.
type Firmador struct {
	secreto []byte
	ttl     time.Duration
	ahora   func() time.Time
}

// New crea un Firmador; ttl es lo que dura cada enlace.
func New(secreto string, ttl time.Duration) *Firmador {
	return &Firmador{secreto: []byte(secreto), ttl: ttl, ahora: time.Now}
}

// URL devuelve la URL firmada del archivo path (relativo a /multimedia/).
func (f *Firmador) URL(path string) string {
	expira := strconv.FormatInt(f.ahora().Add(f.ttl).Unix(), 10)
	return prefijo + path + "?md5=" + firma(expira, prefijo+path, f.secreto) + "&expires=" + expira
}

// firma calcula lo mismo que `secure_link_md5 "$secure_link_expires$uri secreto"`:
// MD5 binario en base64 seguro para URLs y sin relleno.
func firma(expira, uri string, secreto []byte) string {
	suma := md5.Sum([]byte(expira + uri + " " + string(secreto))) //nolint:gosec
	return base64.RawURLEncoding.EncodeToString(suma[:])
}
