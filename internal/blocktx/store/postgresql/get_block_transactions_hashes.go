package postgresql

import (
	"context"
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) GetBlockTransactionsHashes(ctx context.Context, blockHash []byte) (txHashes []*chainhash.Hash, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetBlockTransactionsHashes", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
		SELECT
			t.hash
		FROM blocktx.transactions AS t
			JOIN blocktx.block_transactions_map AS m ON t.hash = m.txhash
			JOIN blocktx.blocks AS b ON m.blockid = b.id
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
