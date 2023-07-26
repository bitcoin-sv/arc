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
	queryGetBlockHashHeightForTransactionHashes = `
SELECT
b.hash, b.height, t.hash
FROM blocks b
INNER JOIN block_transactions_map m ON m.blockid = b.id
INNER JOIN transactions t ON m.txid = t.id
WHERE t.hash in ('%s')
AND b.orphanedyn = false`
)

func getFullQuery(transactions *blocktx_api.Transactions) string {
	var result []string
	for _, v := range transactions.Transactions {
		result = append(result, fmt.Sprintf("\\x%s", hex.EncodeToString((v.Hash))))
	}

	return fmt.Sprintf(queryGetBlockHashHeightForTransactionHashes, strings.Join(result, "','"))
}

func (s *SQL) GetTransactionBlocks(ctx context.Context, transactions *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("GetTransactionsBlocks").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := &blocktx_api.TransactionBlocks{}

	rows, err := s.db.QueryContext(ctx, getFullQuery(transactions))
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
