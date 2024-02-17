package sqlite

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"path"
	"path/filepath"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
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
		reject_reason TEXT,
		raw_tx BLOB,
		merkle_path TEXT,
		full_status_updates BOOLEAN DEFAULT(FALSE)
		);
	`); err != nil {
		_ = db.Close()
		return fmt.Errorf("could not create transactions table - [%+v]", err)
	}

	return nil
}

type SqLite struct {
	db  *sql.DB
	now func() time.Time
}

func (s *SqLite) SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error {
	return nil
}

func (s *SqLite) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
	return 0, nil
}

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
		value.RejectReason,
		value.RawTx,
	)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (s *SqLite) GetUnmined(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
	startNanos := time.Now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("getunmined").AddTime(startNanos)
	}()
	span, ctx := opentracing.StartSpanFromContext(ctx, "sql:GetUnmined")
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
		,raw_tx
		FROM transactions
		WHERE (status < $1 OR status = $2)
		LIMIT $3
		;`

	rows, err := s.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, limit)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}
	defer rows.Close()
	storeData := make([]*store.StoreData, 0)
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

		storeData = append(storeData, data)
	}

	return storeData, nil
}

func (s *SqLite) UpdateStatusBulk(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {

	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatusBulk").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatusBulk")
	defer span.Finish()

	q := `
		UPDATE transactions
		SET status = $1
			,reject_reason = $2
		WHERE hash = $3
		RETURNING stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,callback_url
		,callback_token
		,full_status_updates
		,reject_reason
		,raw_tx
		,merkle_path
		;`

	var storeData []*store.StoreData

	for _, update := range updates {

		data := &store.StoreData{}

		var storedAt string
		var announcedAt string
		var minedAt string
		var blockHeight sql.NullInt64
		var txHash []byte
		var blockHash []byte
		var callbackUrl sql.NullString
		var callbackToken sql.NullString
		var fullStatusUpdates sql.NullBool
		var rejectReason sql.NullString
		var lockedBy sql.NullString
		var status sql.NullInt32
		var merklePath sql.NullString

		err := s.db.QueryRowContext(ctx, q, update.Status, update.RejectReason, update.Hash[:]).Scan(
			&storedAt,
			&announcedAt,
			&minedAt,
			&txHash,
			&status,
			&blockHeight,
			&blockHash,
			&callbackUrl,
			&callbackToken,
			&fullStatusUpdates,
			&rejectReason,
			&data.RawTx,
			&merklePath,
		)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
		}

		if len(txHash) > 0 {
			data.Hash, err = chainhash.NewHash(txHash)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return nil, err
			}
		}

		if len(blockHash) > 0 {
			data.BlockHash, err = chainhash.NewHash(blockHash)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
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

		if status.Valid {
			data.Status = metamorph_api.Status(status.Int32)
		}

		if blockHeight.Valid {
			data.BlockHeight = uint64(blockHeight.Int64)
		}

		if callbackUrl.Valid {
			data.CallbackUrl = callbackUrl.String
		}

		if callbackToken.Valid {
			data.CallbackToken = callbackToken.String
		}

		if fullStatusUpdates.Valid {
			data.FullStatusUpdates = fullStatusUpdates.Bool
		}

		if rejectReason.Valid {
			data.RejectReason = rejectReason.String
		}

		if lockedBy.Valid {
			data.LockedBy = lockedBy.String
		}

		if merklePath.Valid {
			data.MerklePath = merklePath.String
		}

		storeData = append(storeData, data)

	}

	return storeData, nil
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

func (s *SqLite) UpdateMined(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
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
			,merkle_path = $4
		WHERE hash = $5
		RETURNING stored_at
		,announced_at
		,mined_at
		,hash
		,status
		,block_height
		,block_hash
		,callback_url
		,callback_token
		,full_status_updates
		,reject_reason
		,raw_tx
		,merkle_path
		;`

	var storeData []*store.StoreData

	for _, txBlock := range txsBlocks.TransactionBlocks {

		data := &store.StoreData{}

		var storedAt string
		var announcedAt string
		var minedAt string
		var blockHeight sql.NullInt64
		var txHash []byte
		var blockHash []byte
		var callbackUrl sql.NullString
		var callbackToken sql.NullString
		var fullStatusUpdates sql.NullBool
		var rejectReason sql.NullString
		var lockedBy sql.NullString
		var status sql.NullInt32
		var merklePath sql.NullString

		err := s.db.QueryRowContext(ctx, q, metamorph_api.Status_MINED, txBlock.BlockHash[:], txBlock.BlockHeight, txBlock.MerklePath, txBlock.TransactionHash[:]).Scan(
			&storedAt,
			&announcedAt,
			&minedAt,
			&txHash,
			&status,
			&blockHeight,
			&blockHash,
			&callbackUrl,
			&callbackToken,
			&fullStatusUpdates,
			&rejectReason,
			&data.RawTx,
			&merklePath,
		)
		if err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
		}

		if len(txHash) > 0 {
			data.Hash, err = chainhash.NewHash(txHash)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
				return nil, err
			}
		}

		if len(blockHash) > 0 {
			data.BlockHash, err = chainhash.NewHash(blockHash)
			if err != nil {
				span.SetTag(string(ext.Error), true)
				span.LogFields(log.Error(err))
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

		if status.Valid {
			data.Status = metamorph_api.Status(status.Int32)
		}

		if blockHeight.Valid {
			data.BlockHeight = uint64(blockHeight.Int64)
		}

		if callbackUrl.Valid {
			data.CallbackUrl = callbackUrl.String
		}

		if callbackToken.Valid {
			data.CallbackToken = callbackToken.String
		}

		if fullStatusUpdates.Valid {
			data.FullStatusUpdates = fullStatusUpdates.Bool
		}

		if rejectReason.Valid {
			data.RejectReason = rejectReason.String
		}

		if lockedBy.Valid {
			data.LockedBy = lockedBy.String
		}

		if merklePath.Valid {
			data.MerklePath = merklePath.String
		}

		storeData = append(storeData, data)

	}

	return storeData, nil
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

func (s *SqLite) Ping(ctx context.Context) error {
	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Ping").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Ping")
	defer span.Finish()

	_, err := s.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return nil
}

func (s *SqLite) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	startNanos := s.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("ClearData").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:ClearData")
	defer span.Finish()

	start := s.now()

	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(retentionDays))

	res, err := s.db.ExecContext(ctx, "DELETE FROM transactions WHERE stored_at <= $1", deleteBeforeDate)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}
