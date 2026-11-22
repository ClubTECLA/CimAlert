package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ClubTECLA/cimalert/backend/internal/server"
	"github.com/gin-gonic/gin"
)

// TestDocsApagadasPorDefecto comprueba que el contrato y la documentación solo
// se publican si se pide con WithDocs.
func TestDocsApagadasPorDefecto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		opts []server.RouterOption
		want int
	}{
		{name: "sin opción", want: http.StatusNotFound},
		{name: "WithDocs(false)", opts: []server.RouterOption{server.WithDocs(false)}, want: http.StatusNotFound},
		{name: "WithDocs(true)", opts: []server.RouterOption{server.WithDocs(true)}, want: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := server.NewRouter(server.Handlers{}, "secreto-de-prueba-de-al-menos-32-bytes", nil, tt.opts...)

			for _, ruta := range []string{"/v1/docs", "/v1/openapi.json"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, ruta, nil))
				if w.Code != tt.want {
					t.Errorf("GET %s = %d, quiero %d", ruta, w.Code, tt.want)
				}
			}
		})
	}
}
