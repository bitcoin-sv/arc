package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/ordishs/gocore"
)

const (
	postgresDriverName      = "postgres"
	numericalDateHourLayout = "2006010215"
)

type PostgreSQL struct {
	db       *sql.DB
	hostname string
	now      func() time.Time
}

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.now = nowFunc
	}
}

func New(dbInfo string, hostname string, idleConns int, maxOpenConns int, opts ...func(postgreSQL *PostgreSQL)) (*PostgreSQL, error) {
	db, err := sql.Open(postgresDriverName, dbInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres DB: %+v", err)
	}

	db.SetMaxIdleConns(idleConns)
	db.SetMaxOpenConns(maxOpenConns)
	p := &PostgreSQL{
		db:       db,
		hostname: hostname,
		now:      time.Now,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func (p *PostgreSQL) SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("setunlocked").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:SetUnlocked")
	defer span.Finish()

	var hashSlice [][]byte
	for _, hash := range hashes {
		hashSlice = append(hashSlice, hash[:])
	}

	q := `UPDATE metamorph.transactions SET locked_by = 'NONE' WHERE hash = ANY($1);`

	_, err := p.db.ExecContext(ctx, q, pq.Array(hashSlice))
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("setunlockedbyname").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:GetUnmined")
	defer span.Finish()

	q := "UPDATE metamorph.transactions SET locked_by = 'NONE' WHERE locked_by = $1;"

	rows, err := p.db.ExecContext(ctx, q, lockedBy)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := rows.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// Get implements the MetamorphStore interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (p *PostgreSQL) Get(ctx context.Context, hash []byte) (*store.StoreData, error) {
	startNanos := p.now().UnixNano()
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
		,full_status_updates
		,reject_reason
		,raw_tx
		,locked_by
		,merkle_path
	 	FROM metamorph.transactions WHERE hash = $1 LIMIT 1;`

	data := &store.StoreData{}

	var storedAt sql.NullTime
	var announcedAt sql.NullTime
	var minedAt sql.NullTime
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

	err := p.db.QueryRowContext(ctx, q, hash).Scan(
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
		&lockedBy,
		&merklePath,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
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

	if storedAt.Valid {
		data.StoredAt = storedAt.Time.UTC()
	}

	if announcedAt.Valid {
		data.AnnouncedAt = announcedAt.Time.UTC()
	}

	if minedAt.Valid {
		data.MinedAt = minedAt.Time.UTC()
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

	return data, nil
}

// Set implements the MetamorphStore interface. It attempts to store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (p *PostgreSQL) Set(ctx context.Context, _ []byte, value *store.StoreData) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Set").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Set")
	defer span.Finish()

	q := `INSERT INTO metamorph.transactions (
		 stored_at
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
		,locked_by
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
	);`

	var txHash []byte
	var blockHash []byte

	if value.Hash != nil {
		txHash = value.Hash.CloneBytes()
	}

	if value.BlockHash != nil {
		blockHash = value.BlockHash.CloneBytes()
	}

	// If the storedAt time is zero, set it to now on insert
	if value.StoredAt.IsZero() {
		value.StoredAt = p.now()
	}

	_, err := p.db.ExecContext(ctx, q,
		value.StoredAt,
		value.AnnouncedAt,
		value.MinedAt,
		txHash,
		value.Status,
		value.BlockHeight,
		blockHash,
		value.CallbackUrl,
		value.CallbackToken,
		value.FullStatusUpdates,
		value.RejectReason,
		value.RawTx,
		p.hostname,
	)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (p *PostgreSQL) setLockedBy(ctx context.Context, hash *chainhash.Hash, lockedByValue string) error {
	q := `
		UPDATE metamorph.transactions
		SET locked_by = $1
		WHERE hash = $2
	;`

	_, err := p.db.ExecContext(ctx, q, lockedByValue, hash[:])
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) GetUnmined(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
	startNanos := p.now().UnixNano()
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
		,full_status_updates
		,raw_tx
		,locked_by
		FROM metamorph.transactions
		WHERE locked_by = 'NONE'
		AND (status < $1 OR status = $2)
		AND inserted_at_num > $3
		ORDER BY inserted_at_num DESC
		LIMIT $4;`

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, since.Format(numericalDateHourLayout), limit)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
		return nil, err
	}
	defer rows.Close()

	unminedTxs := make([]*store.StoreData, 0)
	for rows.Next() {
		data := &store.StoreData{}

		var storedAt sql.NullTime
		var announcedAt sql.NullTime
		var minedAt sql.NullTime
		var blockHeight sql.NullInt64
		var txHash []byte
		var blockHash []byte
		var callbackUrl sql.NullString
		var callbackToken sql.NullString
		var fullStatusUpdates sql.NullBool
		var lockedBy sql.NullString
		var status sql.NullInt32

		if err = rows.Scan(
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
			&data.RawTx,
			&lockedBy,
		); err != nil {
			span.SetTag(string(ext.Error), true)
			span.LogFields(log.Error(err))
			return nil, err
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

		if storedAt.Valid {
			data.StoredAt = storedAt.Time.UTC()
		}

		if announcedAt.Valid {
			data.AnnouncedAt = announcedAt.Time.UTC()
		}

		if minedAt.Valid {
			data.MinedAt = minedAt.Time.UTC()
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

		if lockedBy.Valid {
			data.LockedBy = lockedBy.String
		}

		err = p.setLockedBy(ctx, data.Hash, p.hostname)
		if err != nil {
			return nil, err
		}

		unminedTxs = append(unminedTxs, data)
	}

	return unminedTxs, nil
}

func (p *PostgreSQL) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateStatus").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateStatus")
	defer span.Finish()

	// do not store other statuses than the following
	if status != metamorph_api.Status_REJECTED &&
		status != metamorph_api.Status_SEEN_ON_NETWORK &&
		status != metamorph_api.Status_MINED &&
		status != metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
		return nil
	}

	args := []any{status, rejectReason, hash[:]}
	q := `
		UPDATE metamorph.transactions
		SET status = $1,
		    reject_reason = $2
		WHERE hash = $3
	;`

	result, err := p.db.ExecContext(ctx, q, args...)
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

func (p *PostgreSQL) UpdateMined(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateMined")
	defer span.Finish()

	txHashes := make([][]byte, len(txsBlocks.TransactionBlocks))
	blockHashes := make([][]byte, len(txsBlocks.TransactionBlocks))
	blockHeights := make([]uint64, len(txsBlocks.TransactionBlocks))
	merklePaths := make([]string, len(txsBlocks.TransactionBlocks))
	for i, tx := range txsBlocks.TransactionBlocks {
		txHashes[i] = tx.TransactionHash
		blockHashes[i] = tx.BlockHash
		blockHeights[i] = tx.BlockHeight
		merklePaths[i] = tx.MerklePath
	}

	qBulkUpdate := `
		UPDATE metamorph.transactions t
			SET
			    status=$1,
			    mined_at=$2,
			    block_hash=bulk_query.block_hash,
			    block_height=bulk_query.block_height,
			  	merkle_path=bulk_query.merkle_path
			FROM
			  (
				SELECT *
				FROM
				  UNNEST($3::BYTEA[], $4::BYTEA[], $5::BIGINT[], $6::TEXT[])
				  AS t(hash, block_hash, block_height, merkle_path)
			  ) AS bulk_query
			WHERE
			  t.hash=bulk_query.hash
		RETURNING t.stored_at
		,t.announced_at
		,t.mined_at
		,t.hash
		,t.status
		,t.block_height
		,t.block_hash
		,t.callback_url
		,t.callback_token
		,t.full_status_updates
		,t.reject_reason
		,t.raw_tx
		,t.locked_by
		,t.merkle_path
		;
`
	rows, err := p.db.QueryContext(ctx, qBulkUpdate, metamorph_api.Status_MINED, p.now(), pq.Array(txHashes), pq.Array(blockHashes), pq.Array(blockHeights), pq.Array(merklePaths))
	if err != nil {
		return nil, fmt.Errorf("failed to execute transaction update query: %v", err)
	}

	var storeData []*store.StoreData

	for rows.Next() {

		data := &store.StoreData{}

		var storedAt sql.NullTime
		var announcedAt sql.NullTime
		var minedAt sql.NullTime
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

		err = rows.Scan(
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
			&lockedBy,
			&merklePath,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, store.ErrNotFound
			}
			return nil, err
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

		if storedAt.Valid {
			data.StoredAt = storedAt.Time.UTC()
		}

		if announcedAt.Valid {
			data.AnnouncedAt = announcedAt.Time.UTC()
		}

		if minedAt.Valid {
			data.MinedAt = minedAt.Time.UTC()
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

func (p *PostgreSQL) Del(ctx context.Context, key []byte) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Del").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Del")
	defer span.Finish()

	q := `DELETE FROM metamorph.transactions WHERE hash = $1;`

	result, err := p.db.ExecContext(ctx, q, key)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	rows, _ := result.RowsAffected()
	fmt.Println(rows)
	return err
}

// Close implements the MetamorphStore interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (p *PostgreSQL) Close(ctx context.Context) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Close").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Close")
	defer span.Finish()

	ctx.Done()
	return p.db.Close()
}

func (p *PostgreSQL) Ping(ctx context.Context) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("Ping").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:Ping")
	defer span.Finish()

	_, err := p.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("ClearData").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:ClearData")
	defer span.Finish()

	start := p.now()

	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(retentionDays))

	res, err := p.db.ExecContext(ctx, "DELETE FROM metamorph.transactions WHERE inserted_at_num <= $1::int", deleteBeforeDate.Format(numericalDateHourLayout))
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}
