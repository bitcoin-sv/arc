package sql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/lib/pq"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

// UpdateBlockTransactions updates the transaction hashes for a given block hash.
func (s *SQL) UpdateBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpdateBlockTransactionsResult, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("UpdateBlockTransactions").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(transactions) != len(merklePaths) {
		return nil, fmt.Errorf("transactions (len=%d) and Merkle paths (len=%d) have not the same lengths", len(transactions), len(merklePaths))
	}

	switch s.engine {
	case sqliteEngine:
		fallthrough
	case sqliteMemoryEngine:
		return nil, s.updateBlockTransactionsSqLite(ctx, blockId, transactions, merklePaths)
	case postgresEngine:
		return s.updateBlockTransactionsPostgres(ctx, blockId, transactions, merklePaths)
	}

	return nil, fmt.Errorf("engine not supported: %s", s.engine)
}

func (s *SQL) updateBlockTransactionsSqLite(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("InsertBlockTransactions").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	qTx, err := s.db.Prepare(`
			UPDATE transactions SET merkle_path = $2 WHERE hash = $1 RETURNING id;
		`)
	if err != nil {
		return fmt.Errorf("failed to prepare query for insertion into transactions: %v", err)
	}

	qMap := `
		INSERT INTO block_transactions_map (
		 blockid
		,txid
		,pos
		) VALUES
	`

	qMapRows := make([]string, 0, len(transactions))
	for pos, tx := range transactions {
		var txid uint64

		err = qTx.QueryRowContext(ctx, tx.GetHash(), merklePaths[pos]).Scan(&txid)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("failed to execute insertion of tx %s into transactions table: %v", utils.ReverseAndHexEncodeSlice(tx.GetHash()), err)
			}

			err = s.db.QueryRowContext(ctx, `
					SELECT id
					FROM transactions
					WHERE hash = $1
				`, tx.GetHash()).Scan(&txid)
			if err != nil {
				return fmt.Errorf("failed to query for transactions with id %d: %v", txid, err)
			}
		}

		// this is ugly, but a lot faster than sprintf
		qMapRows = append(qMapRows, " ("+strconv.FormatUint(blockId, 10)+", "+strconv.FormatUint(txid, 10)+", "+strconv.Itoa(pos)+")")

		// maximum of 1000 rows per query is allowed in postgres
		if len(qMapRows) >= 1000 {
			if err = s.bulkInsert(ctx, qMap, qMapRows); err != nil {
				return fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
			}
			qMapRows = qMapRows[:0]
		}
	}

	// insert the remaining rows
	if len(qMapRows) > 0 {
		if err = s.bulkInsert(ctx, qMap, qMapRows); err != nil {
			return fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
		}
	}

	// Todo: return updated rows

	return nil
}

func (s *SQL) bulkInsert(ctx context.Context, queryTemplate string, queryRows []string) error {
	// remove the last comma
	query := queryTemplate + strings.Join(queryRows, ",")
	query += ` ON CONFLICT DO NOTHING;`

	// insert the block / transaction map in 1 query
	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQL) updateBlockTransactionsPostgres(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpdateBlockTransactionsResult, error) {
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

	rows, err := s.db.QueryContext(ctx, qBulkUpdate, pq.Array(txHashes), pq.Array(merklePaths))
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

		if len(txIDs) >= s.maxPostgresBulkInsertRows {
			_, err = s.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(positions))
			if err != nil {
				return nil, fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
			}
			txIDs = make([]uint64, 0)
			blockIDs = make([]uint64, 0)
			positions = make([]int, 0)
		}
	}

	if len(txIDs) > 0 {
		_, err = s.db.ExecContext(ctx, qMap, pq.Array(blockIDs), pq.Array(txIDs), pq.Array(positions))
		if err != nil {
			return nil, fmt.Errorf("failed to bulk insert transactions into block transactions map for block with id %d: %v", blockId, err)
		}
	}

	return results, nil
}
