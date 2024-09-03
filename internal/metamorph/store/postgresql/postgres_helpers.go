package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"strings"
)

type competingTxsData struct {
	hash         []byte
	competingTxs []string
}

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

		var status sql.NullInt32

		var txHash []byte
		var blockHeight sql.NullInt64
		var blockHash []byte

		var callbacksData []byte
		var statusHistory []byte
		var rejectReason sql.NullString
		var competingTxs sql.NullString
		var merklePath sql.NullString
		var retries sql.NullInt32
		var lastModified sql.NullTime

		err := rows.Scan(
			&data.StoredAt,
			&txHash,
			&status,
			&blockHeight,
			&blockHash,
			&callbacksData,
			&data.FullStatusUpdates,
			&rejectReason,
			&competingTxs,
			&data.RawTx,
			&data.LockedBy,
			&merklePath,
			&retries,
			&statusHistory,
			&lastModified,
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

		if status.Valid {
			data.Status = metamorph_api.Status(status.Int32)
		}

		if blockHeight.Valid {
			data.BlockHeight = uint64(blockHeight.Int64)
		}

		if len(callbacksData) > 0 {
			callbacks, err := readCallbacksFromDB(callbacksData)
			if err != nil {
				return nil, err
			}
			data.Callbacks = callbacks
		}

		if retries.Valid {
			data.Retries = int(retries.Int32)
		}

		if competingTxs.String != "" {
			data.CompetingTxs = strings.Split(competingTxs.String, ",")
		}

		if lastModified.Valid {
			data.LastModified = &lastModified.Time
		}

		data.RejectReason = rejectReason.String
		data.MerklePath = merklePath.String

		storeData = append(storeData, data)
	}

	return storeData, nil
}

func getCompetingTxsFromRows(rows *sql.Rows) []competingTxsData {
	dbData := make([]competingTxsData, 0)

	for rows.Next() {
		data := competingTxsData{}

		var hash []byte
		var competingTxs sql.NullString

		err := rows.Scan(
			&hash,
			&competingTxs,
		)
		if err != nil {
			continue
		}

		data.hash = hash

		if competingTxs.String != "" {
			data.competingTxs = strings.Split(competingTxs.String, ",")
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

	competingTxsData := getCompetingTxsFromRows(rows)

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
		return nil
	}

	//rows.Close()

	rows, err := tx.QueryContext(ctx, qRejectDoubleSpends, metamorph_api.Status_REJECTED, rejectReason, pq.Array(rejectedCompetingTxs))
	if err != nil {
		return nil
	}

	res, err := getStoreDataFromRows(rows)
	if err != nil {
		return nil
	}

	//defer rows.Close()

	return res
}

func prepareStructForSaving(data interface{}) ([]byte, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return dataBytes, nil
}

func readCallbacksFromDB(callbacks []byte) ([]store.StoreCallback, error) {
	var callbacksData []store.StoreCallback
	err := json.Unmarshal(callbacks, &callbacksData)
	if err != nil {
		return nil, err
	}
	return callbacksData, nil
}

func readStatusHistoryFromDB(statusHistory []byte) ([]store.StoreStatus, error) {
	var statusHistoryData []store.StoreStatus
	err := json.Unmarshal(statusHistory, &statusHistoryData)
	if err != nil {
		return nil, err
	}
	return statusHistoryData, nil
}
