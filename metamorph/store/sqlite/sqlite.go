package sqlite

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/labstack/gommon/random"
	_ "github.com/lib/pq"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	_ "modernc.org/sqlite"
)

func WithNow(nowFunc func() time.Time) func(*SqLite) {
	return func(s *SqLite) {
		s.now = nowFunc
	}
}

// New returns a new initialized SqlLiteStore database implementing the MetamorphStore
// interface. If the database cannot be initialized, an error will be returned.
func New(memory bool, folder string, opts ...func(postgreSQL *SqLite)) (store.MetamorphStore, error) {
	var err error
	var filename string
	if memory {
		filename = fmt.Sprintf("file:%s?mode=memory&cache=shared", random.String(16))
	} else {
		filename, err = filepath.Abs(path.Join(folder, "metamorph.db"))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for sqlite DB: %+v", err)
		}
		filename = fmt.Sprintf("%s?cache=shared&_pragma=busy_timeout=10000&_pragma=journal_mode=WAL", filename)
	}

	db, err := sql.Open("sqlite", filename)
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

	if err = CreateSqliteSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create sqlite schema: %+v", err)
	}
	s := &SqLite{
		db:  db,
		now: time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func CreateSqliteSchema(db *sql.DB) error {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("CreateSqliteSchema").AddTime(startNanos)
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
		_ = db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	// Create schema for processed blocks, if necessary...
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS blocks (
		hash BLOB PRIMARY KEY,
		processed_at TEXT
		);
	`); err != nil {
		_ = db.Close()
		return fmt.Errorf("could not create blocks table - [%+v]", err)
	}

	return nil
}

type SqLite struct {
	db  *sql.DB
	now func() time.Time
}

func (s *SqLite) IsCentralised() bool {
	return false
}

func (s *SqLite) RemoveCallbacker(ctx context.Context, hash *chainhash.Hash) error {
	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RemoveCallbacker").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RemoveCallbacker")
	defer span.Finish()

	q := `UPDATE transactions SET status = callback_url = '' WHERE hash = $3;`

	result, err := s.db.ExecContext(ctx, q, hash[:])
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

func (s *SqLite) SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error {
	return nil
}

func (s *SqLite) SetUnlockedByName(ctx context.Context, lockedBy string) (int, error) { return 0, nil }

// Get implements the MetamorphStore interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (s *SqLite) Get(ctx context.Context, hash []byte) (*store.StoreData, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
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
		data.StoredAt, err = time.Parse(time.RFC3339, storedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	if announcedAt != "" {
		data.AnnouncedAt, err = time.Parse(time.RFC3339, announcedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}
	if minedAt != "" {
		data.MinedAt, err = time.Parse(time.RFC3339, minedAt)
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
func (s *SqLite) Set(ctx context.Context, _ []byte, value *store.StoreData) error {
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
		storedAt = value.StoredAt.UTC().Format(time.RFC3339)
	}

	// If the storedAt time is zero, set it to now on insert
	if value.StoredAt.IsZero() {
		value.StoredAt = time.Now()
	}

	if value.AnnouncedAt.UnixMilli() != 0 {
		announcedAt = value.AnnouncedAt.UTC().Format(time.RFC3339)
	}

	if value.MinedAt.UnixMilli() != 0 {
		minedAt = value.MinedAt.UTC().Format(time.RFC3339)
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

func (s *SqLite) GetUnminedTransactions(ctx context.Context) ([]store.StoreData, error) {
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
		FROM transactions WHERE status < $1 OR status = $2 ;`

	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_MINED, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []store.StoreData
	for rows.Next() {
		data := store.StoreData{}

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
			return nil, err
		}

		if txHash != nil {
			data.Hash, _ = chainhash.NewHash(txHash)
		}

		if blockHash != nil {
			data.BlockHash, _ = chainhash.NewHash(blockHash)
		}

		if storedAt != "" {
			data.StoredAt, err = time.Parse(time.RFC3339, storedAt)
			if err != nil {
				return nil, err
			}
		}

		if announcedAt != "" {
			data.AnnouncedAt, err = time.Parse(time.RFC3339, announcedAt)
			if err != nil {
				return nil, err
			}
		}
		if minedAt != "" {
			data.MinedAt, err = time.Parse(time.RFC3339, minedAt)
			if err != nil {
				return nil, err
			}
		}
		txs = append(txs, data)
	}

	return txs, nil
}

func (s *SqLite) GetUnmined(ctx context.Context, callback func(s *store.StoreData)) error {
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
		FROM transactions WHERE status < $1 OR status = $2 ;`

	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_MINED, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL)
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
			data.StoredAt, err = time.Parse(time.RFC3339, storedAt)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return err
			}
		}

		if announcedAt != "" {
			data.AnnouncedAt, err = time.Parse(time.RFC3339, announcedAt)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return err
			}
		}
		if minedAt != "" {
			data.MinedAt, err = time.Parse(time.RFC3339, minedAt)
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

func (s *SqLite) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	startNanos := s.now().UnixNano()
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

func (s *SqLite) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	startNanos := s.now().UnixNano()
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

func (s *SqLite) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	startNanos := s.now().UnixNano()
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}

	var processedAtTime time.Time
	if processedAt != "" {
		processedAtTime, err = time.Parse(time.RFC3339, processedAt)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
		}
	}

	return &processedAtTime, nil
}

func (s *SqLite) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	startNanos := s.now().UnixNano()
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

	processedAt := s.now().UTC().Format(time.RFC3339)

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

func (s *SqLite) Del(ctx context.Context, key []byte) error {
	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Del").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Del")
	defer span.Finish()

	hash := hex.EncodeToString(bt.ReverseBytes(utils.Sha256d(key)))

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
func (s *SqLite) Close(ctx context.Context) error {
	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Close").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Close")
	defer span.Finish()

	ctx.Done()
	return s.db.Close()
}
