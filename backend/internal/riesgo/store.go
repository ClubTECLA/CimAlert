package riesgo

import (
	"context"

	"github.com/ClubTECLA/cimalert/backend/internal/db"
	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/ClubTECLA/cimalert/backend/internal/historial"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store expone las consultas de sqlc y agrega la lectura del resumen, que hace
// varias consultas sobre la misma foto de la base, y las operaciones de
// datasets, que guardan varias filas y el historial en una transacción.
type Store struct {
	*sqlc.Queries
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Queries: sqlc.New(pool), pool: pool}
}

// LeerMismaFoto corre fn en una transacción de solo lectura REPEATABLE READ:
// todas sus consultas ven los mismos datos aunque otra petición pase un
// reporte a histórico entre una y otra (con READ COMMITTED, el default, cada
// consulta vería lo último guardado). Siempre termina con rollback, porque no
// escribe nada.
func (s *Store) LeerMismaFoto(ctx context.Context, fn func(consultas) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return fn(sqlc.New(tx))
}

// ImportarDataset guarda el dataset, todas sus filas (con COPY) y el evento
// del historial; si algo falla no queda nada. Un sha256 repetido rompe el
// UNIQUE (el service lo traduce a ErrDatasetDuplicado).
func (s *Store) ImportarDataset(ctx context.Context, arg ImportarTxParams) (sqlc.DatasetHistorico, error) {
	var d sqlc.DatasetHistorico
	err := db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		var err error
		actor := arg.ActorID
		if d, err = q.CreateDatasetHistorico(ctx, sqlc.CreateDatasetHistoricoParams{
			Nombre:     arg.Nombre,
			Archivo:    arg.Archivo,
			Sha256:     arg.Sha256,
			Filas:      len(arg.Filas),
			CargadoPor: &actor,
		}); err != nil {
			return err
		}

		filas := make([]sqlc.CreateIncidentesHistoricosParams, len(arg.Filas))
		for i, f := range arg.Filas {
			filas[i] = sqlc.CreateIncidentesHistoricosParams{
				DatasetID:         d.ID,
				Fila:              f.Fila,
				IncidenteID:       f.IncidenteID,
				Colonia:           f.Colonia,
				Fecha:             pgtype.Date{Time: f.Fecha, Valid: true},
				Latitude:          f.Latitude,
				Longitude:         f.Longitude,
				PersonasAfectadas: f.PersonasAfectadas,
				Fallecidos:        f.Fallecidos,
			}
		}
		if _, err = q.CreateIncidentesHistoricos(ctx, filas); err != nil {
			return err
		}

		return historial.Registrar(ctx, q, arg.ActorID, "riesgo.importar_dataset", "dataset_historico", d.ID.String(),
			map[string]any{"nombre": d.Nombre, "archivo": d.Archivo, "filas": d.Filas})
	})
	if err != nil {
		return sqlc.DatasetHistorico{}, err
	}
	return d, nil
}

// EliminarDataset borra el dataset (sus filas se van con él) y lo registra en
// el historial. Si no existe devuelve pgx.ErrNoRows.
func (s *Store) EliminarDataset(ctx context.Context, id, actorID uuid.UUID) error {
	return db.ExecTx(ctx, s.pool, func(q *sqlc.Queries) error {
		d, err := q.DeleteDatasetHistorico(ctx, id)
		if err != nil {
			return err
		}
		return historial.Registrar(ctx, q, actorID, "riesgo.eliminar_dataset", "dataset_historico", d.ID.String(),
			map[string]any{"nombre": d.Nombre, "archivo": d.Archivo, "filas": d.Filas})
	})
}
