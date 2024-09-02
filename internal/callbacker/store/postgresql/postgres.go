package postgresql

import (
	"context"
	"database/sql"
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

// It closes the connection to the underlying database.
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
		} else {
			extraInfos[i] = sql.NullString{}
		}

		if d.MerklePath != nil {
			merklePaths[i] = sql.NullString{String: *d.MerklePath, Valid: true}
		} else {
			merklePaths[i] = sql.NullString{}
		}

		if d.BlockHash != nil {
			blockHashes[i] = sql.NullString{String: *d.BlockHash, Valid: true}
		} else {
			blockHashes[i] = sql.NullString{}
		}

		if d.BlockHeight != nil {
			blockHeights[i] = sql.NullInt64{Int64: int64(*d.BlockHeight), Valid: true}
		} else {
			blockHeights[i] = sql.NullInt64{}
		}

		if len(d.CompetingTxs) > 0 {
			competingTxs[i] = sql.NullString{String: strings.Join(d.CompetingTxs, ","), Valid: true}
		} else {
			competingTxs[i] = sql.NullString{}
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
					,UNNEST($10::TEXT[])
					
				ON CONFLICT DO NOTHING`

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

func (p *PostgreSQL) PopMany(ctx context.Context, limit int) ([]*store.CallbackData, error) {
	return nil, nil
}
