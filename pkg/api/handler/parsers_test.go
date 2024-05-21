package handler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/stretchr/testify/assert"
)

var (
	encodingError      = api.NewErrorFields(api.ErrStatusBadRequest, "encoding/hex: invalid byte: U+0069 'i'")
	noTransactionError = api.NewErrorFields(api.ErrStatusBadRequest, "no transaction found - empty request body")
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
					if req.Header.Get("Content-Type") == "application/octet-stream" {
						assert.Equal(t, noTransactionError, err)
					} else {
						assert.Equal(t, encodingError, err)
					}
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
					if req.Header.Get("Content-Type") == "application/octet-stream" {
						assert.Equal(t, noTransactionError, err)
					} else {
						assert.Equal(t, encodingError, err)
					}
				}
			})
		}
	}
}
