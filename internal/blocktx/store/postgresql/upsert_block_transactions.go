package postgresql

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

// UpsertBlockTransactions upserts the transaction hashes for a given block hash and returns updated registered transactions hashes.
func (p *PostgreSQL) UpsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpsertBlockTransactionsResult, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "UpdateBlockTransactions")
		defer span.End()
	}

	if len(transactions) != len(merklePaths) {
		return nil, fmt.Errorf("transactions (len=%d) and Merkle paths (len=%d) have not the same lengths", len(transactions), len(merklePaths))
	}

	txHashes := make([][]byte, len(transactions))
	txHashesMap := map[string]int{}
	for pos, tx := range transactions {
		txHashes[pos] = tx.Hash
		txHashesMap[hex.EncodeToString(tx.Hash)] = pos
	}

	qBulkUpsert := `
		INSERT INTO blocktx.transactions (hash, merkle_path)
			SELECT hash, merkle_path
			FROM UNNEST($1::BYTEA[], $2::TEXT[]) AS t(hash, merkle_path)
		ON CONFLICT (hash) DO UPDATE SET
  			merkle_path = EXCLUDED.merkle_path
		RETURNING id, hash, merkle_path, is_registered;`

	rows, err := p.db.QueryContext(ctx, qBulkUpsert, pq.Array(txHashes), pq.Array(merklePaths))
	if err != nil {
		return nil, fmt.Errorf("failed to execute transaction update query: %v", err)
	}

	txIDs := make([]uint64, 0)
	blockIDs := make([]uint64, 0)
	positions := make([]int, 0)
	registeredRows := make([]store.UpsertBlockTransactionsResult, 0)

	for rows.Next() {
		var txID uint64
		var txHash []byte
		var merklePath string
		var isRegistered bool
		err = rows.Scan(&txID, &txHash, &merklePath, &isRegistered)
		if err != nil {
			return nil, fmt.Errorf("failed to get rows: %v", err)
		}

		if isRegistered {
			registeredRows = append(registeredRows, store.UpsertBlockTransactionsResult{
				TxHash:     txHash,
				MerklePath: merklePath,
			})
		}

		txIDs = append(txIDs, txID)
		blockIDs = append(blockIDs, blockId)

		positions = append(positions, txHashesMap[hex.EncodeToString(txHash)])

		if len(txIDs) >= p.maxPostgresBulkInsertRows {
			err = p.insertTxsIntoBlockMap(ctx, blockId, blockIDs, txIDs, positions)
			if err != nil {
				return nil, err
			}
			txIDs = make([]uint64, 0)
			blockIDs = make([]uint64, 0)
			positions = make([]int, 0)
		}
	}

	if len(txIDs) > 0 {
		err = p.insertTxsIntoBlockMap(ctx, blockId, blockIDs, txIDs, positions)
		if err != nil {
			return nil, err
		}
	}

	return registeredRows, nil
}

func (p *PostgreSQL) insertTxsIntoBlockMap(ctx context.Context, blockId uint64, blockIDs, txIDs []uint64, positions []int) error {
	const qMap = `
		INSERT INTO blocktx.block_transactions_map (
			blockid
			,txid
			,pos
			) 
		SELECT * FROM UNNEST($1::INT[], $2::INT[], $3::INT[])
		ON CONFLICT DO NOTHING
		`
	_, err := p.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(positions))
	if err != nil {
		return fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
	}

	return nil
}
