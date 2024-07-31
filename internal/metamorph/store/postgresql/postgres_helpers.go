package postgresql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strings"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

// mergeUnique merges two string arrays into one with unique values
func mergeUnique(arr1, arr2 []string) []string {
	valueSet := make(map[string]struct{})

	for _, value := range arr1 {
		valueSet[value] = struct{}{}
	}

	for _, value := range arr2 {
		valueSet[value] = struct{}{}
	}

	uniqueSlice := make([]string, 0, len(valueSet))
	for key := range valueSet {
		uniqueSlice = append(uniqueSlice, key)
	}

	return uniqueSlice
}

func getStoreDataFromRows(rows *sql.Rows) ([]*store.StoreData, error) {
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
		var competingTxs string
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
			&competingTxs,
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

		if competingTxs != "" {
			data.CompetingTxs = strings.Split(competingTxs, ",")
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

func getCompetingTxsFromRows(rows *sql.Rows) []*store.StoreData {
	dbData := make([]*store.StoreData, 0)

	for rows.Next() {
		data := &store.StoreData{}

		var hash []byte
		var competingTxs string

		err := rows.Scan(
			&hash,
			&competingTxs,
		)
		if err != nil {
			continue
		}

		if len(hash) > 0 {
			data.Hash, err = chainhash.NewHash(hash)
			if err != nil {
				continue
			}
		}

		if competingTxs != "" {
			data.CompetingTxs = strings.Split(competingTxs, ",")
		}

		dbData = append(dbData, data)
	}

	return dbData
}

func updateDoubleSpendRejected(ctx context.Context, rows *sql.Rows, tx *sql.Tx) []*store.StoreData {
	qRejectDoubleSpends := `
		UPDATE metamorph.transactions t
		SET
			status=$1,
			reject_reason=$2
		WHERE t.hash IN (SELECT UNNEST($3::BYTEA[]))
			AND t.status < $1::INT
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
		,t.competing_txs
		,t.raw_tx
		,t.locked_by
		,t.merkle_path
		,t.retries
		;
	`
	rejectReason := "double spend attempted"

	dbData := getCompetingTxsFromRows(rows)

	rejectedCompetingTxs := make([][]byte, 0)
	for _, data := range dbData {
		for _, competingTx := range data.CompetingTxs {
			decodedTx, err := hex.DecodeString(competingTx)
			if err != nil {
				continue
			}

			hash, err := chainhash.NewHash(decodedTx)
			if err != nil {
				continue
			}

			rejectedCompetingTxs = append(rejectedCompetingTxs, hash.CloneBytes())
		}
	}

	if len(rejectedCompetingTxs) == 0 {
		return nil
	}

	rows, err := tx.QueryContext(ctx, qRejectDoubleSpends, metamorph_api.Status_REJECTED, rejectReason, pq.Array(rejectedCompetingTxs))
	if err != nil {
		return nil
	}

	res, err := getStoreDataFromRows(rows)
	if err != nil {
		return nil
	}

	return res
}
