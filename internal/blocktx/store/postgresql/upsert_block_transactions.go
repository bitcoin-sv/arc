package postgresql

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockID uint64, txsWithMerklePaths []store.TxWithMerklePath) (err error) {
	ctx, span := tracing.StartTracing(ctx, "UpsertBlockTransactions", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(txsWithMerklePaths)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	copyRowsTxHashes := make([][]any, len(txsWithMerklePaths))
	copyRowsBlockTxMap := make([][]any, len(txsWithMerklePaths))

	for pos, tx := range txsWithMerklePaths {
		copyRowsTxHashes[pos] = []any{tx.Hash}
		copyRowsBlockTxMap[pos] = []any{blockID, tx.Hash, tx.MerklePath, tx.MerkleTreeIndex}
	}

	err = p.conn.Raw(func(driverConn any) error {
		conn := driverConn.(*stdlib.Conn).Conn() // conn is a *pgx.Conn
		var pqErr *pgconn.PgError

		// Do pgx specific stuff with conn
		_, err := conn.CopyFrom(
			ctx,
			pgx.Identifier{"blocktx", "transactions"},
			[]string{"hash"},
			pgx.CopyFromRows(copyRowsTxHashes),
		)
		// Error 23505 is: "duplicate key violates unique constraint"
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// ON CONFLICT DO NOTHING
			err = nil
		}
		if err != nil {
			return err
		}

		_, err = conn.CopyFrom(
			ctx,
			pgx.Identifier{"blocktx", "block_transactions_map"},
			[]string{"blockid", "txhash", "merkle_path", "merkle_tree_index"},
			pgx.CopyFromRows(copyRowsBlockTxMap),
		)
		// Error 23505 is: "duplicate key violates unique constraint"
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// ON CONFLICT DO NOTHING
			err = nil
		}

		return err
	})
	if err != nil {
		return errors.Join(store.ErrFailedToUpsertBlockTransactionsMap, err)
	}

	return nil
}
