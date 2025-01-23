package tests

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

func CallbackRecordEqual(a, b *store.CallbackData) bool {
	return reflect.DeepEqual(*a, *b)
}

func ReadAllCallbacks(t *testing.T, db *sql.DB) []*store.CallbackData {
	t.Helper()

	r, err := db.Query(
		`SELECT url
			,token
			,tx_id
			,tx_status
			,extra_info
			,merkle_path
			,block_hash
			,block_height
			,timestamp
			,competing_txs
		FROM callbacker.callbacks`,
	)

	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var callbacks []*store.CallbackData

	for r.Next() {
		c := &store.CallbackData{}
		var ei sql.NullString
		var mp sql.NullString
		var bh sql.NullString
		var bheight sql.NullInt64
		var competingTxs sql.NullString

		_ = r.Scan(&c.URL, &c.Token, &c.TxID, &c.TxStatus, &ei, &mp, &bh, &bheight, &c.Timestamp, &competingTxs)

		if ei.Valid {
			c.ExtraInfo = &ei.String
		}
		if mp.Valid {
			c.MerklePath = &mp.String
		}
		if bh.Valid {
			c.BlockHash = &bh.String
		}
		if bheight.Valid {
			c.BlockHeight = ptrTo(uint64(bheight.Int64))
		}
		if competingTxs.Valid {
			c.CompetingTxs = strings.Split(competingTxs.String, ",")
		}

		c.Timestamp = c.Timestamp.UTC()

		callbacks = append(callbacks, c)
	}

	return callbacks
}

func ptrTo[T any](v T) *T {
	return &v
}
