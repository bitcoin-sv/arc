package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/lib/pq"
)

const (
	postgresDriverName = "postgres"
)

type PostgreSQL struct {
	db *sql.DB
}

func New(dbInfo string, idleConns int, maxOpenConns int) (*PostgreSQL, error) {
	db, err := sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
	}

	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)

	return &PostgreSQL{db: db}, nil
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
	extraInfos := make([]sql.NullString, len(data))
	merklePaths := make([]sql.NullString, len(data))
	blockHashes := make([]sql.NullString, len(data))
	blockHeights := make([]sql.NullInt64, len(data))
	competingTxs := make([]sql.NullString, len(data))

	for i, d := range data {
		urls[i] = d.Url
		tokens[i] = d.Token
		timestamps[i] = d.Timestamp
		txids[i] = d.TxID
		txStatuses[i] = d.TxStatus

		if d.ExtraInfo != nil {
			extraInfos[i] = sql.NullString{String: *d.ExtraInfo, Valid: true}
		}

		if d.MerklePath != nil {
			merklePaths[i] = sql.NullString{String: *d.MerklePath, Valid: true}
		}

		if d.BlockHash != nil {
			blockHashes[i] = sql.NullString{String: *d.BlockHash, Valid: true}
		}

		if d.BlockHeight != nil {
			blockHeights[i] = sql.NullInt64{Int64: int64(*d.BlockHeight), Valid: true}
		}

		if len(d.CompetingTxs) > 0 {
			competingTxs[i] = sql.NullString{String: strings.Join(d.CompetingTxs, ","), Valid: true}
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
					,UNNEST($10::TEXT[])`

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
	)

	return err
}

func (p *PostgreSQL) PopMany(ctx context.Context, limit int) (res []*store.CallbackData, err error) {
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
				,timestamp`

	rows, err := tx.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*store.CallbackData, 0, limit)

	for rows.Next() {
		r := &store.CallbackData{}

		var (
			ts      time.Time
			ei      sql.NullString
			mp      sql.NullString
			bh      sql.NullString
			bHeight sql.NullInt64
			ctxs    sql.NullString
		)

		err = rows.Scan(
			&r.Url,
			&r.Token,
			&r.TxID,
			&r.TxStatus,
			&ei,
			&mp,
			&bh,
			&bHeight,
			&ctxs,
			&ts,
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

		records = append(records, r)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return records, nil
}

func ptrTo[T any](v T) *T {
	return &v
}
