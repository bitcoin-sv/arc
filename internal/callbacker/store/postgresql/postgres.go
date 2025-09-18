package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ccoveille/go-safecast"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

const (
	postgresDriverName = "postgres"
)

var (
	ErrFailedToOpenDB = errors.New("failed to open postgres DB")
)

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(m *PostgreSQL) {
		m.now = nowFunc
	}
}

type PostgreSQL struct {
	db  *sql.DB
	now func() time.Time
}

func New(dbInfo string, idleConns int, maxOpenConns int, opts ...func(postgreSQL *PostgreSQL)) (*PostgreSQL, error) {
	db, err := sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, errors.Join(ErrFailedToOpenDB, err)
	}

	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)

	p := &PostgreSQL{
		db:  db,
		now: time.Now,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func (p *PostgreSQL) Close() error {
	return p.db.Close()
}

func (p *PostgreSQL) Insert(ctx context.Context, data []*store.CallbackData) (int64, error) {
	urls := make([]string, len(data))
	tokens := make([]string, len(data))
	timestamps := make([]time.Time, len(data))
	txids := make([]string, len(data))
	txStatuses := make([]string, len(data))
	extraInfos := make([]*string, len(data))
	merklePaths := make([]*string, len(data))
	blockHashes := make([]*string, len(data))
	blockHeights := make([]sql.NullInt64, len(data))
	competingTxs := make([]*string, len(data))
	allowBatches := make([]bool, len(data))
	hashes := make([][]byte, len(data))

	for i, d := range data {
		urls[i] = d.URL
		tokens[i] = d.Token
		timestamps[i] = d.Timestamp
		txids[i] = d.TxID
		txStatuses[i] = d.TxStatus
		extraInfos[i] = d.ExtraInfo
		merklePaths[i] = d.MerklePath
		blockHashes[i] = d.BlockHash
		allowBatches[i] = d.AllowBatch

		if d.BlockHeight != nil {
			blockHeight, err := safecast.ToInt64(*d.BlockHeight)
			if err != nil {
				return 0, fmt.Errorf("failed to convert block height to int64: %w", err)
			}
			blockHeights[i] = sql.NullInt64{Int64: blockHeight, Valid: true}
		}

		if len(d.CompetingTxs) > 0 {
			competingTxs[i] = ptrTo(strings.Join(d.CompetingTxs, ","))
		}
		hash, err := chainhash.NewHashFromStr(d.TxID)
		if err != nil {
			return 0, fmt.Errorf("failed to convert txid to hash: %w", err)
		}

		hashes[i] = hash[:]
	}

	const query = `INSERT INTO callbacker.transaction_callbacks (
				url
				,token
				,tx_id
				,tx_status
				,extra_info
				,merkle_path
				,block_hash
				,block_height
				,timestamp
				,competing_txs
				,allow_batch
				,hash
				)
				SELECT
					UNNEST($1::TEXT[])
					,UNNEST($2::TEXT[])
					,UNNEST($3::TEXT[])
					,UNNEST($4::TEXT[])
					,UNNEST($5::TEXT[])
					,UNNEST($6::TEXT[])
					,UNNEST($7::TEXT[])
					,UNNEST($8::BIGINT[])
					,UNNEST($9::TIMESTAMPTZ[])
					,UNNEST($10::TEXT[])
					,UNNEST($11::BOOLEAN[])
					,UNNEST($12::BYTEA[])
					ON CONFLICT DO NOTHING
					`

	result, err := p.db.ExecContext(ctx, query,
		pq.Array(urls),
		pq.Array(tokens),
		pq.Array(txids),
		pq.Array(txStatuses),
		pq.Array(extraInfos),
		pq.Array(merklePaths),
		pq.Array(blockHashes),
		pq.Array(blockHeights),
		pq.Array(timestamps),
		pq.Array(competingTxs),
		pq.Array(allowBatches),
		pq.Array(hashes),
	)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, err
}

func (p *PostgreSQL) GetUnsent(ctx context.Context, limit int, expiration time.Duration, batch bool, maxRetries int) ([]*store.CallbackData, error) {
	const q = `
				WITH updated AS (
				UPDATE callbacker.transaction_callbacks c SET pending = $1
				WHERE c.id IN (
				    SELECT id FROM callbacker.transaction_callbacks c
					WHERE timestamp > $2 AND allow_batch = $3 AND sent_at IS NULL AND (c.pending IS NULL OR c.pending < $5) AND retries < $6 AND disable IS NOT TRUE
					AND NOT EXISTS (
					SELECT 1 FROM callbacker.transaction_callbacks c1
					WHERE c1.hash=c.hash AND c1.url=c.url AND c1.pending IS NOT NULL AND c1.pending > $5 -- skip those with hash for which there are already pending callbacks
					)
					LIMIT $4
					FOR UPDATE
				)
				RETURNING
				c.id
			    ,c.url
				,c.token
				,c.tx_id
				,c.tx_status
				,c.extra_info
				,c.merkle_path
				,c.block_hash
				,c.block_height
				,c.competing_txs
				,c.timestamp
				,c.allow_batch
				)
				SELECT *
				FROM updated
				ORDER BY updated.timestamp ASC
				;
			`

	const lockTime = 3 * time.Minute
	expirationDate := p.now().Add(-1 * expiration)
	rows, err := p.db.QueryContext(ctx, q, p.now(), expirationDate, batch, limit, p.now().Add(-1*lockTime), maxRetries)
	if err != nil {
		return nil, err
	}

	var records []*store.CallbackData
	records, err = scanCallbacks(rows, limit)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (p *PostgreSQL) Clear(ctx context.Context, t time.Time) error {
	const q = `DELETE FROM callbacker.transaction_callbacks
			WHERE timestamp <= $1`

	_, err := p.db.ExecContext(ctx, q, t)
	return err
}

func (p *PostgreSQL) SetSent(ctx context.Context, ids []int64) error {
	const q = `UPDATE callbacker.transaction_callbacks SET sent_at = $1, pending = NULL, retries = retries+1 WHERE id = ANY($2::INTEGER[])`

	_, err := p.db.ExecContext(ctx, q, p.now(), pq.Array(ids))
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) UnsetPending(ctx context.Context, ids []int64) error {
	const q = `UPDATE callbacker.transaction_callbacks SET pending = NULL, retries = retries+1 WHERE id = ANY($1::INTEGER[])`

	_, err := p.db.ExecContext(ctx, q, pq.Array(ids))
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) UnsetPendingDisable(ctx context.Context, ids []int64) error {
	const q = `UPDATE callbacker.transaction_callbacks SET pending = NULL, retries = retries+1, disable = TRUE WHERE id = ANY($1::INTEGER[])`

	_, err := p.db.ExecContext(ctx, q, pq.Array(ids))
	if err != nil {
		return err
	}

	return nil
}

func scanCallbacks(rows *sql.Rows, expectedNumber int) ([]*store.CallbackData, error) {
	records := make([]*store.CallbackData, 0, expectedNumber)

	for rows.Next() {
		r := &store.CallbackData{}

		var (
			ts      time.Time
			ei      sql.NullString
			mp      sql.NullString
			bh      sql.NullString
			bHeight sql.NullInt64
			ctxs    sql.NullString
			id      sql.NullInt64
		)

		err := rows.Scan(
			&id,
			&r.URL,
			&r.Token,
			&r.TxID,
			&r.TxStatus,
			&ei,
			&mp,
			&bh,
			&bHeight,
			&ctxs,
			&ts,
			&r.AllowBatch,
		)

		if err != nil {
			return nil, err
		}

		r.Timestamp = ts.UTC()

		if id.Valid {
			r.ID = id.Int64
		}
		if ei.Valid {
			r.ExtraInfo = ptrTo(ei.String)
		}
		if mp.Valid {
			r.MerklePath = ptrTo(mp.String)
		}
		if bh.Valid {
			r.BlockHash = ptrTo(bh.String)
		}
		bhuint64, err := safecast.ToUint64(bHeight.Int64)
		if err != nil {
			return nil, err
		}
		if bHeight.Valid {
			r.BlockHeight = ptrTo(bhuint64)
		}
		if ctxs.String != "" {
			r.CompetingTxs = strings.Split(ctxs.String, ",")
		}

		records = append(records, r)
	}

	return records, nil
}

func ptrTo[T any](v T) *T {
	return &v
}
