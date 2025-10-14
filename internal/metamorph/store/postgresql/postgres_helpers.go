package postgresql

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/global"
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

func getStoreDataFromRows(rows *sql.Rows) ([]*global.TransactionData, error) {
	var storeData []*global.TransactionData

	for rows.Next() {
		data, err := getStoreDataFromRow(rows, &store.TransactionData{})
		if err != nil {
			return nil, err
		}
		storeData = append(storeData, data)
	}

	return storeData, nil
}

func getStoreDataFromRow(rows *sql.Rows, data *global.TransactionData) (*global.TransactionData, error) {
	var storedAt time.Time
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
		&storedAt,
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

	data.StoredAt = storedAt.UTC()

	err = data.UpdateTxHash(txHash)
	if err != nil {
		return nil, err
	}
	err = data.UpdateBlockHash(blockHash)
	if err != nil {
		return nil, err
	}
	err = data.UpdateBlockHeightFromSQL(blockHeight)
	if err != nil {
		return nil, err
	}
	data.UpdateStatusFromSQL(status)
	if len(callbacksData) > 0 {
		callbacks, err := readCallbacksFromDB(callbacksData)
		if err != nil {
			return nil, err
		}
		data.Callbacks = callbacks
	}
	if len(statusHistory) > 0 {
		sHistory, err := readStatusHistoryFromDB(statusHistory)
		if err != nil {
			return nil, err
		}
		data.StatusHistory = sHistory
	}
	data.UpdateRetriesFromSQL(retries)
	data.UpdateCompetingTxs(competingTxs)
	data.UpdateLastModifiedFromSQL(lastModified)
	data.RejectReason = rejectReason.String
	data.MerklePath = merklePath.String
	return data, nil
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

func readCallbacksFromDB(callbacks []byte) ([]global.Callback, error) {
	var callbacksData []global.Callback
	err := json.Unmarshal(callbacks, &callbacksData)
	if err != nil {
		return nil, err
	}
	return callbacksData, nil
}

func readStatusHistoryFromDB(statusHistory []byte) ([]*global.StatusWithTimestamp, error) {
	var statusHistoryData []*global.StatusWithTimestamp
	err := json.Unmarshal(statusHistory, &statusHistoryData)
	if err != nil {
		return nil, err
	}

	for _, status := range statusHistoryData {
		if status != nil {
			status.Timestamp = status.Timestamp.UTC()
		}
	}

	return statusHistoryData, nil
}
