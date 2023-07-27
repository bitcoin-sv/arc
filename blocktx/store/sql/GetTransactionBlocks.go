package sql

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/ordishs/gocore"

	"context"
)

const (
	queryGetBlockHashHeightForTxHashesPostgres = `
SELECT
b.hash, b.height, t.hash
FROM blocks b
INNER JOIN block_transactions_map m ON m.blockid = b.id
INNER JOIN transactions t ON m.txid = t.id
WHERE t.hash in ('%s')
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

func getQueryPostgres(transactions *blocktx_api.Transactions) string {
	var result []string
	for _, v := range transactions.Transactions {
		result = append(result, fmt.Sprintf("\\x%x", (v.Hash)))
	}

	return fmt.Sprintf(queryGetBlockHashHeightForTxHashesPostgres, strings.Join(result, "','"))
}

func getQuerySQLite(transactions *blocktx_api.Transactions) string {
	var result []string
	for _, v := range transactions.Transactions {
		result = append(result, strings.ToUpper(hex.EncodeToString(v.Hash)))
	}

	return fmt.Sprintf(queryGetBlockHashHeightForTxHashesSQLite, strings.Join(result, "','"))
}

func (s *SQL) GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionsBlocks").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := &blocktx_api.TransactionBlocks{}
	var query string

	switch s.engine {
	case sqliteEngine:
		query = getQuerySQLite(transactions)
	case sqliteMemoryEngine:
		query = getQuerySQLite(transactions)
	case postgresEngine:
		query = getQueryPostgres(transactions)
	default:
		return nil, fmt.Errorf("engine not supported: %s", s.engine)
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var BlockHash []byte
		var BlockHeight uint64
		var TransactionHash []byte
		err := rows.Scan(&BlockHash, &BlockHeight, &TransactionHash)
		if err != nil {
			return nil, err
		}

		newBlockTransaction := &blocktx_api.TransactionBlock{
			BlockHash:       BlockHash,
			BlockHeight:     BlockHeight,
			TransactionHash: TransactionHash,
		}

		results.TransactionBlocks = append(results.TransactionBlocks, newBlockTransaction)
	}

	return results, nil
}
