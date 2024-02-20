package postgresql

import (
	"context"
	"database/sql"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/lib/pq"
	"github.com/ordishs/gocore"
)

const (
	queryGetBlockHashHeightForTxHashesPostgres = `
			SELECT
			b.hash, b.height, t.hash
			FROM blocks b
			INNER JOIN block_transactions_map m ON m.blockid = b.id
			INNER JOIN transactions t ON m.txid = t.id
			WHERE t.hash = ANY($1)
			AND b.orphanedyn = FALSE`
)

func (s *PostgreSQL) GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	start := gocore.CurrentNanos()

	defer gocore.NewStat("blocktx").NewStat("GetTransactionsBlocks").AddTime(start)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := &blocktx_api.TransactionBlocks{}
	var rows *sql.Rows
	var err error

	var hashSlice [][]byte
	for _, tx := range transactions.GetTransactions() {
		hashSlice = append(hashSlice, tx.GetHash())
	}

	rows, err = s.db.QueryContext(ctx, queryGetBlockHashHeightForTxHashesPostgres, pq.Array(hashSlice))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var BlockHash []byte
		var BlockHeight sql.NullInt64
		var TransactionHash []byte
		err := rows.Scan(&BlockHash, &BlockHeight, &TransactionHash)
		if err != nil {
			return nil, err
		}

		newBlockTransaction := &blocktx_api.TransactionBlock{
			BlockHash:       BlockHash,
			BlockHeight:     uint64(BlockHeight.Int64),
			TransactionHash: TransactionHash,
		}

		results.TransactionBlocks = append(results.GetTransactionBlocks(), newBlockTransaction)
	}

	return results, nil
}
