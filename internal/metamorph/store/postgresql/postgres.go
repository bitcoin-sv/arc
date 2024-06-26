package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

const (
	postgresDriverName = "postgres"
	until              = -1 * 2 * time.Hour
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

func (p *PostgreSQL) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
	q := "UPDATE metamorph.transactions SET locked_by = 'NONE' WHERE locked_by = $1 AND (status < $2 OR status = $3);"

	rows, err := p.db.ExecContext(ctx, q, lockedBy, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL)
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
	q := `SELECT
	    stored_at
		,announced_at
		,mined_at
		,last_submitted_at
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
		,retries
	 	FROM metamorph.transactions WHERE hash = $1 LIMIT 1;`

	var storedAt time.Time
	var announcedAt sql.NullTime
	var minedAt sql.NullTime
	var lastSubmittedAt time.Time
	var status sql.NullInt32
	var blockHeight sql.NullInt64
	var blockHash []byte
	var callbackUrl sql.NullString
	var callbackToken sql.NullString
	var fullStatusUpdates bool
	var rejectReason sql.NullString
	var rawTx []byte
	var lockedBy string
	var merklePath sql.NullString
	var retries sql.NullInt32

	err := p.db.QueryRowContext(ctx, q, hash).Scan(
		&storedAt,
		&announcedAt,
		&minedAt,
		&lastSubmittedAt,
		&status,
		&blockHeight,
		&blockHash,
		&callbackUrl,
		&callbackToken,
		&fullStatusUpdates,
		&rejectReason,
		&rawTx,
		&lockedBy,
		&merklePath,
		&retries,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	data := &store.StoreData{}
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

	if announcedAt.Valid {
		data.AnnouncedAt = announcedAt.Time.UTC()
	}

	if minedAt.Valid {
		data.MinedAt = minedAt.Time.UTC()
	}

	data.LastSubmittedAt = lastSubmittedAt.UTC()

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

	data.FullStatusUpdates = fullStatusUpdates

	if rejectReason.Valid {
		data.RejectReason = rejectReason.String
	}

	data.LockedBy = lockedBy

	if merklePath.Valid {
		data.MerklePath = merklePath.String
	}

	if retries.Valid {
		data.Retries = int(retries.Int32)
	}

	return data, nil
}

func (p *PostgreSQL) IncrementRetries(ctx context.Context, hash *chainhash.Hash) error {
	q := `UPDATE metamorph.transactions SET retries = retries+1 WHERE hash = $1;`

	_, err := p.db.ExecContext(ctx, q, hash[:])
	if err != nil {
		return err
	}

	return nil
}

// Set implements the MetamorphStore interface. It attempts to store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (p *PostgreSQL) Set(ctx context.Context, _ []byte, value *store.StoreData) error {
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
		,last_submitted_at
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
	) ON CONFLICT (hash) DO UPDATE SET last_submitted_at=$14`

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
		value.LastSubmittedAt,
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
		   WHERE t2.locked_by = 'NONE'
		   AND (t2.status < $3 OR t2.status = $4)
		   AND last_submitted_at > $5
		   LIMIT $2
		   FOR UPDATE SKIP LOCKED
		);
	;`

	_, err := p.db.ExecContext(ctx, q, p.hostname, limit, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, since)
	if err != nil {
		return err
	}

	return nil
}

func (p *PostgreSQL) GetUnmined(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.StoreData, error) {

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
		,retries
		FROM metamorph.transactions
		WHERE locked_by = $6
		AND (status < $1 OR status = $2)
		AND last_submitted_at > $3
		ORDER BY last_submitted_at DESC
		LIMIT $4 OFFSET $5;`

	rows, err := p.db.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, since, limit, offset, p.hostname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return p.getStoreDataFromRows(rows)
}

func (p *PostgreSQL) GetSeenOnNetwork(ctx context.Context, since time.Time, untilTime time.Time, limit int64, offset int64) ([]*store.StoreData, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`SELECT hash
	FROM metamorph.transactions
	WHERE (locked_by = $6 OR locked_by = 'NONE')
	AND status = $1
	AND last_submitted_at > $2
	AND last_submitted_at <= $3
	ORDER BY hash DESC
	LIMIT $4 OFFSET $5
	FOR UPDATE`, metamorph_api.Status_SEEN_ON_NETWORK, since, untilTime, limit, offset, p.hostname)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	q := `UPDATE metamorph.transactions
		SET locked_by=$6
		FROM (
			SELECT hash
			FROM metamorph.transactions
			WHERE (locked_by = $6 OR locked_by = 'NONE')
			AND status = $1
			AND last_submitted_at >= $2
			AND last_submitted_at <= $3
			ORDER BY hash DESC
			LIMIT $4 OFFSET $5)
		as t
		WHERE metamorph.transactions.hash = t.hash
		RETURNING
	     stored_at
		,announced_at
		,mined_at
		,t.hash
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
		,retries;`

	rows, err := tx.QueryContext(ctx, q, metamorph_api.Status_SEEN_ON_NETWORK, since, untilTime, limit, offset, p.hostname)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	res, err := p.getStoreDataFromRows(rows)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}
	defer rows.Close()

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) UpdateStatusBulk(ctx context.Context, updates []store.UpdateStatus) ([]*store.StoreData, error) {
	txHashes := make([][]byte, len(updates))
	statuses := make([]metamorph_api.Status, len(updates))
	rejectReasons := make([]string, len(updates))

	for i, update := range updates {
		txHashes[i] = update.Hash.CloneBytes()
		statuses[i] = update.Status
		rejectReasons[i] = update.RejectReason
	}

	qBulk := `
		UPDATE metamorph.transactions
			SET
			status=bulk_query.status,
			reject_reason=bulk_query.reject_reason
			FROM
			(
				SELECT *
						FROM
						UNNEST($1::BYTEA[], $2::INT[], $3::TEXT[])
				AS t(hash, status, reject_reason)
			) AS bulk_query
			WHERE
			metamorph.transactions.hash=bulk_query.hash
				AND ( metamorph.transactions.status < bulk_query.status OR ( metamorph.transactions.status = $5 AND bulk_query.status = $4 ))
			    AND metamorph.transactions.status != $6
		RETURNING metamorph.transactions.stored_at
		,metamorph.transactions.announced_at
		,metamorph.transactions.mined_at
		,metamorph.transactions.hash
		,metamorph.transactions.status
		,metamorph.transactions.block_height
		,metamorph.transactions.block_hash
		,metamorph.transactions.callback_url
		,metamorph.transactions.callback_token
		,metamorph.transactions.full_status_updates
		,metamorph.transactions.reject_reason
		,metamorph.transactions.raw_tx
		,metamorph.transactions.locked_by
		,metamorph.transactions.merkle_path
		,metamorph.transactions.retries
		;
    `

	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`SELECT * FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, qBulk, pq.Array(txHashes), pq.Array(statuses), pq.Array(rejectReasons), metamorph_api.Status_SEEN_ON_NETWORK, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, metamorph_api.Status_MINED)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	res, err := p.getStoreDataFromRows(rows)
	if err != nil {
		return res, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) UpdateMined(ctx context.Context, txsBlocks *blocktx_api.TransactionBlocks) ([]*store.StoreData, error) {
	if txsBlocks == nil {
		return nil, nil
	}

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
		,t.retries
		;
`
	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`SELECT * FROM metamorph.transactions WHERE hash in (SELECT UNNEST($1::BYTEA[])) ORDER BY hash FOR UPDATE`, pq.Array(txHashes))
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, qBulkUpdate, metamorph_api.Status_MINED, p.now(), pq.Array(txHashes), pq.Array(blockHashes), pq.Array(blockHeights), pq.Array(merklePaths))
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return nil, err
	}

	res, err := p.getStoreDataFromRows(rows)
	if err != nil {
		return res, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *PostgreSQL) getStoreDataFromRows(rows *sql.Rows) ([]*store.StoreData, error) {
	var storeData []*store.StoreData

	for rows.Next() {
		data := &store.StoreData{}

		var announcedAt sql.NullTime
		var minedAt sql.NullTime
		var status sql.NullInt32

		var txHash []byte
		var blockHeight sql.NullInt64
		var blockHash []byte

		var callbackUrl sql.NullString
		var callbackToken sql.NullString
		var rejectReason sql.NullString
		var merklePath sql.NullString
		var retries sql.NullInt32

		err := rows.Scan(
			&data.StoredAt,
			&announcedAt,
			&minedAt,
			&txHash,
			&status,
			&blockHeight,
			&blockHash,
			&callbackUrl,
			&callbackToken,
			&data.FullStatusUpdates,
			&rejectReason,
			&data.RawTx,
			&data.LockedBy,
			&merklePath,
			&retries,
		)
		if err != nil {
			return nil, err
		}

		if len(txHash) > 0 {
			data.Hash, err = chainhash.NewHash(txHash)
			if err != nil {
				return nil, err
			}
		}

		if len(blockHash) > 0 {
			data.BlockHash, err = chainhash.NewHash(blockHash)
			if err != nil {
				return nil, err
			}
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

		if rejectReason.Valid {
			data.RejectReason = rejectReason.String
		}

		if merklePath.Valid {
			data.MerklePath = merklePath.String
		}

		if retries.Valid {
			data.Retries = int(retries.Int32)
		}

		storeData = append(storeData, data)
	}

	return storeData, nil
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
	_, err := p.db.QueryContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return nil
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

func (p *PostgreSQL) GetStats(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*store.Stats, error) {
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
	    $11::integer]) AS status) AS all_statuses
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
		metamorph_api.Status_SEEN_ON_NETWORK,
		metamorph_api.Status_MINED,
		metamorph_api.Status_REJECTED,
		metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
	).Scan(
		&stats.StatusStored,
		&stats.StatusAnnouncedToNetwork,
		&stats.StatusRequestedByNetwork,
		&stats.StatusSentToNetwork,
		&stats.StatusAcceptedByNetwork,
		&stats.StatusSeenOnNetwork,
		&stats.StatusMined,
		&stats.StatusRejected,
		&stats.StatusSeenInOrphanMempool,
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
	err = p.db.QueryRowContext(ctx, qNotSeen, since, metamorph_api.Status_SEEN_ON_NETWORK, p.hostname, p.now(), notSeenLimit.Seconds()).Scan(&stats.StatusNotSeen)
	if err != nil {
		return nil, err
	}

	qNotMined := `
	SELECT
		count(*)
	FROM
		metamorph.transactions t
		WHERE t.last_submitted_at > $1 AND status = $2 AND t.locked_by = $3
		AND EXTRACT(EPOCH FROM ($4 - t.stored_at)) > $5
`
	err = p.db.QueryRowContext(ctx, qNotMined, since, metamorph_api.Status_SEEN_ON_NETWORK, p.hostname, p.now(), notMinedLimit.Seconds()).Scan(&stats.StatusNotMined)
	if err != nil {
		return nil, err
	}

	return stats, nil
}
