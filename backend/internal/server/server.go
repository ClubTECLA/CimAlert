package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/ClubTECLA/cimalert/backend/internal/almacen"
	"github.com/ClubTECLA/cimalert/backend/internal/area"
	"github.com/ClubTECLA/cimalert/backend/internal/centro"
	"github.com/ClubTECLA/cimalert/backend/internal/comentario"
	"github.com/ClubTECLA/cimalert/backend/internal/config"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/googleid"
	"github.com/ClubTECLA/cimalert/backend/internal/incidente"
	"github.com/ClubTECLA/cimalert/backend/internal/multimedia"
	"github.com/ClubTECLA/cimalert/backend/internal/reporte"
	"github.com/ClubTECLA/cimalert/backend/internal/riesgo"
	"github.com/ClubTECLA/cimalert/backend/internal/usuario"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// multimediaTimeout es lo que se espera a que multimedia reciba y guarde un archivo.
const multimediaTimeout = 30 * time.Second

// googleTimeout es lo que se espera a que Google entregue sus claves.
const googleTimeout = 5 * time.Second

// New arma los servicios de producción sobre pool y devuelve el router con
// el que se sirven. Es la única pieza que conoce a todas las funcionalidades.
func New(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) (*gin.Engine, error) {
	usuarios := usuario.NewStore(pool)
	mailer, err := usuario.NewMailer(cfg.MailDriver)
	if err != nil {
		return nil, err
	}

	var authOpts []usuario.AuthOption
	if len(cfg.GoogleClientIDs) > 0 {
		v := googleid.New(cfg.GoogleClientIDs, &http.Client{Timeout: googleTimeout}, googleid.CertsURL)
		authOpts = append(authOpts, usuario.WithGoogle(v, mailer))
		slog.InfoContext(ctx, "google sign-in enabled", "google_client_ids", len(cfg.GoogleClientIDs))
	} else {
		slog.WarnContext(ctx, "google sign-in disabled: GOOGLE_CLIENT_IDS empty", "google_client_ids", 0)
	}

	auth, err := usuario.NewAuth(ctx, usuarios, []byte(cfg.JWTSecret), cfg.JWTTTL, authOpts...)
	if err != nil {
		return nil, err
	}
	rec := usuario.NewRecuperacion(usuarios, mailer, []byte(cfg.RecoverySecret))

	// Un solo cliente de multimedia: multimedia sube y borra archivos, y
	// reporte borra los de un reporte eliminado.
	archivos := almacen.NewCliente(cfg.MultimediaURL, multimediaTimeout)

	reportes := reporte.NewService(reporte.NewStore(pool), archivos, cfg.ReportePoligono)
	centros := centro.NewService(centro.NewStore(pool))

	return NewRouter(Handlers{
		Reportes:    reporte.NewHandler(reportes),
		Incidentes:  incidente.NewHandler(incidente.NewService(incidente.NewStore(pool))),
		Comentarios: comentario.NewHandler(comentario.NewService(comentario.NewStore(pool))),
		Usuarios:    usuario.NewHandler(auth, usuario.NewService(usuarios), rec),
		// multimedia no tiene operaciones de varias sentencias: le basta con
		// las consultas que genera sqlc.
		Multimedia: multimedia.NewHandler(multimedia.NewService(sqlc.New(pool), archivos, multimedia.OpcionesFoto{
			MaxBytes:   cfg.MaxFotoBytes,
			MaxPixeles: cfg.MaxFotoPixeles,
		}, multimedia.OpcionesVideo{
			MaxBytes:    cfg.MaxVideoBytes,
			MaxDuracion: time.Duration(cfg.MaxVideoSegundos) * time.Second,
		})),
		Centros: centro.NewHandler(centros),
		Areas:   area.NewHandler(area.NewService(reportes, centros)),
		Riesgo:  riesgo.NewHandler(riesgo.NewService(riesgo.NewStore(pool))),
	}, cfg.JWTSecret, usuarios.GetPasswordChangedAtByUserID, WithMaxMultimediaBytes(cfg.MaxMultimediaBytes), WithDocs(cfg.DocsEnabled)), nil
}
