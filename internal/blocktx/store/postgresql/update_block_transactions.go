package postgresql

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

// UpdateBlockTransactions updates the transaction hashes for a given block hash.
func (p *PostgreSQL) UpdateBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpdateBlockTransactionsResult, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "UpdateBlockTransactions")
		defer span.End()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(transactions) != len(merklePaths) {
		return nil, fmt.Errorf("transactions (len=%d) and Merkle paths (len=%d) have not the same lengths", len(transactions), len(merklePaths))
	}

	txHashes := make([][]byte, len(transactions))
	txHashesMap := map[string]int{}
	for pos, tx := range transactions {

		txHashes[pos] = tx.Hash

		txHashesMap[hex.EncodeToString(tx.Hash)] = pos
	}

	qBulkUpdate := `
		UPDATE transactions
			SET
			  merkle_path=bulk_query.merkle_path
			FROM
			  (
				SELECT *
				FROM
				  UNNEST($1::BYTEA[], $2::TEXT[])
				  AS t(hash, merkle_path)
			  ) AS bulk_query
			WHERE
			  transactions.hash=bulk_query.hash
		RETURNING transactions.id, transactions.hash, transactions.merkle_path
`

	qMap := `
		INSERT INTO block_transactions_map (
		 blockid
		,txid
		,pos
		) SELECT * FROM UNNEST($1::INT[], $2::INT[], $3::INT[])
		ON CONFLICT DO NOTHING
	`

	results := make([]store.UpdateBlockTransactionsResult, 0)

	rows, err := p.db.QueryContext(ctx, qBulkUpdate, pq.Array(txHashes), pq.Array(merklePaths))
	if err != nil {
		return nil, fmt.Errorf("failed to execute transaction update query: %v", err)
	}

	txIDs := make([]uint64, 0)
	blockIDs := make([]uint64, 0)
	positions := make([]int, 0)

	for rows.Next() {
		var txID uint64
		var txHash []byte
		var merklePath string
		err = rows.Scan(&txID, &txHash, &merklePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get rows: %v", err)
		}

		results = append(results, store.UpdateBlockTransactionsResult{
			TxHash:     txHash,
			MerklePath: merklePath,
		})

		txIDs = append(txIDs, txID)
		blockIDs = append(blockIDs, blockId)

		positions = append(positions, txHashesMap[hex.EncodeToString(txHash)])

		if len(txIDs) >= p.maxPostgresBulkInsertRows {
			_, err = p.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(positions))
			if err != nil {
				return nil, fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
			}
			txIDs = make([]uint64, 0)
			blockIDs = make([]uint64, 0)
			positions = make([]int, 0)
		}
	}

	if len(txIDs) > 0 {
		_, err = p.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(positions))
		if err != nil {
			return nil, fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
		}
	}

	return results, nil
}
