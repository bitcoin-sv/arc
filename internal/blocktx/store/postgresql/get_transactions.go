package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
)

func (p *PostgreSQL) GetMinedTransactions(ctx context.Context, hashes [][]byte) ([]store.TransactionBlock, error) {
	if tracer != nil {
		var span trace.Span
		ctx, span = tracer.Start(ctx, "GetMinedTransactions")
		defer span.End()
	}

	predicate := "WHERE t.hash = ANY($1) AND b.status = $2"

	return p.getTransactionBlocksByPredicate(ctx, predicate, pq.Array(hashes), blocktx_api.Status_LONGEST)
}

func (p *PostgreSQL) GetRegisteredTransactions(ctx context.Context, blockHashes [][]byte) ([]store.TransactionBlock, error) {
	predicate := "WHERE b.hash = ANY($1) AND t.is_registered = TRUE"

	return p.getTransactionBlocksByPredicate(ctx, predicate, blockHashes)
}

func (p *PostgreSQL) GetRegisteredTxsByBlockHashes(ctx context.Context, blockHashes [][]byte) ([]store.TransactionBlock, error) {
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
			b.status
	  FROM blocktx.transactions AS t
	  	JOIN blocktx.block_transactions_map AS m ON t.id = m.txid
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
		var blockStatus blocktx_api.Status

		err = rows.Scan(
			&txHash,
			&blockHash,
			&blockHeight,
			&merklePath,
			&blockStatus,
		)
		if err != nil {
			return nil, err
		}

		transactionBlocks = append(transactionBlocks, store.TransactionBlock{
			TxHash:      txHash,
			BlockHash:   blockHash,
			BlockHeight: blockHeight,
			MerklePath:  merklePath,
			BlockStatus: blockStatus,
		})

	}

	return transactionBlocks, nil
}
