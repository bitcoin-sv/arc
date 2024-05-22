package handler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTransactionFromRequest(t *testing.T) {
	testCases := []struct {
		name    string
		tx      string
		txBytes []byte

		expectedHex []byte
	}{
		{
			name:        "valid Raw Transaction",
			tx:          validTx,
			expectedHex: validTxBytes,
		},
		{
			name:        "valid BEEF Transaction",
			tx:          validBeef,
			expectedHex: validBeefBytes,
		},
		{
			name: "invalid transaction",
			tx:   "invalidTx",
		},
	}

	for _, tc := range testCases {
		var requests []*http.Request

		for _, ct := range contentTypes {
			var r *http.Request

			switch ct {
			case "text/plain":
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(tc.tx)))
				r.Header.Set("Content-Type", ct)

			case "application/json":
				body := fmt.Sprintf("{\"rawTx\": \"%s\"}", tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(body)))
				r.Header.Set("Content-Type", ct)

			case "application/octet-stream":
				body, _ := hex.DecodeString(tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer(body))
				r.Header.Set("Content-Type", ct)
			}

			requests = append(requests, r)
		}

		for _, req := range requests {
			t.Run(tc.name+"-"+req.Header.Get("Content-Type"), func(t *testing.T) {
				hexTx, err := parseTransactionFromRequest(req)

				if tc.expectedHex != nil {
					assert.Nil(t, err, "should not return an error")
					assert.Equal(t, tc.expectedHex, hexTx)
				} else {
					assert.Nil(t, hexTx)
					assert.Error(t, err)
				}
			})
		}
	}
}

func TestParseTransactionsFromRequest(t *testing.T) {
	testCases := []struct {
		name string
		tx   string

		expectedHex []byte
	}{
		{
			name:        "Raw Transaction",
			tx:          validTx,
			expectedHex: validTxBytes,
		},
		{
			name:        "BEEF Transaction",
			tx:          validBeef,
			expectedHex: validBeefBytes,
		},
		{
			name: "invalid transaction",
			tx:   "invalidTx",
		},
	}

	for _, tc := range testCases {
		var requests []*http.Request

		for _, ct := range contentTypes {
			var r *http.Request

			switch ct {
			case "text/plain":
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(tc.tx+"\n")))
				r.Header.Set("Content-Type", ct)

			case "application/json":
				body := fmt.Sprintf("[{\"rawTx\": \"%s\"}]", tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(body)))
				r.Header.Set("Content-Type", ct)

			case "application/octet-stream":
				body, _ := hex.DecodeString(tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer(body))
				r.Header.Set("Content-Type", ct)
			}

			requests = append(requests, r)
		}

		for _, req := range requests {
			t.Run(tc.name+"-"+req.Header.Get("Content-Type"), func(t *testing.T) {
				hexTx, err := parseTransactionsFromRequest(req)

				if tc.expectedHex != nil {
					assert.Nil(t, err, "should not return an error")
					assert.Equal(t, tc.expectedHex, hexTx)
				} else {
					assert.Nil(t, hexTx)
					assert.Error(t, err)
				}
			})
		}
	}
}
