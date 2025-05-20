package postgresql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ccoveille/go-safecast"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	postgresDriverName = "postgres"
	failedRollback     = "failed to rollback: %v"
)

type PostgreSQL struct {
	db                *sql.DB
	hostname          string
	now               func() time.Time
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithNow(nowFunc func() time.Time) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.now = nowFunc
	}
}

func WithTracing(attr []attribute.KeyValue) func(*PostgreSQL) {
	return func(p *PostgreSQL) {
		p.tracingEnabled = true
		p.tracingAttributes = attr
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

func (p *PostgreSQL) SetUnlockedByNameExcept(ctx context.Context, except []string) (int64, error) {
	q := "UPDATE metamorph.transactions SET locked_by = 'NONE' WHERE NOT locked_by = ANY($1::TEXT[]);"

	// remove empty strings
	var exceptUnlocked []string
	for _, ex := range except {
		if ex != "" {
			exceptUnlocked = append(exceptUnlocked, ex)
		}
	}

	// do not update entries which are already locked by NONE
	param := "{" + strings.Join(append(exceptUnlocked, "NONE"), ",") + "}"

	rows, err := p.db.ExecContext(ctx, q, param)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := rows.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (p *PostgreSQL) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
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
func (p *PostgreSQL) Get(ctx context.Context, hash []byte) (data *store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "Get", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `SELECT
	    stored_at
		,last_submitted_at
		,status
		,block_height
		,block_hash
		,callbacks
		,full_status_updates
		,reject_reason
		,competing_txs
		,raw_tx
		,locked_by
		,merkle_path
		,retries
		,status_history
		,last_modified
	 	FROM metamorph.transactions WHERE hash = $1 LIMIT 1;`

	var storedAt time.Time
	var lastSubmittedAt time.Time
	var status sql.NullInt32
	var blockHeight sql.NullInt64
	var blockHash []byte
	var callbacksData []byte
	var fullStatusUpdates bool
	var rejectReason sql.NullString
	var competingTxs sql.NullString
	var rawTx []byte
	var lockedBy string
	var merklePath sql.NullString
	var retries sql.NullInt32
	var statusHistory []byte
	var lastModified sql.NullTime

	err = p.db.QueryRowContext(ctx, q, hash).Scan(
		&storedAt,
		&lastSubmittedAt,
		&status,
		&blockHeight,
		&blockHash,
		&callbacksData,
		&fullStatusUpdates,
		&rejectReason,
		&competingTxs,
		&rawTx,
		&lockedBy,
		&merklePath,
		&retries,
		&statusHistory,
		&lastModified,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	data = &store.Data{}
	data.Hash, err = chainhash.NewHash(hash)
	if err != nil {
		return nil, err
	}

	if len(blockHash) > 0 {
		data.BlockHash, err = chainhash.NewHash(blockHash)
		if err != nil {
			return nil, err
		}
	}

	data.RawTx = rawTx

	data.StoredAt = storedAt.UTC()

	data.LastSubmittedAt = lastSubmittedAt.UTC()

	if lastModified.Valid {
		data.LastModified = lastModified.Time.UTC()
	}

	if status.Valid {
		data.Status = metamorph_api.Status(status.Int32)
	}

	blockHeightUint64, err := safecast.ToUint64(blockHeight.Int64)
	if err != nil {
		return nil, err
	}
	if blockHeight.Valid {
		data.BlockHeight = blockHeightUint64
	}

	if len(callbacksData) > 0 {
		callbacks, err := readCallbacksFromDB(callbacksData)
		if err != nil {
			return nil, err
		}
		data.Callbacks = callbacks
	}

	if len(statusHistory) > 0 {
		statuses, err := readStatusHistoryFromDB(statusHistory)
		if err != nil {
			return nil, err
		}
		data.StatusHistory = statuses
	}

	if retries.Valid {
		data.Retries = int(retries.Int32)
	}

	if competingTxs.String != "" {
		data.CompetingTxs = strings.Split(competingTxs.String, ",")
	}

	data.FullStatusUpdates = fullStatusUpdates
	data.RejectReason = rejectReason.String
	data.LockedBy = lockedBy
	data.MerklePath = merklePath.String

	return data, nil
}

// GetRawTxs implements the MetamorphStore interface. It attempts to get rawTxs for given hashes.
// If the hashes do not exist an empty array is returned, otherwise the retrieved values.
// If an error happens during the process of getting the results, the error is returned
// along with already found rawTxs up to the error point.
func (p *PostgreSQL) GetRawTxs(ctx context.Context, hashes [][]byte) ([][]byte, error) {
	retRawTxs := make([][]byte, 0)

	q := `SELECT raw_tx
		FROM metamorph.transactions
		WHERE hash in (SELECT UNNEST($1::BYTEA[]))`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(hashes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rawTx []byte
		err = rows.Scan(&rawTx)
		if err != nil {
			return retRawTxs, err
		}
		retRawTxs = append(retRawTxs, rawTx)
	}

	if err = rows.Err(); err != nil {
		return retRawTxs, err
	}

	return retRawTxs, nil
}

func (p *PostgreSQL) GetMany(ctx context.Context, keys [][]byte) (data []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetMany", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	const q = `
	 SELECT
	 	stored_at
		,hash
		,status
		,block_height
		,block_hash
		,callbacks
		,full_status_updates
		,reject_reason
		,competing_txs
		,raw_tx
		,locked_by
		,merkle_path
		,retries
		,status_history
		,last_modified
	 FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[]));`

	rows, err := p.db.QueryContext(ctx, q, pq.Array(keys))
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return getStoreDataFromRows(rows)
}

func (p *PostgreSQL) IncrementRetries(ctx context.Context, hash *chainhash.Hash) error {
	q := `UPDATE metamorph.transactions SET retries = retries+1 WHERE hash = $1;`

	_, err := p.db.ExecContext(ctx, q, hash[:])
	if err != nil {
		return err
	}

	return nil
}

// Set stores a single record in the transactions table.
func (p *PostgreSQL) Set(ctx context.Context, value *store.Data) (err error) {
	ctx, span := tracing.StartTracing(ctx, "Set", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `INSERT INTO metamorph.transactions (
		 stored_at
		,hash
		,status
		,block_height
		,block_hash
		,callbacks
		,full_status_updates
		,reject_reason
		,competing_txs
		,raw_tx
		,locked_by
		,last_submitted_at
		,status_history
		,last_modified
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
	) ON CONFLICT (hash) DO UPDATE SET last_submitted_at=$12, callbacks=$6;`

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

	callbacksData, err := json.Marshal(value.Callbacks)
	if err != nil {
		return err
	}

	if value.StatusHistory == nil {
		value.StatusHistory = make([]*store.StatusWithTimestamp, 0)
	}
	statusHistoryData, err := json.Marshal(value.StatusHistory)
	if err != nil {
		return err
	}

	_, err = p.db.ExecContext(ctx, q,
		value.StoredAt,
		txHash,
		value.Status,
		value.BlockHeight,
		blockHash,
		callbacksData,
		value.FullStatusUpdates,
		value.RejectReason,
		strings.Join(value.CompetingTxs, ","),
		value.RawTx,
		p.hostname,
		value.LastSubmittedAt,
		statusHistoryData,
		p.now(),
	)
	if err != nil {
		return err
	}

	return nil
}

// SetBulk bulk inserts records into the transactions table. If a record with the same hash already exists the field last_submitted_at will be overwritten with NOW()
func (p *PostgreSQL) SetBulk(ctx context.Context, data []*store.Data) error {
	storedAt := make([]time.Time, len(data))
	hashes := make([][]byte, len(data))
	statuses := make([]int, len(data))
	callbacks := make([]string, len(data))
	statusHistory := make([]string, len(data))
	fullStatusUpdate := make([]bool, len(data))
	rawTxs := make([][]byte, len(data))
	lockedBy := make([]string, len(data))
	lastSubmittedAt := make([]time.Time, len(data))

	for i, txData := range data {
		storedAt[i] = txData.StoredAt
		hashes[i] = txData.Hash[:]
		statuses[i] = int(txData.Status)
		fullStatusUpdate[i] = txData.FullStatusUpdates
		rawTxs[i] = txData.RawTx
		lockedBy[i] = p.hostname
		lastSubmittedAt[i] = txData.LastSubmittedAt

		callbacksData, err := json.Marshal(txData.Callbacks)
		if err != nil {
			return err
		}
		callbacks[i] = string(callbacksData)

		if txData.StatusHistory == nil {
			txData.StatusHistory = make([]*store.StatusWithTimestamp, 0)
		}
		statusHistoryData, err := json.Marshal(txData.StatusHistory)
		if err != nil {
			return err
		}
		statusHistory[i] = string(statusHistoryData)
	}

	q := `INSERT INTO metamorph.transactions (
		 stored_at
		,hash
		,status
		,callbacks
		,full_status_updates
		,raw_tx
		,locked_by
		,last_submitted_at
		,status_history
		,last_modified
		)
		SELECT
			UNNEST($1::TIMESTAMPTZ[]),
			UNNEST($2::BYTEA[]),
			UNNEST($3::INT[]),
			UNNEST($4::TEXT[])::JSONB,
			UNNEST($5::BOOL[]),
			UNNEST($6::BYTEA[]),
			UNNEST($7::TEXT[]),
			UNNEST($8::TIMESTAMPTZ[]),
			UNNEST($9::TEXT[])::JSONB,
			$10
		ON CONFLICT (hash) DO UPDATE SET last_submitted_at = $10, callbacks=EXCLUDED.callbacks;
		`

	_, err := p.db.ExecContext(ctx, q,
		pq.Array(storedAt),
		pq.Array(hashes),
		pq.Array(statuses),
		pq.Array(callbacks),
		pq.Array(fullStatusUpdate),
		pq.Array(rawTxs),
		pq.Array(lockedBy),
		pq.Array(lastSubmittedAt),
		pq.Array(statusHistory),
		p.now(),
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) SetLocked(ctx context.Context, since time.Time, limit int64) error {
	q := `
		UPDATE metamorph.transactions t
		SET locked_by = $1
		WHERE t.hash IN (
		   SELECT t2.hash
		   FROM metamorph.transactions t2
		   WHERE t2.locked_by = 'NONE' AND t2.status <= $3 AND last_submitted_at > $4
		   ORDER BY hash
		   LIMIT $2
		   FOR UPDATE SKIP LOCKED
		);
	;`

	_, err := p.db.ExecContext(ctx, q, p.hostname, limit, metamorph_api.Status_SEEN_ON_NETWORK, since)
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) GetUnseen(ctx context.Context, since time.Time, limit int64, offset int64) (data []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetUnseen", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `SELECT
		stored_at
		,hash
		,status
		,block_height
		,block_hash
		,callbacks
		,full_status_updates
		,reject_reason
		,competing_txs
		,raw_tx
		,locked_by
		,merkle_path
		,retries
		,status_history
		,last_modified
		FROM metamorph.transactions
		WHERE locked_by = $5
		AND status < $1
		AND last_submitted_at > $2
		ORDER BY last_submitted_at DESC
		LIMIT $3 OFFSET $4;`

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, since, limit, offset, p.hostname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return getStoreDataFromRows(rows)
}

// GetSeenPending returns all transactions which are pending in SEEN_ON_NETWORK status for longer than `pendingSince`
func (p *PostgreSQL) GetSeenPending(ctx context.Context, fromDuration time.Duration, sinceLastRequestedDuration time.Duration, pendingSince time.Duration, limit int64, offset int64) (res []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetSeen", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `
	WITH seen_txs AS (SELECT
		TO_TIMESTAMP(substring(elem->>'timestamp' from 1 for 22), 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')::TIMESTAMP WITHOUT TIME ZONE AS seen_at,
		t.stored_at AT TIME ZONE 'UTC' AS stored_at_utc,
		t.stored_at,
		t.hash,
		t.status,
		t.block_height,
		t.block_hash,
		t.callbacks,
		t.full_status_updates,
		t.reject_reason,
		t.competing_txs,
		t.raw_tx,
		t.locked_by,
		t.merkle_path,
		t.retries,
		t.status_history,
		t.last_modified,
		t.last_submitted_at,
	 	t.requested_at,
	 	t.confirmed_at
	FROM
		metamorph.transactions t,
	    LATERAL jsonb_array_elements(status_history) AS elem
	WHERE
	(elem->>'status')::int = $1
	AND t.status = $1
	)
	SELECT
		seen_txs.stored_at,
		seen_txs.hash,
		seen_txs.status,
		seen_txs.block_height,
		seen_txs.block_hash,
		seen_txs.callbacks,
		seen_txs.full_status_updates,
		seen_txs.reject_reason,
		seen_txs.competing_txs,
		seen_txs.raw_tx,
		seen_txs.locked_by,
		seen_txs.merkle_path,
		seen_txs.retries,
		seen_txs.status_history,
		seen_txs.last_modified
	FROM seen_txs
	WHERE seen_txs.last_submitted_at > $2
	AND $8 - seen_txs.seen_at > $3 * INTERVAL '1 SEC'
	AND COALESCE(seen_txs.requested_at, '1900-01-01') < $4 AND COALESCE(seen_txs.confirmed_at, '1900-01-01') < $4
	AND seen_txs.locked_by = $5
	LIMIT $6 OFFSET $7
	;
	`

	getSeenFromAgo := p.now().Add(-1 * fromDuration)
	sinceLastRequested := p.now().Add(-1 * sinceLastRequestedDuration)

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, getSeenFromAgo, pendingSince.Seconds(), sinceLastRequested, p.hostname, limit, offset, p.now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return getStoreDataFromRows(rows)
}

func (p *PostgreSQL) GetSeen(ctx context.Context, fromDuration time.Duration, toDuration time.Duration, limit int64, offset int64) (res []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetSeen", p.tracingEnabled, p.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	q := `SELECT
    		stored_at
			,hash
			,status
			,block_height
			,block_hash
			,callbacks
			,full_status_updates
			,reject_reason
			,competing_txs
			,raw_tx
			,locked_by
			,merkle_path
			,retries
			,status_history
			,last_modified
	FROM metamorph.transactions
	WHERE locked_by = $6
	AND status = $1
	AND last_submitted_at > $2
	AND last_submitted_at <= $3
	LIMIT $4 OFFSET $5
	`

	from := p.now().Add(-1 * fromDuration)
	to := p.now().Add(-1 * toDuration)

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, from, to, limit, offset, p.hostname)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res, err = getStoreDataFromRows(rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) UpdateStatus(ctx context.Context, updates []store.UpdateStatus) (res []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpdateStatusBulk", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(updates)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()
	txHashes, statuses, rejectReasons, statusHistories, timestamps, err := p.prepareStatusHistories(updates)
	if err != nil {
		return nil, err
	}

	qBulk := `
		UPDATE metamorph.transactions
			SET
				status = bulk_query.status,
				reject_reason = bulk_query.reject_reason,
				last_modified = $1,
				status_history = status_history
					|| COALESCE(
						bulk_query.history_update || json_build_object(
							'status', bulk_query.status,
							'timestamp', bulk_query.timestamp
						)::JSONB,
						json_build_object(
							'status', bulk_query.status,
							'timestamp', bulk_query.timestamp
						)::JSONB
					)
			FROM
			(
				SELECT t.hash, t.status, t.reject_reason, t.history_update, t.timestamp
				FROM UNNEST($2::BYTEA[], $3::INT[], $4::TEXT[], $5::JSONB[], $6::TIMESTAMP WITH TIME ZONE[]) AS t(hash, status, reject_reason, history_update, timestamp)
			) AS bulk_query
			WHERE metamorph.transactions.hash = bulk_query.hash
				AND metamorph.transactions.status < bulk_query.status
		RETURNING metamorph.transactions.stored_at
		,metamorph.transactions.hash
		,metamorph.transactions.status
		,metamorph.transactions.block_height
		,metamorph.transactions.block_hash
		,metamorph.transactions.callbacks
		,metamorph.transactions.full_status_updates
		,metamorph.transactions.reject_reason
		,metamorph.transactions.competing_txs
		,metamorph.transactions.raw_tx
		,metamorph.transactions.locked_by
		,metamorph.transactions.merkle_path
		,metamorph.transactions.retries
		,metamorph.transactions.status_history
		,metamorph.transactions.last_modified
		;
    `

	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(`SELECT * FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, qBulk, p.now(), pq.Array(txHashes), pq.Array(statuses), pq.Array(rejectReasons), pq.Array(statusHistories), pq.Array(timestamps))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err = getStoreDataFromRows(rows)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) prepareStatusHistories(updates []store.UpdateStatus) ([][]byte, []metamorph_api.Status, []string, []*string, []time.Time, error) {
	txHashes := make([][]byte, len(updates))
	statuses := make([]metamorph_api.Status, len(updates))
	rejectReasons := make([]string, len(updates))
	statusHistories := make([]*string, len(updates))
	timestamps := make([]time.Time, len(updates))

	for i, update := range updates {
		txHashes[i] = update.Hash.CloneBytes()
		statuses[i] = update.Status
		timestamps[i] = update.Timestamp
		if timestamps[i].IsZero() {
			timestamps[i] = p.now()
		}

		if update.Error != nil {
			rejectReasons[i] = update.Error.Error()
		}

		var historyDataStr *string
		if update.StatusHistory != nil {
			historyData, err := json.Marshal(update.StatusHistory)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			historyStr := string(historyData)
			historyDataStr = &historyStr
		}
		statusHistories[i] = historyDataStr
	}
	return txHashes, statuses, rejectReasons, statusHistories, timestamps, nil
}

func (p *PostgreSQL) UpdateStatusHistory(ctx context.Context, updates []store.UpdateStatus) (res []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpdateStatusHistoryBulk", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(updates)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txHashes := make([][]byte, len(updates))
	statuses := make([]metamorph_api.Status, len(updates))
	statusHistories := make([]*string, len(updates))
	timestamps := make([]time.Time, len(updates))

	for i, update := range updates {
		txHashes[i] = update.Hash.CloneBytes()
		statuses[i] = update.Status
		timestamps[i] = update.Timestamp
		if timestamps[i].IsZero() {
			timestamps[i] = p.now()
		}

		// Marshal the StatusHistory to JSON
		var historyDataStr *string
		if update.StatusHistory != nil {
			historyData, err := json.Marshal(update.StatusHistory)
			if err != nil {
				return nil, err
			}
			historyStr := string(historyData)
			historyDataStr = &historyStr
		}
		statusHistories[i] = historyDataStr
	}

	qBulk := `
    UPDATE metamorph.transactions
    SET
        status_history = status_history || COALESCE((
            SELECT jsonb_agg(new_status)
            FROM (
                SELECT jsonb_build_object(
                   'status', (new_status->>'status')::INT,
                   'timestamp', (new_status->>'timestamp')::TIMESTAMP WITH TIME ZONE
                ) AS new_status
                FROM jsonb_array_elements(COALESCE(bulk_query.history_update, '[]'::JSONB)) AS new_status
                WHERE NOT EXISTS (
                    SELECT 1
                    FROM jsonb_array_elements(metamorph.transactions.status_history) AS existing_status
                    WHERE existing_status->>'status' = new_status->>'status'
                )
                UNION ALL
                SELECT jsonb_build_object(
                    'status', bulk_query.status,
                    'timestamp', bulk_query.timestamp
                ) AS new_status
                WHERE bulk_query.status < metamorph.transactions.status
                  AND NOT EXISTS (
                      SELECT 1
                      FROM jsonb_array_elements(metamorph.transactions.status_history) AS existing_status
                      WHERE existing_status->>'status' = bulk_query.status::text
                  )
            ) AS valid_statuses
        ), '[]'::JSONB)
    FROM (
        SELECT t.hash, t.status, t.history_update, t.timestamp
        FROM UNNEST($1::BYTEA[], $2::INT[], $3::JSONB[], $4::TIMESTAMP WITH TIME ZONE[]) AS t(hash, status, history_update, timestamp)
    ) AS bulk_query
    WHERE metamorph.transactions.hash = bulk_query.hash
    RETURNING metamorph.transactions.stored_at
    ,metamorph.transactions.hash
    ,metamorph.transactions.status
    ,metamorph.transactions.block_height
    ,metamorph.transactions.block_hash
    ,metamorph.transactions.callbacks
    ,metamorph.transactions.full_status_updates
    ,metamorph.transactions.reject_reason
    ,metamorph.transactions.competing_txs
    ,metamorph.transactions.raw_tx
    ,metamorph.transactions.locked_by
    ,metamorph.transactions.merkle_path
    ,metamorph.transactions.retries
    ,metamorph.transactions.status_history
    ,metamorph.transactions.last_modified
    ;
`

	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(`SELECT * FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, qBulk, pq.Array(txHashes), pq.Array(statuses), pq.Array(statusHistories), pq.Array(timestamps))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err = getStoreDataFromRows(rows)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) UpdateDoubleSpend(ctx context.Context, updates []store.UpdateStatus, updateCompetingTxs bool) (res []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpdateDoubleSpend", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(updates)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	qBulk := `
		UPDATE metamorph.transactions
			SET
			status=bulk_query.status,
			reject_reason=bulk_query.reject_reason,
			competing_txs=bulk_query.competing_txs,
			last_modified=$1,
			status_history=status_history
					|| COALESCE(
						bulk_query.history_update || json_build_object(
							'status', bulk_query.status,
							'timestamp', bulk_query.timestamp
						)::JSONB,
						json_build_object(
							'status', bulk_query.status,
							'timestamp', bulk_query.timestamp
						)::JSONB
					)
			FROM
			(
				SELECT t.hash, t.status, t.reject_reason, t.competing_txs, t.timestamp, t.history_update
				FROM UNNEST($2::BYTEA[], $3::INT[], $4::TEXT[], $5::TEXT[], $6::TIMESTAMP WITH TIME ZONE[], $7::JSONB[])
				AS t(hash, status, reject_reason, competing_txs, timestamp, history_update)
			) AS bulk_query
			WHERE metamorph.transactions.hash=bulk_query.hash
				AND metamorph.transactions.status <= bulk_query.status
				AND (metamorph.transactions.competing_txs IS NULL
						OR LENGTH(metamorph.transactions.competing_txs) < LENGTH(bulk_query.competing_txs))
		RETURNING metamorph.transactions.stored_at
		,metamorph.transactions.hash
		,metamorph.transactions.status
		,metamorph.transactions.block_height
		,metamorph.transactions.block_hash
		,metamorph.transactions.callbacks
		,metamorph.transactions.full_status_updates
		,metamorph.transactions.reject_reason
		,metamorph.transactions.competing_txs
		,metamorph.transactions.raw_tx
		,metamorph.transactions.locked_by
		,metamorph.transactions.merkle_path
		,metamorph.transactions.retries
		,metamorph.transactions.status_history
		,metamorph.transactions.last_modified
		;
    `

	txHashes := make([][]byte, len(updates))
	statusHistories := make([]*string, len(updates))
	timestamps := make([]time.Time, len(updates))
	for i, update := range updates {
		txHashes[i] = updates[i].Hash[:]
		timestamps[i] = updates[i].Timestamp
		if timestamps[i].IsZero() {
			timestamps[i] = p.now()
		}
		var historyDataStr *string
		if update.StatusHistory != nil {
			historyData, err := json.Marshal(update.StatusHistory)
			if err != nil {
				return nil, err
			}
			historyStr := string(historyData)
			historyDataStr = &historyStr
		}
		statusHistories[i] = historyDataStr
	}

	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}

	// Get current competing transactions and lock them for update
	rows, err := tx.QueryContext(ctx, `SELECT hash, competing_txs FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollbackErr))
		}
		return nil, err
	}
	defer rows.Close()

	compTxsData := getCompetingTxsFromRows(rows)

	statuses := make([]metamorph_api.Status, len(updates))
	competingTxs := make([]string, len(updates))
	allComletingTxs := make([]string, 0)
	rejectReasons := make([]string, len(updates))

	for i, update := range updates {
		statuses[i] = update.Status

		if update.Error != nil {
			rejectReasons[i] = update.Error.Error()
		}

		for _, tx := range compTxsData {
			if bytes.Equal(txHashes[i], tx.hash) {
				uniqueTxs := mergeUnique(update.CompetingTxs, tx.competingTxs)
				competingTxs[i] = strings.Join(uniqueTxs, ",")
				allComletingTxs = append(allComletingTxs, uniqueTxs...)
				break
			}
		}
	}

	rows, err = tx.QueryContext(ctx, qBulk, p.now(), pq.Array(txHashes), pq.Array(statuses), pq.Array(rejectReasons), pq.Array(competingTxs), pq.Array(timestamps), pq.Array(statusHistories))
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollbackErr))
		}
		return nil, err
	}
	defer rows.Close()

	res, err = getStoreDataFromRows(rows)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollbackErr))
		}
		return nil, err
	}

	if updateCompetingTxs {
		compTxUpdates := make([]store.UpdateStatus, 0)
		for _, cmptx := range allComletingTxs {
			hash, err := hex.DecodeString(cmptx)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			txHash, err := chainhash.NewHash(hash)
			if err != nil {
				fmt.Println(err, cmptx)
				return nil, err
			}
			compTxUpdates = append(compTxUpdates, store.UpdateStatus{
				Hash:   *txHash,
				Status: metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			})
		}
		_, err = p.UpdateDoubleSpend(ctx, compTxUpdates, false)
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) UpdateMined(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) (data []*store.Data, err error) {
	ctx, span := tracing.StartTracing(ctx, "UpdateMined", p.tracingEnabled, append(p.tracingAttributes, attribute.Int("updates", len(txsBlocks)))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if txsBlocks == nil {
		return nil, nil
	}

	txHashes := make([][]byte, len(txsBlocks))
	blockHashes := make([][]byte, len(txsBlocks))
	blockHeights := make([]uint64, len(txsBlocks))
	merklePaths := make([]string, len(txsBlocks))
	statuses := make([]metamorph_api.Status, len(txsBlocks))

	for i, tx := range txsBlocks {
		txHashes[i] = tx.TransactionHash
		blockHashes[i] = tx.BlockHash
		blockHeights[i] = tx.BlockHeight
		merklePaths[i] = tx.MerklePath
		statuses[i] = metamorph_api.Status_MINED
		if tx.BlockStatus == blocktx_api.Status_STALE {
			statuses[i] = metamorph_api.Status_MINED_IN_STALE_BLOCK
		}
	}

	qBulkUpdate := `
		UPDATE metamorph.transactions t
			SET
			    status=bulk_query.mined_status,
			    block_hash=bulk_query.block_hash,
			    block_height=bulk_query.block_height,
			  	merkle_path=bulk_query.merkle_path,
			  	last_modified=$1,
				status_history=status_history || json_build_object(
					'status', bulk_query.mined_status,
					'timestamp', last_modified
				)::JSONB
			FROM
			  (
				SELECT *
				FROM
					UNNEST($2::INT[], $3::BYTEA[], $4::BYTEA[], $5::BIGINT[], $6::TEXT[])
					AS t(mined_status, hash, block_hash, block_height, merkle_path)
			  ) AS bulk_query
			WHERE
			  t.hash=bulk_query.hash
		RETURNING t.stored_at
		,t.hash
		,t.status
		,t.block_height
		,t.block_hash
		,t.callbacks
		,t.full_status_updates
		,t.reject_reason
		,t.competing_txs
		,t.raw_tx
		,t.locked_by
		,t.merkle_path
		,t.retries
		,t.status_history
		,t.last_modified
		;
	`

	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `SELECT hash, competing_txs FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		if rollBackErr := tx.Rollback(); rollBackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollBackErr))
		}
		return nil, err
	}
	defer rows.Close()

	compTxsData := getCompetingTxsFromRows(rows)

	rows, err = tx.QueryContext(ctx, qBulkUpdate, p.now(), pq.Array(statuses), pq.Array(txHashes), pq.Array(blockHashes), pq.Array(blockHeights), pq.Array(merklePaths))
	if err != nil {
		if rollBackErr := tx.Rollback(); rollBackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollBackErr))
		}
		return nil, err
	}

	defer rows.Close()
	res, err := getStoreDataFromRows(rows)
	if err != nil {
		if rollBackErr := tx.Rollback(); rollBackErr != nil {
			return nil, errors.Join(err, fmt.Errorf(failedRollback, rollBackErr))
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	rejectedResponses, err := p.updateDoubleSpendRejected(ctx, compTxsData)
	if err != nil {
		return nil, errors.Join(store.ErrUpdateCompeting, err)
	}

	return append(res, rejectedResponses...), nil
}

func (p *PostgreSQL) updateDoubleSpendRejected(ctx context.Context, competingTxsData []competingTxsData) ([]*store.Data, error) {
	qRejectDoubleSpends := `
		UPDATE metamorph.transactions t
		SET
			status=$1,
			reject_reason=$2
		WHERE t.hash IN (SELECT UNNEST($3::BYTEA[]))
			AND t.status < $1::INT
		RETURNING t.stored_at
		,t.hash
		,t.status
		,t.block_height
		,t.block_hash
		,t.callbacks
		,t.full_status_updates
		,t.reject_reason
		,t.competing_txs
		,t.raw_tx
		,t.locked_by
		,t.merkle_path
		,t.retries
		,t.status_history
		,t.last_modified
		;
	`
	rejectReason := "double spend attempted"

	rejectedCompetingTxs := make([][]byte, 0)
	for _, tx := range competingTxsData {
		for _, competingTx := range tx.competingTxs {
			hash, err := chainhash.NewHashFromStr(competingTx)
			if err != nil {
				continue
			}

			rejectedCompetingTxs = append(rejectedCompetingTxs, hash.CloneBytes())
		}
	}

	if len(rejectedCompetingTxs) == 0 {
		return nil, nil
	}
	// update competing
	rows, err := p.db.QueryContext(ctx, qRejectDoubleSpends, metamorph_api.Status_REJECTED, rejectReason, pq.Array(rejectedCompetingTxs))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res, err := getStoreDataFromRows(rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) Del(ctx context.Context, key []byte) error {
	q := `DELETE FROM metamorph.transactions WHERE hash = $1;`

	_, err := p.db.ExecContext(ctx, q, key)
	if err != nil {
		return err
	}

	return nil
}

// Close implements the MetamorphStore interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (p *PostgreSQL) Close(ctx context.Context) error {
	ctx.Done()
	return p.db.Close()
}

func (p *PostgreSQL) Ping(ctx context.Context) error {
	rows, err := p.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return rows.Close()
}

func (p *PostgreSQL) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	start := p.now()

	deleteBeforeDate := start.Add(-24 * time.Hour * time.Duration(retentionDays))

	res, err := p.db.ExecContext(ctx, "DELETE FROM metamorph.transactions WHERE last_submitted_at <= $1", deleteBeforeDate)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}

func (p *PostgreSQL) GetStats(ctx context.Context, since time.Time, notSeenLimit time.Duration, notFinalLimit time.Duration) (*store.Stats, error) {
	q := `
	SELECT
	max(status_counts.status_count) FILTER (where status_counts.status = $3 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $4 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $5 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $6 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $7 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $8 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $9 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $10 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $11 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $12 )
	FROM
	(SELECT all_statuses.status, COALESCE (found_statuses.status_count, 0) AS status_count FROM
	(SELECT unnest(ARRAY[
	    $3::integer,
	    $4::integer,
	    $5::integer,
	    $6::integer,
	    $7::integer,
	    $8::integer,
	    $9::integer,
	    $10::integer,
	    $11::integer,
	    $12::integer]) AS status) AS all_statuses
	LEFT JOIN
	(
	SELECT
		t.status,
		count(*) AS status_count
	FROM
		metamorph.transactions t WHERE t.last_submitted_at > $1 AND t.locked_by = $2
	GROUP BY
		t.status
	) AS found_statuses ON found_statuses.status = all_statuses.status) AS status_counts
	;
	`

	stats := &store.Stats{}

	err := p.db.QueryRowContext(ctx, q, since, p.hostname,
		metamorph_api.Status_STORED,
		metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		metamorph_api.Status_REQUESTED_BY_NETWORK,
		metamorph_api.Status_SENT_TO_NETWORK,
		metamorph_api.Status_ACCEPTED_BY_NETWORK,
		metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
		metamorph_api.Status_SEEN_ON_NETWORK,
		metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
		metamorph_api.Status_REJECTED,
		metamorph_api.Status_MINED,
	).Scan(
		&stats.StatusStored,
		&stats.StatusAnnouncedToNetwork,
		&stats.StatusRequestedByNetwork,
		&stats.StatusSentToNetwork,
		&stats.StatusAcceptedByNetwork,
		&stats.StatusSeenInOrphanMempool,
		&stats.StatusSeenOnNetwork,
		&stats.StatusDoubleSpendAttempted,
		&stats.StatusRejected,
		&stats.StatusMined,
	)
	if err != nil {
		return nil, err
	}

	q = `
	SELECT
	max(status_counts.status_count) FILTER (where status_counts.status = $2 )
	,max(status_counts.status_count) FILTER (where status_counts.status = $3 )
	FROM
	(SELECT all_statuses.status, COALESCE (found_statuses.status_count, 0) AS status_count FROM
	(SELECT unnest(ARRAY[
	    $2::integer,
	    $3::integer]) AS status) AS all_statuses
	LEFT JOIN
	(
	SELECT
		t.status,
		count(*) AS status_count
	FROM
		metamorph.transactions t WHERE t.last_submitted_at > $1
	GROUP BY
		t.status
	) AS found_statuses ON found_statuses.status = all_statuses.status) AS status_counts
	;
	`

	err = p.db.QueryRowContext(ctx, q, since,
		metamorph_api.Status_SEEN_ON_NETWORK,
		metamorph_api.Status_MINED,
	).Scan(
		&stats.StatusSeenOnNetworkTotal,
		&stats.StatusMinedTotal,
	)
	if err != nil {
		return nil, err
	}

	qNotSeen := `
	SELECT
		count(*)
	FROM
		metamorph.transactions t
		WHERE t.last_submitted_at > $1 AND status < $2 AND t.locked_by = $3
		AND $4 - t.stored_at > $5
`
	err = p.db.QueryRowContext(ctx, qNotSeen, since, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, p.hostname, p.now(), notSeenLimit.Seconds()).Scan(&stats.StatusNotSeen)
	if err != nil {
		return nil, err
	}

	qNotFinal := `
	SELECT
		count(*)
	FROM
		metamorph.transactions t
		WHERE t.last_submitted_at > $1 AND status >= $2 AND status <$3 AND t.locked_by = $4
		AND EXTRACT(EPOCH FROM ($5 - t.stored_at)) > $6
`
	err = p.db.QueryRowContext(ctx, qNotFinal, since, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_REJECTED, p.hostname, p.now(), notFinalLimit.Seconds()).Scan(&stats.StatusNotFinal)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (p *PostgreSQL) SetRequested(ctx context.Context, hashes []*chainhash.Hash) error {
	q := `
		UPDATE metamorph.transactions SET requested_at = $2
		FROM
		(
		SELECT t.hash
		FROM UNNEST($1::BYTEA[]) AS t(hash)
		) AS bulk_query
		WHERE metamorph.transactions.hash = bulk_query.hash
		`

	hashArray := make([][]byte, len(hashes))
	for i, hash := range hashes {
		hashArray[i] = hash[:]
	}

	_, err := p.db.ExecContext(ctx, q, pq.Array(hashArray), p.now())
	if err != nil {
		return err
	}

	return nil
}

// MarkConfirmedRequested updates the confirmed_at date to timestamp now
func (p *PostgreSQL) MarkConfirmedRequested(ctx context.Context, hash *chainhash.Hash) error {
	q := `UPDATE metamorph.transactions SET confirmed_at = $1 WHERE hash = $2`

	_, err := p.db.ExecContext(ctx, q, p.now(), hash[:])
	if err != nil {
		return err
	}

	return nil
}

// GetUnconfirmedRequested gets all entries in table requested_transactions which have either never been confirmed or were last confirmation was more than 'fromAgo' time ago
func (p *PostgreSQL) GetUnconfirmedRequested(ctx context.Context, fromAgo time.Duration, limit int64, offset int64) ([]*chainhash.Hash, error) {
	q := `
	SELECT hash FROM metamorph.transactions t WHERE requested_at IS NOT NULL AND (confirmed_at IS NULL OR confirmed_at < $1) AND status = $4
	LIMIT $2 OFFSET $3
	`
	since := p.now().Add(-fromAgo)

	rows, err := p.db.QueryContext(ctx, q, since, limit, offset, metamorph_api.Status_SEEN_ON_NETWORK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := make([]*chainhash.Hash, 0)

	for rows.Next() {
		var hashBytes []byte
		err = rows.Scan(&hashBytes)
		if err != nil {
			return nil, err
		}

		newHash, err := chainhash.NewHash(hashBytes)
		if err != nil {
			return nil, err
		}

		hashes = append(hashes, newHash)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return hashes, nil
}
