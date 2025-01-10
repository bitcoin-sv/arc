package postgresql

import (
	"context"
	"errors"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) GetBlockTransactionsHashes(ctx context.Context, blockHash []byte) (txHashes []*chainhash.Hash, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetBlockTransactionsHashes", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
		SELECT
			bt.hash
		FROM blocktx.block_transactions AS bt
			JOIN blocktx.blocks AS b ON bt.block_id = b.id
		WHERE b.hash = $1
	`

	rows, err := p.db.QueryContext(ctx, q, blockHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var txHash []byte
		err = rows.Scan(&txHash)
		if err != nil {
			return nil, errors.Join(store.ErrFailedToGetRows, err)
		}

		cHash, err := chainhash.NewHash(txHash)
		if err != nil {
			return nil, errors.Join(store.ErrFailedToParseHash, err)
		}

		txHashes = append(txHashes, cHash)
	}

	return txHashes, nil
}
