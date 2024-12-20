package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/lib/pq"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes [][]byte, onlyLongestChain bool) (minedTransactions []store.TransactionBlock, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetMinedTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if onlyLongestChain {
		predicate := "WHERE t.hash = ANY($1) AND b.is_longest = true"
		return p.getTransactionBlocksByPredicate(ctx, predicate, pq.Array(hashes))
	}

	predicate := "WHERE t.hash = ANY($1) AND (b.status = $2 OR b.status = $3) AND b.processed_at IS NOT NULL"

	return p.getTransactionBlocksByPredicate(ctx, predicate,
		pq.Array(hashes),
		blocktx_api.Status_LONGEST,
		blocktx_api.Status_STALE,
	)
}

func (p *PostgreSQL) GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) (registeredTxs []store.TransactionBlock, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetMinedTransactions", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	predicate := "WHERE b.hash = ANY($1) AND t.is_registered = TRUE"

	return p.getTransactionBlocksByPredicate(ctx, predicate, pq.Array(blockHashes))
}

func (p *PostgreSQL) getTransactionBlocksByPredicate(ctx context.Context, predicate string, predicateParams ...any) ([]store.TransactionBlock, error) {
	transactionBlocks := make([]store.TransactionBlock, 0)

	q := `
		SELECT
			t.hash,
			b.hash,
			b.height,
			m.merkle_path,
			m.merkle_tree_index,
			b.status
		FROM blocktx.transactions AS t
			JOIN blocktx.block_transactions_map AS m ON t.hash = m.txhash
			JOIN blocktx.blocks AS b ON m.blockid = b.id
	`
	q += " " + predicate

	rows, err := p.db.QueryContext(ctx, q, predicateParams...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var txHash []byte
		var blockHash []byte
		var blockHeight uint64
		var merklePath string
		var merkleTreeIndex int64
		var blockStatus blocktx_api.Status

		err = rows.Scan(
			&txHash,
			&blockHash,
			&blockHeight,
			&merklePath,
			&merkleTreeIndex,
			&blockStatus,
		)
		if err != nil {
			return nil, err
		}

		transactionBlocks = append(transactionBlocks, store.TransactionBlock{
			TxHash:          txHash,
			BlockHash:       blockHash,
			BlockHeight:     blockHeight,
			MerklePath:      merklePath,
			MerkleTreeIndex: merkleTreeIndex,
			BlockStatus:     blockStatus,
		})
	}

	return transactionBlocks, nil
}
