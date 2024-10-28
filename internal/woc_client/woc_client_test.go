package woc_client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

const testnetAddr = "miGidJu3HrpGuFxhF7gCg1yV13AJzANz4P"

func Test_New(t *testing.T) {
	tcs := []struct {
		name                string
		useMainnet          bool
		expectedRequestPath string
	}{
		{
			name:                "client for mainnet",
			useMainnet:          true,
			expectedRequestPath: "/main/",
		},
		{
			name:                "client for testnet",
			useMainnet:          false,
			expectedRequestPath: "/test/",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer svr.Close()

			// given
			sut := New(tc.useMainnet, WithURL(svr.URL))

			// when
			httpReq, err := sut.httpRequest(context.TODO(), "GET", "", nil)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedRequestPath, httpReq.URL.Path)
		})
	}
}

func Test_WithAuth(t *testing.T) {
	tcs := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "client with api key",
			apiKey: "test-api-key",
		},
		{
			name: "client without api key",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer svr.Close()

			// given
			sut := New(false, WithURL(svr.URL), WithAuth(tc.apiKey))

			// when
			httpReq, err := sut.httpRequest(context.TODO(), "GET", "", nil)

			// then
			require.NoError(t, err)

			authHeader, found := httpReq.Header["Authorization"]
			if tc.apiKey != "" {
				require.True(t, found)
				require.Equal(t, tc.apiKey, authHeader[0])
			} else {
				require.False(t, found)
			}
		})
	}
}

func Test_GetUTXOs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	txIDbytes, err := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	require.NoError(t, err)

	tt := []struct {
		name         string
		responseOk   bool
		responseBody any

		expectedError error
		expected      sdkTx.UTXOs
	}{
		{
			name:       "response OK",
			responseOk: true,
			responseBody: []*wocUtxo{
				{
					Txid:     "4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c",
					Vout:     1,
					Height:   2,
					Satoshis: 4,
				},
			},

			expected: sdkTx.UTXOs{{
				TxID:     txIDbytes,
				Vout:     1,
				Satoshis: 4,
			}},
		},
		{
			name:       "response not OK",
			responseOk: false,

			expectedError: ErrWOCResponseNotOK,
		},
		{
			name:         "response not OK",
			responseOk:   true,
			responseBody: "not a WoC UTXO",

			expectedError: ErrWOCFailedToDecodeResponse,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.responseOk {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusBadRequest)
				}

				jsonResp, err := json.Marshal(tc.responseBody)
				require.NoError(t, err)
				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer svr.Close()

			// given
			sut := New(false, WithURL(svr.URL))

			// when
			actual, err := sut.GetUTXOs(context.TODO(), nil, testnetAddr)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			require.Equal(t, tc.expected, actual)
		})
	}
}

func Test_GetBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tt := []struct {
		name       string
		responseOk bool

		expectedError error
	}{
		{
			name:       "response OK",
			responseOk: true,
		},
		{
			name:       "response not OK",
			responseOk: false,

			expectedError: ErrWOCResponseNotOK,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				resp := make(map[string]any)
				if tc.responseOk {
					w.WriteHeader(http.StatusOK)
					resp["confirmed"] = 1
					resp["unconfirmed"] = 2
				} else {
					w.WriteHeader(http.StatusBadRequest)
				}

				jsonResp, err := json.Marshal(resp)
				require.NoError(t, err)
				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer svr.Close()

			// given
			sut := New(false, WithURL(svr.URL))

			// when
			actualConfirmed, actualUnconfirmed, err := sut.GetBalance(context.TODO(), testnetAddr)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			require.Equal(t, int64(1), actualConfirmed)
			require.Equal(t, int64(2), actualUnconfirmed)
		})
	}
}

func Test_TopUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tt := []struct {
		name       string
		responseOk bool
		mainnet    bool

		expectedError error
	}{
		{
			name:       "response OK",
			responseOk: true,
		},
		{
			name:       "response not OK",
			responseOk: false,

			expectedError: ErrWOCResponseNotOK,
		},
		{
			name:       "not testnet",
			mainnet:    true,
			responseOk: true,

			expectedError: ErrWOCFailedToTopUp,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				resp := make(map[string]any)
				if tc.responseOk {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusBadRequest)
				}

				jsonResp, err := json.Marshal(resp)
				require.NoError(t, err)

				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer svr.Close()

			// given
			sut := New(tc.mainnet, WithURL(svr.URL))

			// when
			err := sut.TopUp(context.TODO(), testnetAddr)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_GetRawTxs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	rawTx := &WocRawTx{
		TxID:          "2d5d5def1dcf8304fdd44ecc6e645997b67dc4c5e6310df4bf5286f960c6afb3",
		BlockHash:     "0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd",
		BlockHeight:   10,
		Confirmations: 5,
	}

	tt := []struct {
		name         string
		responseOk   bool
		responseBody any

		expectedError error
		expected      []*WocRawTx
	}{
		{
			name:       "response OK",
			responseOk: true,
			responseBody: []*WocRawTx{
				rawTx, rawTx, rawTx, rawTx,
			},

			expected: []*WocRawTx{rawTx, rawTx, rawTx, rawTx, rawTx, rawTx, rawTx, rawTx},
		},
		{
			name:       "response not OK",
			responseOk: false,

			expectedError: ErrWOCResponseNotOK,
		},
		{
			name:         "response not OK",
			responseOk:   true,
			responseBody: "not a WoC raw TX",

			expectedError: ErrWOCFailedToDecodeRawTxs,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.responseOk {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusBadRequest)
				}

				jsonResp, err := json.Marshal(tc.responseBody)
				require.NoError(t, err)

				_, err = w.Write(jsonResp)
				require.NoError(t, err)
			}))
			defer svr.Close()

			// given
			sut := New(false, WithURL(svr.URL), WithMaxNumIDs(4))

			const idsCount = 8
			ids := make([]string, idsCount)
			for i := 0; i < idsCount; i++ {
				ids[i] = fmt.Sprintf("id%d", i)
			}

			// when
			actual, err := sut.GetRawTxs(context.TODO(), ids)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}
