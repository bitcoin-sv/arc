package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
)

func (p *PostgreSQL) GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) (longestTxs []store.GetMinedTransactionResult, staleTxs []store.GetMinedTransactionResult, err error) {
	q := `
		SELECT
			t.hash,
			b.hash,
	  	b.height,
	  	m.merkle_path,
			b.status
		FROM blocktx.blocks AS b
	  	JOIN blocktx.block_transactions_map AS m ON m.blockid = b.id
			JOIN blocktx.transactions AS t ON t.id = m.txid AND t.is_registered = TRUE
	  WHERE b.hash = ANY($1)
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(blockHashes))
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var txHash []byte
		var blockHash []byte
		var blockHeight uint64
		var merklePath string
		var status blocktx_api.Status

		err = rows.Scan(
			&txHash,
			&blockHash,
			&blockHeight,
			&merklePath,
			&status,
		)
		if err != nil {
			return
		}

		result := store.GetMinedTransactionResult{
			TxHash:      txHash,
			BlockHash:   blockHash,
			BlockHeight: blockHeight,
			MerklePath:  merklePath,
		}

		switch status {
		case blocktx_api.Status_LONGEST:
			longestTxs = append(longestTxs, result)
		case blocktx_api.Status_STALE:
			staleTxs = append(staleTxs, result)
		default:
			// do nothing - ignore ORPHANED and UNKNOWN blocks
		}
	}

	return
}
