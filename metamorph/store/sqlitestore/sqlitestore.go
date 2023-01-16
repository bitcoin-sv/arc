package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	store2 "github.com/TAAL-GmbH/arc/metamorph/store"
	_ "github.com/lib/pq"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

const ISO8601 = "2006-01-02T15:04:05.999Z"

type SQL struct {
	db *sql.DB
}

// New returns a new initialized SqlLiteStore database implementing the Store
// interface. If the database cannot be initialized, an error will be returned.
func New(engine string) (store2.Store, error) {
	var db *sql.DB
	var err error

	var memory bool

	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log("mmsql", gocore.NewLogLevelFromString(logLevel))

	switch engine {
	case "postgres":
		dbHost, _ := gocore.Config().Get("dbHost", "localhost")
		dbPort, _ := gocore.Config().GetInt("dbPort", 5432)
		dbName, _ := gocore.Config().Get("dbName", "arc")
		dbUser, _ := gocore.Config().Get("dbUser", "arc")
		dbPassword, _ := gocore.Config().Get("dbPassword", "arc")

		dbInfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%d", dbUser, dbPassword, dbName, dbHost, dbPort)

		db, err = sql.Open(engine, dbInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
		}

		if err := createPostgresSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create postgres schema: %+v", err)
		}

	case "sqlite_memory":
		memory = true
		fallthrough
	case "sqlite":
		folder, _ := gocore.Config().Get("dataFolder", "data")

		filename, err := filepath.Abs(path.Join(folder, "metamorph.db"))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
		}

		if memory {
			filename = ":memory:"
		} else {
			// filename = fmt.Sprintf("file:%s?cache=shared&mode=rwc", filename)
			filename = fmt.Sprintf("%s?_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
		}

		logger.Infof("Using sqlite DB: %s", filename)

		db, err = sql.Open("sqlite", filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite DB: %+v", err)
		}

		if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("could not enable foreign keys support: %+v", err)
		}

		if _, err := db.Exec(`PRAGMA locking_mode = SHARED;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("could not enable shared locking mode: %+v", err)
		}

		if err := createSqliteSchema(db); err != nil {
			return nil, fmt.Errorf("failed to create sqlite schema: %+v", err)
		}

	default:
		return nil, fmt.Errorf("unknown database engine: %s", engine)
	}

	return &SQL{
		db: db,
	}, nil
}

func createPostgresSchema(db *sql.DB) error {
	return nil
}

func createSqliteSchema(db *sql.DB) error {
	// Create schema, if necessary...
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
		hash BLOB PRIMARY KEY,
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
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create block_transactions_map table - [%+v]", err)
	}

	return nil
}

// Get implements the Store interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (s *SQL) Get(ctx context.Context, hash []byte) (*store2.StoreData, error) {
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
		if err == sql.ErrNoRows {
			return nil, store2.ErrNotFound
		}
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
func (s *SQL) Set(ctx context.Context, hash []byte, value *store2.StoreData) error {
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

func (s *SQL) GetUnseen(ctx context.Context, callback func(s *store2.StoreData)) error {
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

func (s *SQL) UpdateStatus(ctx context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error {
	q := `
		UPDATE transactions
		SET status = $1
			,reject_reason = $2
		WHERE hash = $3
	;`

	_, err := s.db.ExecContext(ctx, q, status, rejectReason, hash)

	return err
}

func (s *SQL) UpdateMined(ctx context.Context, hash []byte, blockHash []byte, blockHeight int32) error {
	q := `
		UPDATE transactions
		SET status = $1
			,block_hash = $2
			,block_height = $3
		WHERE hash = $4
	;`

	_, err := s.db.ExecContext(ctx, q, metamorph_api.Status_MINED, blockHash, blockHeight, hash)

	return err
}

func (s *SQL) Del(ctx context.Context, key []byte) error {
	hash := store2.HashString(key)

	q := `DELETE FROM transactions WHERE hash = $1;`

	_, err := s.db.ExecContext(ctx, q, hash)

	return err

}

// Close implements the Store interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (s *SQL) Close(ctx context.Context) error {
	ctx.Done()
	return s.db.Close()
}
