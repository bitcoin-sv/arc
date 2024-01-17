package sql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

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

	queryGetBlockHashHeightForTxHashesSQLite = `
			SELECT
			b.hash, b.height, t.hash
			FROM blocks b
			INNER JOIN block_transactions_map m ON m.blockid = b.id
			INNER JOIN transactions t ON m.txid = t.id
			WHERE HEX(t.hash) in ('%s')
			AND b.orphanedyn = FALSE`
)

func getQuerySQLite(transactions *blocktx_api.Transactions) string {
	var result []string
	for _, v := range transactions.GetTransactions() {
		result = append(result, strings.ToUpper(hex.EncodeToString(v.GetHash())))
	}

	return fmt.Sprintf(queryGetBlockHashHeightForTxHashesSQLite, strings.Join(result, "','"))
}

func (s *SQL) GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	start := gocore.CurrentNanos()

	defer gocore.NewStat("blocktx").NewStat("GetTransactionsBlocks").AddTime(start)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := &blocktx_api.TransactionBlocks{}
	var rows *sql.Rows
	var err error

	switch s.engine {
	case sqliteEngine:
		fallthrough
	case sqliteMemoryEngine:
		rows, err = s.db.QueryContext(ctx, getQuerySQLite(transactions))
		if err != nil {
			return nil, err
		}
	case postgresEngine:
		var hashSlice [][]byte
		for _, tx := range transactions.GetTransactions() {
			hashSlice = append(hashSlice, tx.GetHash())
		}

		rows, err = s.db.QueryContext(ctx, queryGetBlockHashHeightForTxHashesPostgres, pq.Array(hashSlice))
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("engine not supported: %s", s.engine)
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
