package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/tracing"
)

func (p *PostgreSQL) GetUnminedRegisteredTxsHashes(ctx context.Context) (txHashes [][]byte, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetUnminedRegisteredTxsHashes", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
		SELECT
			t.hash
		FROM blocktx.transactions AS t
		LEFT JOIN blocktx.block_transactions_map AS m ON t.hash = m.txhash
		WHERE m.blockid IS NULL AND t.is_registered = true
	`

	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var txHash []byte

		err = rows.Scan(&txHash)
		if err != nil {
			return nil, err
		}

		txHashes = append(txHashes, txHash)
	}

	return txHashes, nil
}
