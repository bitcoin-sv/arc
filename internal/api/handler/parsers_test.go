package handler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestParseTransactionFromRequest(t *testing.T) {
	testCases := []struct {
		name    string
		tx      string
		txBytes []byte

		expectedHex        []byte
		invalidContentType bool
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
		{
			name:               "invalid content type",
			tx:                 "invalidTx",
			invalidContentType: true,
		},
	}

	for _, tc := range testCases {
		var requests []*http.Request

		if tc.invalidContentType {
			var r *http.Request
			body := fmt.Sprintf("{\"rawTx\": \"%s\"}", tc.tx)
			r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(body)))
			r.Header.Set(echo.HeaderContentType, echo.MIMEMultipartForm)
			requests = append(requests, r)
		} else {
			for _, ct := range contentTypes {
				var r *http.Request
				//revive:disable:enforce-switch-style
				switch ct {
				case echo.MIMETextPlain:
					r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(tc.tx)))
					r.Header.Set(echo.HeaderContentType, ct)

				case echo.MIMEApplicationJSON:
					body := fmt.Sprintf("{\"rawTx\": \"%s\"}", tc.tx)
					r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(body)))
					r.Header.Set(echo.HeaderContentType, ct)
				case echo.MIMEOctetStream:
					body, _ := hex.DecodeString(tc.tx)
					r, _ = http.NewRequest("GET", "", bytes.NewBuffer(body))
					r.Header.Set(echo.HeaderContentType, ct)
				}
				//revive:enable:enforce-switch-style

				requests = append(requests, r)
			}
		}

		for _, req := range requests {
			t.Run(tc.name+"-"+req.Header.Get(echo.HeaderContentType), func(t *testing.T) {
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
			//revive:disable:enforce-switch-style
			switch ct {
			case echo.MIMETextPlain:
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(tc.tx+"\n")))
				r.Header.Set(echo.HeaderContentType, ct)

			case echo.MIMEApplicationJSON:
				body := fmt.Sprintf("[{\"rawTx\": \"%s\"}]", tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer([]byte(body)))
				r.Header.Set(echo.HeaderContentType, ct)

			case echo.MIMEOctetStream:
				body, _ := hex.DecodeString(tc.tx)
				r, _ = http.NewRequest("GET", "", bytes.NewBuffer(body))
				r.Header.Set(echo.HeaderContentType, ct)
			}
			//revive:enable:enforce-switch-style

			requests = append(requests, r)
		}

		for _, req := range requests {
			t.Run(tc.name+"-"+req.Header.Get(echo.HeaderContentType), func(t *testing.T) {
				// when
				actualHexTx, actualErr := parseTransactionsFromRequest(req)

				if tc.expectedHex != nil {
					assert.Nil(t, actualErr, "should not return an error")
					assert.Equal(t, tc.expectedHex, actualHexTx)
				} else {
					assert.Nil(t, actualHexTx)
					assert.Error(t, actualErr)
				}
			})
		}
	}
}
