package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

const (
	postgresDriverName = "postgres"
)

var (
	ErrFailedToOpenDB   = errors.New("failed to open postgres DB")
	ErrFailedToRollback = errors.New("failed to rollback")
)

type PostgreSQL struct {
	db *sql.DB
}

func New(dbInfo string, idleConns int, maxOpenConns int) (*PostgreSQL, error) {
	db, err := sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, errors.Join(ErrFailedToOpenDB, err)
	}

	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)

	return &PostgreSQL{db: db}, nil
}

func (p *PostgreSQL) Close() error {
	return p.db.Close()
}

func (p *PostgreSQL) Set(ctx context.Context, dto *store.CallbackData) error {
	return p.SetMany(ctx, []*store.CallbackData{dto})
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
	delayUntils := make([]sql.NullTime, len(data))
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
			blockHeights[i] = sql.NullInt64{Int64: int64(*d.BlockHeight), Valid: true}
		}

		if len(d.CompetingTxs) > 0 {
			competingTxs[i] = ptrTo(strings.Join(d.CompetingTxs, ","))
		}

		if d.PostponedUntil != nil {
			delayUntils[i] = sql.NullTime{Time: d.PostponedUntil.UTC(), Valid: true}
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
				,postponed_until
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
					,UNNEST($11::TIMESTAMPTZ[])
					,UNNEST($12::BOOLEAN[])`

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
		pq.Array(delayUntils),
		pq.Array(allowBatches),
	)

	return err
}

func (p *PostgreSQL) PopMany(ctx context.Context, limit int) ([]*store.CallbackData, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				err = errors.Join(ErrFailedToRollback, err, rErr)
			}
		}
	}()

	const q = `DELETE FROM callbacker.callbacks
			WHERE id IN (
				SELECT id FROM callbacker.callbacks
				WHERE postponed_until IS NULL
				ORDER BY id
				LIMIT $1
				FOR UPDATE
			)
			RETURNING
				url
				,token
				,tx_id
				,tx_status
				,extra_info
				,merkle_path
				,block_hash
				,block_height
				,competing_txs
				,timestamp
				,postponed_until
				,allow_batch`

	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*store.CallbackData
	records, err = scanCallbacks(rows, limit)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return records, nil
}

func (p *PostgreSQL) PopFailedMany(ctx context.Context, t time.Time, limit int) ([]*store.CallbackData, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to rollback: %v", rErr))
			}
		}
	}()

	const q = `DELETE FROM callbacker.callbacks
			WHERE id IN (
				SELECT id FROM callbacker.callbacks
				WHERE postponed_until IS NOT NULL AND postponed_until<= $1
				ORDER BY id
				LIMIT $2
				FOR UPDATE
			)
			RETURNING
				url
				,token
				,tx_id
				,tx_status
				,extra_info
				,merkle_path
				,block_hash
				,block_height
				,competing_txs
				,timestamp
				,postponed_until
				,allow_batch`

	rows, err := tx.QueryContext(ctx, q, t, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*store.CallbackData
	records, err = scanCallbacks(rows, limit)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return records, nil
}

func (p *PostgreSQL) DeleteFailedOlderThan(ctx context.Context, t time.Time) error {
	const q = `DELETE FROM callbacker.callbacks
			WHERE postponed_until IS NOT NULL AND timestamp <= $1`

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

func (p *PostgreSQL) DeleteURLMapping(ctx context.Context, instance string) error {
	const q = `DELETE FROM callbacker.url_mapping
			WHERE instance=$1`

	_, err := p.db.ExecContext(ctx, q, instance)
	if err != nil {
		return store.ErrURLMappingDeleteFailed
	}

	return nil
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
			ts             time.Time
			ei             sql.NullString
			mp             sql.NullString
			bh             sql.NullString
			bHeight        sql.NullInt64
			ctxs           sql.NullString
			postponedUntil sql.NullTime
		)

		err := rows.Scan(
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
			&postponedUntil,
			&r.AllowBatch,
		)

		if err != nil {
			return nil, err
		}

		r.Timestamp = ts.UTC()

		if ei.Valid {
			r.ExtraInfo = ptrTo(ei.String)
		}
		if mp.Valid {
			r.MerklePath = ptrTo(mp.String)
		}
		if bh.Valid {
			r.BlockHash = ptrTo(bh.String)
		}
		if bHeight.Valid {
			r.BlockHeight = ptrTo(uint64(bHeight.Int64))
		}
		if ctxs.String != "" {
			r.CompetingTxs = strings.Split(ctxs.String, ",")
		}
		if postponedUntil.Valid {
			r.PostponedUntil = ptrTo(postponedUntil.Time.UTC())
		}

		records = append(records, r)
	}

	return records, nil
}

func ptrTo[T any](v T) *T {
	return &v
}
