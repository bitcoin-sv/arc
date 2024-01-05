package sql

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"modernc.org/sqlite"
)

const (
	queryGetBlockHashHeightForTransactionHash = `
		SELECT
		 b.hash, b.height
		FROM blocks b
		INNER JOIN block_transactions_map m ON m.blockid = b.id
		INNER JOIN transactions t ON m.txid = t.id
		WHERE t.hash = $1
		AND b.orphanedyn = false
	`
)

// RegisterTransaction registers a transaction in the database.
func (s *SQL) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (string, string, []byte, uint64, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("RegisterTransaction").AddTime(start)
	}()
	var spanCtx context.Context
	if opentracing.IsGlobalTracerRegistered() {
		var span opentracing.Span
		span, spanCtx = opentracing.StartSpanFromContext(ctx, "sql:RegisterTransaction")
		defer span.Finish()
	} else {
		spanCtx = ctx
	}

	ctx, cancel := context.WithCancel(spanCtx)
	defer cancel()

	if transaction.Hash == nil {
		return "", "", nil, 0, fmt.Errorf("invalid request - no hash")
	}

	if transaction.GetSource() == "" {
		return "", "", nil, 0, fmt.Errorf("source missing for transaction %s", utils.ReverseAndHexEncodeSlice(transaction.GetHash()))
	}

	q := `INSERT INTO transactions (hash, source) VALUES ($1, $2)`

	if _, err := s.db.ExecContext(ctx, q, transaction.Hash[:], transaction.Source); err != nil {
		var spanErr opentracing.Span
		if opentracing.IsGlobalTracerRegistered() {
			spanErr, ctx = opentracing.StartSpanFromContext(ctx, "sql:RegisterTransaction:Err")
			defer spanErr.Finish()
		}

		var uniqueConstraint bool

		// Check if this is a postgres unique constraint violation
		pErr, ok := err.(*pq.Error)
		if ok && pErr.Code == "23505" {
			uniqueConstraint = true
		} else {
			// Check if this ia a sqlite unique constraint violation
			sErr, ok := err.(*sqlite.Error)
			if ok && sErr.Code() == 2067 {
				uniqueConstraint = true
			}
		}

		if !uniqueConstraint {
			if spanErr != nil {
				spanErr.SetTag(string(ext.Error), true)
				spanErr.LogFields(log.Error(err))
			}
			return "", "", nil, 0, err
		}

		// If we reach here, we have a unique violation, which means that the transaction already exists
		if spanErr != nil {
			spanErr.SetTag("unique_constraint", true)
		}
		q = `UPDATE transactions SET source = $1 WHERE source IS NULL AND hash = $2 RETURNING merkle_path`
		result, err := s.db.QueryContext(ctx, q, transaction.Source, transaction.Hash[:])
		if err != nil {
			if spanErr != nil {
				spanErr.SetTag(string(ext.Error), true)
				spanErr.LogFields(log.Error(err))
			}
			return "", "", nil, 0, err
		}

		var rows int
		var merklePath string
		for result.Next() {
			if err := result.Scan(&merklePath); err != nil {
				return "", "", nil, 0, err
			}
			rows++
		}

		if rows == 1 {
			// We successfully updated the source and which means it had already been mined
			// so we return the block hash and height
			var blockHashBytes []byte
			var blockHeight uint64

			if spanErr != nil {
				spanErr.SetTag("already_mined", true)
			}

			if err := s.db.QueryRowContext(ctx, queryGetBlockHashHeightForTransactionHash, transaction.GetHash()).Scan(&blockHashBytes, &blockHeight); err != nil {
				return "", "", nil, 0, err
			}

			return transaction.GetSource(), merklePath, blockHashBytes, blockHeight, nil
		}

		var source string
		if err := s.db.QueryRowContext(ctx, "SELECT source, merkle_path FROM transactions WHERE hash = $1", transaction.GetHash()).Scan(&source, &merklePath); err != nil {
			if spanErr != nil {
				spanErr.SetTag(string(ext.Error), true)
				spanErr.LogFields(log.Error(err))
			}
			return "", "", nil, 0, err
		}
		return source, merklePath, nil, 0, nil
	}

	return transaction.GetSource(), "", nil, 0, nil
}
