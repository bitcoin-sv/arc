package postgresql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
)

// GetTransactionMerklePath returns the merkle path of a transaction.
func (s *PostgreSQL) GetTransactionMerklePath(ctx context.Context, txhash *chainhash.Hash) (string, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionMerklePath").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 t.merkle_path
		FROM transactions t
		WHERE t.hash = $1
	`

	var merklePath sql.NullString

	if err := s.db.QueryRowContext(ctx, q, txhash[:]).Scan(&merklePath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", store.ErrNotFound
		}
		return "", err
	}

	return merklePath.String, nil
}
