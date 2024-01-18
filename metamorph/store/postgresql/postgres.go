package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
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

func (p *PostgreSQL) SetUnlockedByName(ctx context.Context, lockedBy string) (int, error) {
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

	return int(rowsAffected), nil
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
		,merkle_proof
		,reject_reason
		,raw_tx
		,locked_by
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
	var merkleProof sql.NullBool
	var rejectReason sql.NullString
	var lockedBy sql.NullString
	var status sql.NullInt32

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
		&merkleProof,
		&rejectReason,
		&data.RawTx,
		&lockedBy,
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

	if merkleProof.Valid {
		data.MerkleProof = merkleProof.Bool
	}

	if rejectReason.Valid {
		data.RejectReason = rejectReason.String
	}

	if lockedBy.Valid {
		data.LockedBy = lockedBy.String
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
		,merkle_proof
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
		value.MerkleProof,
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
		,merkle_proof
		,raw_tx
		,locked_by
		FROM metamorph.transactions
		WHERE locked_by = 'NONE'
		AND (status < $1 OR status = $2)
		AND inserted_at_num > $3
		ORDER BY inserted_at_num DESC
		LIMIT $4;`

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_MINED, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, since.Format(numericalDateHourLayout), limit)
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
		var merkleProof sql.NullBool
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
			&merkleProof,
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

		if merkleProof.Valid {
			data.MerkleProof = merkleProof.Bool
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

func (p *PostgreSQL) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("UpdateMined").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:UpdateMined")
	defer span.Finish()

	var blockHashBytes []byte
	if blockHash == nil {
		blockHashBytes = nil
	} else {
		blockHashBytes = blockHash[:]
	}

	q := `
		UPDATE metamorph.transactions
		SET status = $1
			,block_hash = $2
			,block_height = $3
			,mined_at = $4
		WHERE hash = $5
	;`

	_, err := p.db.ExecContext(ctx, q, metamorph_api.Status_MINED, blockHashBytes, blockHeight, p.now().Format(time.RFC3339), hash[:])
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (p *PostgreSQL) RemoveCallbacker(ctx context.Context, hash *chainhash.Hash) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("RemoveCallbacker").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:RemoveCallbacker")
	defer span.Finish()

	q := `UPDATE metamorph.transactions SET callback_url = '' WHERE hash = $1;`

	_, err := p.db.ExecContext(ctx, q, hash[:])
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
}

func (p *PostgreSQL) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("GetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:GetBlockProcessed")
	defer span.Finish()

	q := `SELECT
		processed_at
	 	FROM metamorph.blocks WHERE hash = $1 LIMIT 1;`

	var processedAt string

	err := p.db.QueryRowContext(ctx, q, blockHash[:]).Scan(
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

func (p *PostgreSQL) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	startNanos := p.now().UnixNano()
	defer func() {
		gocore.NewStat("mtm_store_sql").NewStat("SetBlockProcessed").AddTime(startNanos)
	}()
	span, _ := opentracing.StartSpanFromContext(ctx, "sql:SetBlockProcessed")
	defer span.Finish()

	q := `INSERT INTO metamorph.blocks (
		hash
		,processed_at
	) VALUES (
		 $1
		,$2
	);`

	processedAt := p.now().UTC().Format(time.RFC3339)

	_, err := p.db.ExecContext(ctx, q,
		blockHash[:],
		processedAt,
	)
	if err != nil {
		span.SetTag(string(ext.Error), true)
		span.LogFields(log.Error(err))
	}

	return err
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

func (p *PostgreSQL) ClearData(ctx context.Context, retentionDays int32) (*metamorph_api.ClearDataResponse, error) {
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
		return nil, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	return &metamorph_api.ClearDataResponse{Rows: rows}, nil
}
