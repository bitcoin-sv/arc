package postgresql

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes [][]byte) (minedTransactions []store.BlockTransaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetMinedTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
		SELECT
			bt.hash,
			b.hash,
			b.height,
			bt.merkle_tree_index,
			b.status,
			b.merkleroot,
			b.timestamp
		FROM blocktx.block_transactions AS bt
			JOIN blocktx.blocks AS b ON bt.block_id = b.id
		WHERE bt.hash = ANY($1) AND (b.status = $2 OR b.status = $3) AND b.processed_at IS NOT NULL
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashes),
		blocktx_api.Status_LONGEST,
		blocktx_api.Status_STALE)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.getBlockTransactions(rows)
}

func (p *PostgreSQL) getBlockTransactions(rows *sql.Rows) ([]store.BlockTransaction, error) {
	transactionBlocks := make([]store.BlockTransaction, 0)
	for rows.Next() {
		var txHash []byte
		var blockHash []byte
		var blockHeight uint64
		var merkleTreeIndex int64
		var blockStatus blocktx_api.Status
		var merkleRoot []byte
		var timestamp sql.NullTime

		err := rows.Scan(
			&txHash,
			&blockHash,
			&blockHeight,
			&merkleTreeIndex,
			&blockStatus,
			&merkleRoot,
			&timestamp,
		)
		if err != nil {
			return nil, err
		}

		bt := store.BlockTransaction{
			TxHash:          txHash,
			BlockHash:       blockHash,
			BlockHeight:     blockHeight,
			MerkleTreeIndex: merkleTreeIndex,
			BlockStatus:     blockStatus,
			MerkleRoot:      merkleRoot,
		}

		if timestamp.Valid {
			bt.Timestamp = timestamp.Time
		}

		transactionBlocks = append(transactionBlocks, bt)
	}

	return transactionBlocks, nil
}

func (p *PostgreSQL) GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) (registeredTxs []store.BlockTransaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetRegisteredTxsByBlockHashes", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
		SELECT
			bt.hash,
			b.hash,
			b.height,
			bt.merkle_tree_index,
			b.status,
			b.merkleroot,
			b.timestamp
		FROM blocktx.registered_transactions AS r
			JOIN blocktx.block_transactions AS bt ON r.hash = bt.hash
			JOIN blocktx.blocks AS b ON bt.block_id = b.id
		WHERE b.hash = ANY($1)
	`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(blockHashes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.getBlockTransactions(rows)
}
