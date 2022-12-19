package sqlitestore

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	store2 "github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

const ISO8601 = "2006-01-02T15:04:05.999Z"

type SqliteStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// New returns a new initialized SqlLiteStore database implementing the Store
// interface. If the database cannot be initialized, an error will be returned.
func New(dbLocation string) (store2.Store, error) {
	if dbLocation == "" {
		dbLocation = "./sqlite_data.db"
	}
	sqliteDb, err := sql.Open("sqlite", dbLocation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open sqlite DB")
	}

	s := &SqliteStore{
		db: sqliteDb,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create schema, if necessary...
	q := `CREATE TABLE IF NOT EXISTS transactions (
		hash BLOCK PRIMARY KEY,
		stored_at TEXT,
		announced_at TEXT,
		mined_at TEXT,
		status INTEGER,
		block_height BIGINT,
		block_hash BLOB,
		api_key_id BIGINT,
		standard_fee_id BIGINT,
		data_fee_id BIGINT,
		source_ip TEXT,
		callback_url TEXT,
		callback_token TEXT,
		merkle_proof TEXT,
		reject_reason TEXT,
		raw_tx BLOB
	);`

	if _, err := sqliteDb.Exec(q); err != nil {
		return nil, errors.Wrap(err, "failed to create transactions table")
	}

	return s, err
}

// Get implements the Store interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (s *SqliteStore) Get(ctx context.Context, hash []byte) (*store2.StoreData, error) {
	q := `SELECT
	   stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,api_key_id
		,standard_fee_id
		,data_fee_id
		,source_ip
		,callback_url
		,callback_token
		,merkle_proof
		,reject_reason
		,raw_tx
	 	FROM transactions WHERE hash = $1 LIMIT 1;`

	data := &store2.StoreData{}

	var storedAt string
	var announcedAt string
	var minedAt string

	s.mu.RLock()
	defer s.mu.RUnlock()

	err := s.db.QueryRowContext(ctx, q, hash).Scan(
		&storedAt,
		&announcedAt,
		&minedAt,
		&data.Hash,
		&data.Status,
		&data.BlockHeight,
		&data.BlockHash,
		&data.ApiKeyId,
		&data.StandardFeeId,
		&data.DataFeeId,
		&data.SourceIp,
		&data.CallbackUrl,
		&data.CallbackToken,
		&data.MerkleProof,
		&data.RejectReason,
		&data.RawTx,
	)
	if err != nil {
		return nil, err
	}

	if storedAt != "" {
		data.StoredAt, err = time.Parse(ISO8601, storedAt)
		if err != nil {
			return nil, err
		}
	}

	if announcedAt != "" {
		data.AnnouncedAt, err = time.Parse(ISO8601, announcedAt)
		if err != nil {
			return nil, err
		}
	}
	if minedAt != "" {
		data.MinedAt, err = time.Parse(ISO8601, minedAt)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// Set implements the Store interface. It attempts to store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (s *SqliteStore) Set(ctx context.Context, hash []byte, value *store2.StoreData) error {
	// storedAt := time.Now().UTC().Format(ISO8601)

	q := `INSERT INTO transactions (
		 stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,api_key_id
		,standard_fee_id
		,data_fee_id
		,source_ip
		,callback_url
		,callback_token
		,merkle_proof
		,reject_reason
		,raw_tx
	) VALUES (
		 $1
		,$2
		,$3
		,$4
		,$5
		,$6
		,$7
		,$8
		,$9
		,$10
		,$11
		,$12
		,$13
		,$14
		,$15
		,$16
	);`

	var storedAt string
	var announcedAt string
	var minedAt string

	if value.StoredAt.UnixMilli() != 0 {
		storedAt = value.StoredAt.UTC().Format(ISO8601)
	}

	if value.AnnouncedAt.UnixMilli() != 0 {
		announcedAt = value.AnnouncedAt.UTC().Format(ISO8601)
	}

	if value.MinedAt.UnixMilli() != 0 {
		minedAt = value.MinedAt.UTC().Format(ISO8601)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, q,
		storedAt,
		announcedAt,
		minedAt,
		value.Hash,
		value.Status,
		value.BlockHeight,
		value.BlockHash,
		value.ApiKeyId,
		value.StandardFeeId,
		value.DataFeeId,
		value.SourceIp,
		value.CallbackUrl,
		value.CallbackToken,
		value.MerkleProof,
		value.RejectReason,
		value.RawTx,
	)

	return err

}

func (s *SqliteStore) GetUnseen(ctx context.Context, callback func(s *store2.StoreData)) error {
	q := `SELECT
	   stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,api_key_id
		,standard_fee_id
		,data_fee_id
		,source_ip
		,callback_url
		,callback_token
		,merkle_proof
		,raw_tx
	 	FROM transactions WHERE status < $1;`

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		data := &store2.StoreData{}

		var storedAt string
		var announcedAt string
		var minedAt string

		if err = rows.Scan(
			&storedAt,
			&announcedAt,
			&minedAt,
			&data.Hash,
			&data.Status,
			&data.BlockHeight,
			&data.BlockHash,
			&data.ApiKeyId,
			&data.StandardFeeId,
			&data.DataFeeId,
			&data.SourceIp,
			&data.CallbackUrl,
			&data.CallbackToken,
			&data.MerkleProof,
			&data.RawTx,
		); err != nil {
			return err
		}
		if storedAt != "" {
			data.StoredAt, err = time.Parse(ISO8601, storedAt)
			if err != nil {
				return err
			}
		}

		if announcedAt != "" {
			data.AnnouncedAt, err = time.Parse(ISO8601, announcedAt)
			if err != nil {
				return err
			}
		}
		if minedAt != "" {
			data.MinedAt, err = time.Parse(ISO8601, minedAt)
			if err != nil {
				return err
			}
		}

		callback(data)
	}

	return nil
}

func (s *SqliteStore) UpdateStatus(ctx context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error {
	q := `
		UPDATE transactions
		SET status = $1
			,reject_reason = $2
		WHERE hash = $3
	;`

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, q, status, rejectReason, hash)

	return err
}

func (s *SqliteStore) UpdateMined(ctx context.Context, hash []byte, blockHash []byte, blockHeight int32) error {
	q := `
		UPDATE transactions
		SET status = $1
			,block_hash = $2
			,block_height = $3
		WHERE hash = $4
	;`

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, q, metamorph_api.Status_MINED, blockHash, blockHeight, hash)

	return err
}

func (s *SqliteStore) Del(ctx context.Context, key []byte) error {
	hash := store2.HashString(key)

	q := `DELETE FROM transactions WHERE hash = $1;`

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, q, hash)

	return err

}

// Close implements the Store interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (s *SqliteStore) Close(ctx context.Context) error {
	ctx.Done()
	return s.db.Close()
}
