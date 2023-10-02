package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TestTxID = "testtxid"

var testTime = time.Now()

func TestGetTransactionStatus(t *testing.T) {
	for _, c := range []struct {
		ExpectedTransactionStatus TransactionStatus
		TransactionID             string
	}{
		{
			TransactionID: TestTxID,
			ExpectedTransactionStatus: TransactionStatus{
				Txid:      TestTxID,
				Timestamp: testTime,
			},
		},
	} {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(c.ExpectedTransactionStatus)
		}))
		defer ts.Close()

		client, err := NewClientWithResponses(ts.URL)

		resp, err := client.GETTransactionStatusWithResponse(context.Background(), c.TransactionID)

		require.NoError(t, err)

		assert.Equal(t, c.TransactionID, resp.JSON200.Txid)
		//Warn: assert.Equal(t, testtime, resp.JSON200.Timestamp) returns false
		assert.True(t, c.ExpectedTransactionStatus.Timestamp.Equal(resp.JSON200.Timestamp))
	}
}
