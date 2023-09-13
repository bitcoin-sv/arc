package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/test"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var contentTypes = []string{
	echo.MIMETextPlain,
	echo.MIMEApplicationJSON,
	echo.MIMEOctetStream,
}

var (
	validTx         = "0100000001358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffff02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validTxBytes, _ = hex.DecodeString(validTx)
	validExtendedTx = "010000000000000000ef01358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffffc70a0000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validTxID       = "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"

	defaultPolicy = &bitcoin.Settings{
		ExcessiveBlockSize:              2000000000,
		BlockMaxSize:                    512000000,
		MaxTxSizePolicy:                 100000000,
		MaxOrphanTxSize:                 1000000000,
		DataCarrierSize:                 4294967295,
		MaxScriptSizePolicy:             100000000,
		MaxOpsPerScriptPolicy:           4294967295,
		MaxScriptNumLengthPolicy:        10000,
		MaxPubKeysPerMultisigPolicy:     4294967295,
		MaxTxSigopsCountsPolicy:         4294967295,
		MaxStackMemoryUsagePolicy:       100000000,
		MaxStackMemoryUsageConsensus:    200000000,
		LimitAncestorCount:              10000,
		LimitCPFPGroupMembersCount:      25,
		MaxMempool:                      2000000000,
		MaxMempoolSizedisk:              0,
		MempoolMaxPercentCPFP:           10,
		AcceptNonStdOutputs:             true,
		DataCarrier:                     true,
		MinMiningTxFee:                  1e-8,
		MaxStdTxValidationDuration:      3,
		MaxNonStdTxValidationDuration:   1000,
		MaxTxChainValidationBudget:      50,
		ValidationClockCpu:              true,
		MinConsolidationFactor:          20,
		MaxConsolidationInputScriptSize: 150,
		MinConfConsolidationInput:       6,
		MinConsolidationInputMaturity:   6,
		AcceptNonStdConsolidationInput:  false,
	}
)

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, nil)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGETPolicy(t *testing.T) { //nolint:funlen
	t.Run("default policy", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/policy", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.GETPolicy(ctx)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		bPolicy := rec.Body.Bytes()
		var policyResponse api.PolicyResponse
		_ = json.Unmarshal(bPolicy, &policyResponse)

		require.NotNil(t, policyResponse)
		assert.Equal(t, uint64(1), policyResponse.Policy.MiningFee.Satoshis)
		assert.Equal(t, uint64(1000), policyResponse.Policy.MiningFee.Bytes)
		assert.Equal(t, uint64(100000000), policyResponse.Policy.Maxscriptsizepolicy)
		assert.Equal(t, uint64(4294967295), policyResponse.Policy.Maxtxsigopscountspolicy)
		assert.Equal(t, uint64(100000000), policyResponse.Policy.Maxtxsizepolicy)
		assert.False(t, policyResponse.Timestamp.IsZero())
	})
}

func TestPOSTTransaction(t *testing.T) { //nolint:funlen
	t.Run("empty tx", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		for _, contentType := range contentTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, contentType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrBadRequest.Status, rec.Code)
		}
	})

	t.Run("invalid parameters", func(t *testing.T) {
		inputTx := strings.NewReader(validExtendedTx)
		rec, ctx := createEchoRequest(inputTx, echo.MIMETextPlain, "/v1/tx")

		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)

		options := api.POSTTransactionParams{
			XCallbackUrl:   api.StringToPointer("callback.example.com"),
			XCallbackToken: api.StringToPointer("test-token"),
			XWaitForStatus: api.IntToPointer(4),
			XMerkleProof:   api.StringToPointer("true"),
		}

		err = defaultHandler.POSTTransaction(ctx, options)
		assert.Equal(t, api.ErrBadRequest.Status, rec.Code)

		b := rec.Body.Bytes()
		var bErr api.ErrorMalformed
		_ = json.Unmarshal(b, &bErr)

		assert.Equal(t, "invalid callback URL [parse \"callback.example.com\": invalid URI for request]", *bErr.ExtraInfo)
	})

	t.Run("invalid mime type", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationXML)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
		require.NoError(t, err)
		assert.Equal(t, api.ErrBadRequest.Status, rec.Code)
	})

	t.Run("invalid tx", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		expectedErrors := map[string]string{
			echo.MIMETextPlain:       "encoding/hex: invalid byte: U+0074 't'",
			echo.MIMEApplicationJSON: "invalid character 'e' in literal true (expecting 'r')",
			echo.MIMEOctetStream:     "could not read varint type: EOF",
		}

		for contentType, expectedError := range expectedErrors {
			rec, ctx := createEchoRequest(strings.NewReader("test"), contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrBadRequest.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr api.ErrorMalformed
			_ = json.Unmarshal(b, &bErr)

			require.NotNil(t, bErr.ExtraInfo)
			assert.Equal(t, expectedError, *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx - missing inputs", func(t *testing.T) {
		testNode := &test.Node{}
		defaultHandler, err := NewDefault(p2p.TestLogger{}, testNode, defaultPolicy)
		require.NoError(t, err)

		validTxBytes, _ := hex.DecodeString(validTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validTx),
			echo.MIMEApplicationJSON: strings.NewReader("{\"rawTx\":\"" + validTx + "\"}"),
			echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrStatusTxFormat, api.StatusCode(rec.Code))

			b := rec.Body.Bytes()
			var bErr api.ErrorFee
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, "parent transaction not found", *bErr.ExtraInfo)
		}
	})

	t.Run("valid tx with params", func(t *testing.T) {
		testNode := &test.Node{}
		txResult := &transactionHandler.TransactionStatus{
			TxID:        validTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
		}
		// set the node/metamorph responses for the 3 test requests
		testNode.SubmitTransactionResult = append(testNode.SubmitTransactionResult, txResult, txResult, txResult)

		defaultHandler, err := NewDefault(p2p.TestLogger{}, testNode, defaultPolicy)
		require.NoError(t, err)

		validExtendedTxBytes, _ := hex.DecodeString(validExtendedTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validExtendedTx),
			echo.MIMEApplicationJSON: strings.NewReader("{\"rawTx\":\"" + validExtendedTx + "\"}"),
			echo.MIMEOctetStream:     bytes.NewReader(validExtendedTxBytes),
		}

		callbackUrl := "https://callback.example.com"
		callbackToken := "test-token"
		waitFor := 4
		merkleProof := "true"
		options := api.POSTTransactionParams{
			XCallbackUrl:   &callbackUrl,
			XCallbackToken: &callbackToken,
			XWaitForStatus: &waitFor,
			XMerkleProof:   &merkleProof,
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/tx")
			err = defaultHandler.POSTTransaction(ctx, options)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			b := rec.Body.Bytes()
			var bResponse api.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, validTxID, bResponse.Txid)
		}

		// check the callback request
		require.Equal(t, 3, len(testNode.SubmitTransactionRequests))
		for _, req := range testNode.SubmitTransactionRequests {
			assert.Equal(t, callbackUrl, req.Options.CallbackURL)
			assert.Equal(t, callbackToken, req.Options.CallbackToken)
			assert.Equal(t, metamorph_api.Status(waitFor), req.Options.WaitForStatus)
			assert.True(t, req.Options.MerkleProof)
		}
	})
}

func TestPOSTTransactions(t *testing.T) { //nolint:funlen
	t.Run("empty tx", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		for _, contentType := range contentTypes {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/v1/txs", strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, contentType)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			// multiple txs post always returns 200, the error code is given per tx
			assert.Equal(t, api.ErrStatusBadRequest, api.StatusCode(rec.Code))
		}
	})

	t.Run("invalid parameters", func(t *testing.T) {
		inputTx := strings.NewReader(validExtendedTx)
		rec, ctx := createEchoRequest(inputTx, echo.MIMETextPlain, "/v1/tx")
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)

		options := api.POSTTransactionsParams{
			XCallbackUrl:   api.StringToPointer("callback.example.com"),
			XCallbackToken: api.StringToPointer("test-token"),
			XWaitForStatus: api.IntToPointer(4),
			XMerkleProof:   api.StringToPointer("true"),
		}

		err = defaultHandler.POSTTransactions(ctx, options)
		assert.Equal(t, api.ErrBadRequest.Status, rec.Code)

		b := rec.Body.Bytes()
		var bErr api.ErrorMalformed
		_ = json.Unmarshal(b, &bErr)

		assert.Equal(t, "invalid callback URL [parse \"callback.example.com\": invalid URI for request]", *bErr.ExtraInfo)
	})

	t.Run("invalid mime type", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/txs", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationXML)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
		require.NoError(t, err)
		assert.Equal(t, rec.Code, api.ErrBadRequest.Status)
	})

	t.Run("invalid txs", func(t *testing.T) {
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		expectedErrors := map[string]string{
			echo.MIMETextPlain:       "encoding/hex: invalid byte: U+0074 't'",
			echo.MIMEApplicationJSON: "invalid character 'e' in literal true (expecting 'r')",
			echo.MIMEOctetStream:     "",
		}

		for contentType, expectedError := range expectedErrors {
			rec, ctx := createEchoRequest(strings.NewReader("test"), contentType, "/v1/txs")
			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			assert.Equal(t, api.ErrBadRequest.Status, rec.Code)

			b := rec.Body.Bytes()
			var bErr api.ErrorBadRequest
			err = json.Unmarshal(b, &bErr)
			require.NoError(t, err)

			assert.Equal(t, float64(api.ErrBadRequest.Status), bErr.Status)
			assert.Equal(t, api.ErrBadRequest.Title, bErr.Title)
			if expectedError != "" {
				require.NotNil(t, bErr.ExtraInfo)
				assert.Equal(t, expectedError, *bErr.ExtraInfo)
			}
		}
	})

	t.Run("valid tx - missing inputs", func(t *testing.T) {
		txHandler := &test.TransactionHandlerMock{
			SubmitTransactionsFunc: func(ctx context.Context, tx [][]byte, options *api.TransactionOptions) ([]*transactionHandler.TransactionStatus, error) {
				txStatuses := []*transactionHandler.TransactionStatus{}
				return txStatuses, nil
			},

			GetTransactionFunc: func(ctx context.Context, txID string) ([]byte, error) {
				return nil, transactionHandler.ErrTransactionNotFound
			},
		}
		defaultHandler, err := NewDefault(p2p.TestLogger{}, txHandler, defaultPolicy)
		require.NoError(t, err)

		validTxBytes, _ := hex.DecodeString(validTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validTx + "\n"),
			echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validTx + "\"}]"),
			echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/txs")
			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			assert.Equal(t, api.StatusOK, api.StatusCode(rec.Code))

			b := rec.Body.Bytes()
			var bErr []api.ErrorFields
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, api.ErrTxFormat.Status, bErr[0].Status)
			assert.Equal(t, "parent transaction not found", *bErr[0].ExtraInfo)
		}
	})

	t.Run("valid tx", func(t *testing.T) {
		txResult := &transactionHandler.TransactionStatus{
			TxID:        validTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
		}
		// set the node/metamorph responses for the 3 test requests
		txHandler := &test.TransactionHandlerMock{
			SubmitTransactionsFunc: func(ctx context.Context, tx [][]byte, options *api.TransactionOptions) ([]*transactionHandler.TransactionStatus, error) {
				txStatuses := []*transactionHandler.TransactionStatus{txResult}
				return txStatuses, nil
			},
		}

		defaultHandler, err := NewDefault(p2p.TestLogger{}, txHandler, defaultPolicy)
		require.NoError(t, err)

		validExtendedTxBytes, _ := hex.DecodeString(validExtendedTx)
		inputTxs := map[string]io.Reader{
			echo.MIMETextPlain:       strings.NewReader(validExtendedTx + "\n"),
			echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validExtendedTx + "\"}]"),
			echo.MIMEOctetStream:     bytes.NewReader(validExtendedTxBytes),
		}

		for contentType, inputTx := range inputTxs {
			rec, ctx := createEchoRequest(inputTx, contentType, "/v1/txs")
			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			b := rec.Body.Bytes()
			var bResponse []api.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, validTxID, bResponse[0].Txid)
		}
	})
}

func createEchoRequest(inputTx io.Reader, contentType, target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, target, inputTx)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}

func Test_calcFeesFromBSVPerKB(t *testing.T) {
	tests := []struct {
		name     string
		feePerKB float64
		satoshis uint64
		bytes    uint64
	}{
		{
			name:     "50 sats per KB",
			feePerKB: 0.00000050,
			satoshis: 50,
			bytes:    1000,
		},
		{
			name:     "5 sats per KB",
			feePerKB: 0.00000005,
			satoshis: 5,
			bytes:    1000,
		},
		{
			name:     "0.5 sats per KB",
			feePerKB: 0.000000005,
			satoshis: 5,
			bytes:    10000,
		},
		{
			name:     "0.01 sats per KB",
			feePerKB: 0.0000000001,
			satoshis: 1,
			bytes:    100000,
		},
		{
			name:     "0.001 sats per KB - for Craig",
			feePerKB: 0.00000000001,
			satoshis: 1,
			bytes:    1000000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := calcFeesFromBSVPerKB(tt.feePerKB)
			assert.Equalf(t, tt.satoshis, got, "calcFeesFromBSVPerKB(%v)", tt.feePerKB)
			assert.Equalf(t, tt.bytes, got1, "calcFeesFromBSVPerKB(%v)", tt.feePerKB)
		})
	}
}

func TestArcDefaultHandler_extendTransaction(t *testing.T) {
	node := test.Node{
		GetTransactionResult: []interface{}{
			nil,
			validTxBytes,
		},
	}
	tests := []struct {
		name        string
		transaction string
		err         error
	}{
		{
			name:        "valid normal transaction - missing parent",
			transaction: validTx,
			err:         transactionHandler.ErrParentTransactionNotFound,
		},
		{
			name:        "valid normal transaction",
			transaction: validTx,
			err:         nil,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &ArcDefaultHandler{
				TransactionHandler: &node,
				NodePolicy:         &bitcoin.Settings{},
				logger:             p2p.TestLogger{},
			}
			btTx, err := bt.NewTxFromString(tt.transaction)
			require.NoError(t, err)
			if tt.err != nil {
				assert.ErrorIs(t, handler.extendTransaction(ctx, btTx), tt.err, fmt.Sprintf("extendTransaction(%v)", tt.transaction))
			} else {
				assert.NoError(t, handler.extendTransaction(ctx, btTx), fmt.Sprintf("extendTransaction(%v)", tt.transaction))
			}
		})
	}
}

func TestGetTransactionOptions(t *testing.T) {
	tt := []struct {
		name   string
		params api.POSTTransactionParams

		expectedErrorStr string
		expectedOptions  *api.TransactionOptions
	}{
		{
			name:   "no options",
			params: api.POSTTransactionParams{},

			expectedOptions: &api.TransactionOptions{},
		},
		{
			name: "valid callback url",
			params: api.POSTTransactionParams{
				XCallbackUrl:   api.StringToPointer("http://api.callme.com"),
				XCallbackToken: api.StringToPointer("1234"),
			},

			expectedOptions: &api.TransactionOptions{
				CallbackURL:   "http://api.callme.com",
				CallbackToken: "1234",
			},
		},
		{
			name: "invalid callback url",
			params: api.POSTTransactionParams{
				XCallbackUrl: api.StringToPointer("api.callme.com"),
			},

			expectedErrorStr: "invalid callback URL",
		},
		{
			name: "merkle proof - true",
			params: api.POSTTransactionParams{
				XMerkleProof: api.StringToPointer("true"),
			},

			expectedOptions: &api.TransactionOptions{
				MerkleProof: true,
			},
		},
		{
			name: "merkle proof - 1",
			params: api.POSTTransactionParams{
				XMerkleProof: api.StringToPointer("1"),
			},

			expectedOptions: &api.TransactionOptions{
				MerkleProof: true,
			},
		},
		{
			name: "wait for status - 1",
			params: api.POSTTransactionParams{
				XWaitForStatus: api.IntToPointer(1),
			},

			expectedOptions: &api.TransactionOptions{},
		},
		{
			name: "wait for status - 2",
			params: api.POSTTransactionParams{
				XWaitForStatus: api.IntToPointer(2),
			},

			expectedOptions: &api.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
		},
		{
			name: "wait for status - 6",
			params: api.POSTTransactionParams{
				XWaitForStatus: api.IntToPointer(6),
			},

			expectedOptions: &api.TransactionOptions{
				WaitForStatus: metamorph_api.Status_SENT_TO_NETWORK,
			},
		},
		{
			name: "wait for status - 7",
			params: api.POSTTransactionParams{
				XWaitForStatus: api.IntToPointer(7),
			},

			expectedOptions: &api.TransactionOptions{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			options, err := getTransactionOptions(tc.params)

			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedOptions, options)

		})
	}
}
