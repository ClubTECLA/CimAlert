package riesgo

import (
	"context"

	"github.com/ClubTECLA/cimalert/backend/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store expone las consultas de sqlc y agrega la lectura del resumen, que hace
// varias consultas sobre la misma foto de la base.
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
