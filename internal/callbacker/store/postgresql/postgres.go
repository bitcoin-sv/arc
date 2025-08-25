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

func (p *PostgreSQL) SetMany(ctx context.Context, data []*store.CallbackData) error {
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
				return fmt.Errorf("failed to convert block height to int64: %w", err)
			}
			blockHeights[i] = sql.NullInt64{Int64: blockHeight, Valid: true}
		}

		if len(d.CompetingTxs) > 0 {
			competingTxs[i] = ptrTo(strings.Join(d.CompetingTxs, ","))
		}
	}

	const query = `INSERT INTO callbacker.callbacks (
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
					ON CONFLICT DO NOTHING
					`

	_, err := p.db.ExecContext(ctx, query,
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
	)

	return err
}

func (p *PostgreSQL) GetMany(ctx context.Context, limit int, expiration time.Duration, batch bool) ([]*store.CallbackData, error) {
	const q = `
				UPDATE callbacker.callbacks c SET pending = $1
				WHERE id IN (
				    SELECT id FROM callbacker.callbacks
					WHERE timestamp > $2 AND allow_batch = $3 AND sent_at IS NULL AND (c.pending IS NULL OR c.pending > $4)
					AND NOT EXISTS (
					SELECT 1 FROM callbacker.callbacks c1
					WHERE c1.url=c.url AND (c1.pending IS NOT NULL OR c.pending > $4)
					)
					ORDER BY timestamp ASC
					LIMIT $5
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
				;
			`

	lockTime := 120 * time.Second
	expirationDate := p.now().Add(-1 * expiration)
	rows, err := p.db.QueryContext(ctx, q, p.now(), expirationDate, batch, p.now().Add(-1*lockTime), limit)
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

// GetAndMarkSent returns and marks sent a number of callbacks limited by `limit` ordered by timestamp in ascending order
func (p *PostgreSQL) GetAndMarkSent(ctx context.Context, url string, limit int, expiration time.Duration, batch bool) (data []*store.CallbackData, commitFunc func() error, rollbackFunc func() error, err error) {
	const q = `UPDATE callbacker.callbacks SET sent_at = $5
			WHERE id IN (
				SELECT id FROM callbacker.callbacks
				WHERE url = $1 AND timestamp > $2 AND allow_batch = $3 AND sent_at IS NULL
				ORDER BY timestamp ASC
				LIMIT $4
				FOR UPDATE
			)
			RETURNING
				id
			    ,url
				,token
				,tx_id
				,tx_status
				,extra_info
				,merkle_path
				,block_hash
				,block_height
				,competing_txs
				,timestamp
				,allow_batch`

	expirationDate := p.now().Add(-1 * expiration)

	tx, err := p.db.Begin()
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := tx.QueryContext(ctx, q, url, expirationDate, batch, limit, p.now())
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	var records []*store.CallbackData
	records, err = scanCallbacks(rows, limit)
	if err != nil {
		return nil, nil, nil, err
	}

	return records, tx.Commit, tx.Rollback, nil
}

func (p *PostgreSQL) DeleteOlderThan(ctx context.Context, t time.Time) error {
	const q = `DELETE FROM callbacker.callbacks
			WHERE timestamp <= $1`

	_, err := p.db.ExecContext(ctx, q, t)
	return err
}

func (p *PostgreSQL) SetURLMapping(ctx context.Context, m store.URLMapping) error {
	const q = `INSERT INTO callbacker.url_mapping (
		 url
		,instance
	) VALUES (
		 $1
		,$2
	)`

	var pqErr *pq.Error
	_, err := p.db.ExecContext(ctx, q, m.URL, m.Instance)
	// Error 23505 is: "duplicate key violates unique constraint"
	if errors.As(err, &pqErr) && pqErr.Code == pq.ErrorCode("23505") {
		return store.ErrURLMappingDuplicateKey
	}

	return nil
}

func (p *PostgreSQL) SetSent(ctx context.Context, ids []int64) error {
	const q = `UPDATE callbacker.callbacks SET sent_at = $1, pending = NULL WHERE id = ANY($2::INTEGER[])`

	_, err := p.db.ExecContext(ctx, q, time.Now(), ids)
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) SetNotPending(ctx context.Context, ids []int64) error {
	const q = `UPDATE callbacker.callbacks SET pending = NULL WHERE id = ANY($1::INTEGER[])`

	_, err := p.db.ExecContext(ctx, q, pq.Array(ids))
	if err != nil {
		return err
	}

	return nil
}

// GetUnmappedURL Returns unmapped URLs for which there exists a pending callback in the callbacks table
func (p *PostgreSQL) GetUnmappedURL(ctx context.Context) (url string, err error) {
	const q = `SELECT c.url FROM callbacker.callbacks c LEFT JOIN callbacker.url_mapping um ON um.url = c.url WHERE um.url IS NULL LIMIT 1;`

	err = p.db.QueryRowContext(ctx, q).Scan(&url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", store.ErrNoUnmappedURLsFound
		}

		return "", err
	}

	return url, nil
}

func (p *PostgreSQL) DeleteURLMapping(ctx context.Context, instance string) (rowsAffected int64, err error) {
	const q = `DELETE FROM callbacker.url_mapping
			WHERE instance=$1`

	rows, err := p.db.ExecContext(ctx, q, instance)
	if err != nil {
		return 0, errors.Join(store.ErrURLMappingDeleteFailed, err)
	}

	rowsAffected, err = rows.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return rowsAffected, nil
}

func (p *PostgreSQL) DeleteURLMappingsExcept(ctx context.Context, except []string) (rowsAffected int64, err error) {
	const q = `DELETE FROM callbacker.url_mapping
			WHERE NOT instance = ANY($1::TEXT[])`

	param := "{" + strings.Join(except, ",") + "}"
	rows, err := p.db.ExecContext(ctx, q, param)
	if err != nil {
		return 0, errors.Join(store.ErrURLMappingsDeleteFailed, err)
	}

	rowsAffected, err = rows.RowsAffected()
	if err != nil {
		return 0, nil
	}

	return rowsAffected, nil
}

func (p *PostgreSQL) GetURLMappings(ctx context.Context) (map[string]string, error) {
	const q = `SELECT url, instance FROM callbacker.url_mapping`

	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urlMappings := map[string]string{}

	for rows.Next() {
		var url string
		var instance string

		err = rows.Scan(&url, &instance)
		if err != nil {
			return nil, err
		}

		urlMappings[url] = instance
	}

	return urlMappings, nil
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
