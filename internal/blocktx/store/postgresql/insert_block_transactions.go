package postgresql

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

// InsertBlockTransactions inserts the transaction hashes for a given block hash
func (p *PostgreSQL) InsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxHashWithMerkleTreeIndex) (err error) {
	ctx, span := tracing.StartTracing(ctx, "InsertBlockTransactions", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(txsWithMerklePaths)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	copyRows := make([][]any, len(txsWithMerklePaths))

	for pos, tx := range txsWithMerklePaths {
		copyRows[pos] = []any{blockID, tx.Hash, tx.MerkleTreeIndex}
	}

	err = p.conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return errors.New("driverConn.(*stdlib.Conn) conversion failed")
		}
		conn := c.Conn() // conn is a *pgx.Conn
		var pqErr *pgconn.PgError

		_, err = conn.CopyFrom(
			ctx,
			pgx.Identifier{"blocktx", "block_transactions"},
			[]string{"block_id", "hash", "merkle_tree_index"},
			pgx.CopyFromRows(copyRows),
		)

		// Error 23505 is: "duplicate key violates unique constraint"
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// ON CONFLICT DO NOTHING
			err = nil
		}
		if err != nil {
			return err
		}

		return nil
	})

	return nil
}
