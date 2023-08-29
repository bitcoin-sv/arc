package sql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/labstack/gommon/random"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

const ISO8601 = "2006-01-02T15:04:05.999Z"

type SQL struct {
	db *sql.DB
}

// New returns a new initialized SqlLiteStore database implementing the MetamorphStore
// interface. If the database cannot be initialized, an error will be returned.
func New(engine string) (store.MetamorphStore, error) {
	var db *sql.DB
	var err error

	var memory bool

	logLevel := viper.GetString("logLevel")
	logger := gocore.Log("mmsql", gocore.NewLogLevelFromString(logLevel))

	switch engine {
	case "postgres":
		dbHost := viper.GetString("metamorph_dbHost")
		if dbHost == "" {
			return nil, fmt.Errorf("metamorph_dbHost not found in config")
		}
		dbPort := viper.GetInt("metamorph_dbPort")
		dbName := viper.GetString("metamorph_dbName")
		dbUser := viper.GetString("metamorph_dbUser")
		dbPassword := viper.GetString("metamorph_dbPassword")

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
		var filename string
		if memory {
			filename = fmt.Sprintf("file:%s?mode=memory&cache=shared", random.String(16))
		} else {
			folder := viper.GetString("dataFolder")
			if folder == "" {
				return nil, fmt.Errorf("dataFolder not found in config")
			}
			if err := os.MkdirAll(folder, 0755); err != nil {
				return nil, fmt.Errorf("failed to create data folder %s: %+v", folder, err)
			}

			filename, err = filepath.Abs(path.Join(folder, "metamorph.db"))
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
			}
			filename = fmt.Sprintf("%s?cache=shared&_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
		}

		logger.Infof("Using sqlite DB: %s", filename)

		db, err = sql.Open("sqlite", filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite DB: %+v", err)
		}

		if _, err = db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("could not enable foreign keys support: %+v", err)
		}

		if _, err = db.Exec(`PRAGMA locking_mode = SHARED;`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("could not enable shared locking mode: %+v", err)
		}

		if err = createSqliteSchema(db); err != nil {
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
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("createPostgresSchema").NewStat("duration").AddTime(startNanos)
	}()

	// Create schema, if necessary...
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
		hash BYTEA PRIMARY KEY,
		stored_at TEXT,
		announced_at TEXT,
		mined_at TEXT,
		status INTEGER,
		block_height BIGINT,
		block_hash BYTEA,
		callback_url TEXT,
		callback_token TEXT,
		merkle_proof TEXT,
		reject_reason TEXT,
		raw_tx BYTEA
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	// Create schema, if necessary...
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		hash BYTEA PRIMARY KEY,
		processed_at TEXT
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create blocks table - [%+v]", err)
	}

	return nil
}

func createSqliteSchema(db *sql.DB) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("createSqliteSchema").AddTime(startNanos)
	}()

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
		callback_url TEXT,
		callback_token TEXT,
		merkle_proof TEXT,
		reject_reason TEXT,
		raw_tx BLOB
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	// Create schema for processed blocks, if necessary...
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		hash BLOB PRIMARY KEY,
		processed_at TEXT
		);
	`); err != nil {
		db.Close()
		return fmt.Errorf("could not create blocks table - [%+v]", err)
	}

	return nil
}

// Get implements the MetamorphStore interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (s *SQL) Get(ctx context.Context, hash []byte) (*store.StoreData, error) {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Get").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Get")
	defer span.Finish()

	q := `SELECT
	   stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,callback_url
		,callback_token
		,merkle_proof
		,reject_reason
		,raw_tx
	 	FROM transactions WHERE hash = $1 LIMIT 1;`

	data := &store.StoreData{}

	var storedAt string
	var announcedAt string
	var minedAt string
	var txHash []byte
	var blockHash []byte

	err := s.db.QueryRowContext(ctx, q, hash).Scan(
		&storedAt,
		&announcedAt,
		&minedAt,
		&txHash,
		&data.Status,
		&data.BlockHeight,
		&blockHash,
		&data.CallbackUrl,
		&data.CallbackToken,
		&data.MerkleProof,
		&data.RejectReason,
		&data.RawTx,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	if txHash != nil {
		data.Hash, err = chainhash.NewHash(txHash)
		if err != nil {
			return nil, err
		}
	}

	if blockHash != nil {
		data.BlockHash, err = chainhash.NewHash(blockHash)
		if err != nil {
			return nil, err
		}
	}

	if storedAt != "" {
		data.StoredAt, err = time.Parse(ISO8601, storedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	if announcedAt != "" {
		data.AnnouncedAt, err = time.Parse(ISO8601, announcedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}
	if minedAt != "" {
		data.MinedAt, err = time.Parse(ISO8601, minedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	return data, nil
}

// Set implements the MetamorphStore interface. It attempts to store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (s *SQL) Set(ctx context.Context, _ []byte, value *store.StoreData) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Set").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Set")
	defer span.Finish()

	q := `INSERT INTO transactions (
		 stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
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
	);`

	var storedAt string
	var announcedAt string
	var minedAt string
	var txHash []byte
	var blockHash []byte

	if value.Hash != nil {
		txHash = value.Hash.CloneBytes()
	}

	if value.BlockHash != nil {
		blockHash = value.BlockHash.CloneBytes()
	}

	if value.StoredAt.UnixMilli() != 0 {
		storedAt = value.StoredAt.UTC().Format(ISO8601)
	}

	// If the storedAt time is zero, set it to now on insert
	if value.StoredAt.IsZero() {
		value.StoredAt = time.Now()
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
		txHash,
		value.Status,
		value.BlockHeight,
		blockHash,
		value.CallbackUrl,
		value.CallbackToken,
		value.MerkleProof,
		value.RejectReason,
		value.RawTx,
	)

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (s *SQL) GetUnmined(ctx context.Context, callback func(s *store.StoreData)) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("getunmined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:GetUnmined")
	defer span.Finish()

	q := `SELECT
	   stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,callback_url
		,callback_token
		,merkle_proof
		,raw_tx
	 	FROM transactions WHERE status < $1;`

	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_MINED)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}
	defer rows.Close()

	for rows.Next() {
		data := &store.StoreData{}

		var txHash []byte
		var blockHash []byte
		var storedAt string
		var announcedAt string
		var minedAt string

		if err = rows.Scan(
			&storedAt,
			&announcedAt,
			&minedAt,
			&txHash,
			&data.Status,
			&data.BlockHeight,
			&blockHash,
			&data.CallbackUrl,
			&data.CallbackToken,
			&data.MerkleProof,
			&data.RawTx,
		); err != nil {
			return err
		}

		if txHash != nil {
			data.Hash, _ = chainhash.NewHash(txHash)
		}

		if blockHash != nil {
			data.BlockHash, _ = chainhash.NewHash(blockHash)
		}

		if storedAt != "" {
			data.StoredAt, err = time.Parse(ISO8601, storedAt)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return err
			}
		}

		if announcedAt != "" {
			data.AnnouncedAt, err = time.Parse(ISO8601, announcedAt)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return err
			}
		}
		if minedAt != "" {
			data.MinedAt, err = time.Parse(ISO8601, minedAt)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return err
			}
		}

		callback(data)
	}

	return nil
}

func (s *SQL) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	q := `
		UPDATE transactions
		SET status = $1
			,reject_reason = $2
		WHERE hash = $3
	;`

	result, err := s.db.ExecContext(ctx, q, status, rejectReason, hash[:])
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}

	var n int64
	n, err = result.RowsAffected()
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}

	return nil
}

func (s *SQL) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateMined")
	defer span.Finish()

	q := `
		UPDATE transactions
		SET status = $1
			,block_hash = $2
			,block_height = $3
		WHERE hash = $4
	;`

	_, err := s.db.ExecContext(ctx, q, metamorph_api.Status_MINED, blockHash[:], blockHeight, hash[:])

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (s *SQL) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("GetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:GetBlockProcessed")
	defer span.Finish()

	q := `SELECT
		processed_at
	 	FROM blocks WHERE hash = $1 LIMIT 1;`

	var processedAt string

	err := s.db.QueryRowContext(ctx, q, blockHash[:]).Scan(
		&processedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	var processedAtTime time.Time
	if processedAt != "" {
		processedAtTime, err = time.Parse(ISO8601, processedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	return &processedAtTime, nil
}

func (s *SQL) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("SetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:SetBlockProcessed")
	defer span.Finish()

	q := `INSERT INTO blocks (
		hash
		,processed_at
	) VALUES (
		 $1
		,$2
	);`

	processedAt := time.Now().UTC().Format(ISO8601)

	_, err := s.db.ExecContext(ctx, q,
		blockHash[:],
		processedAt,
	)

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (s *SQL) Del(ctx context.Context, key []byte) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Del").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Del")
	defer span.Finish()

	hash := store.HashString(key)

	q := `DELETE FROM transactions WHERE hash = $1;`

	_, err := s.db.ExecContext(ctx, q, hash)

	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err

}

// Close implements the MetamorphStore interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (s *SQL) Close(ctx context.Context) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Close").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Close")
	defer span.Finish()

	ctx.Done()
	return s.db.Close()
}
