package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/api/test"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/validator"
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

	inputTxLowFees         = "0100000001fbbe01d83cb1f53a63ef91c0fce5750cbd8075efef5acd2ff229506a45ab832c010000006a473044022064be2f304950a87782b44e772390836aa613f40312a0df4993e9c5123d0c492d02202009b084b66a3da939fb7dc5d356043986539cac4071372d0a6481d5b5e418ca412103fc12a81e5213e30c7facc15581ac1acbf26a8612a3590ffb48045084b097d52cffffffff02bf010000000000001976a914c2ca67db517c0c972b9a6eb1181880ed3a528e3188acD0070000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac00000000"
	inputTxLowFeesBytes, _ = hex.DecodeString(inputTxLowFees)

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

func TestGETTransactionStatus(t *testing.T) {
	tt := []struct {
		name                 string
		txHandlerStatusFound *transactionHandler.TransactionStatus
		txHandlerErr         error

		expectedStatus   api.StatusCode
		expectedResponse any
	}{
		{
			name: "success",
			txHandlerStatusFound: &transactionHandler.TransactionStatus{
				TxID:      "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
				Status:    "SEEN_ON_NETWORK",
				Timestamp: time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC).Unix(),
			},

			expectedStatus: api.StatusOK,
			expectedResponse: api.TransactionStatus{
				MerklePath:  ptr.To(""),
				BlockHeight: ptr.To(uint64(0)),
				BlockHash:   ptr.To(""),
				Timestamp:   time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC),
				TxStatus:    ptr.To("SEEN_ON_NETWORK"),
				Txid:        "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
			},
		},
		{
			name:                 "error - tx not found",
			txHandlerStatusFound: nil,
			txHandlerErr:         transactionHandler.ErrTransactionNotFound,

			expectedStatus:   api.ErrStatusNotFound,
			expectedResponse: *api.NewErrorFields(api.ErrStatusNotFound, "transaction not found"),
		},
		{
			name:                 "error - generic",
			txHandlerStatusFound: nil,
			txHandlerErr:         errors.New("some error"),

			expectedStatus:   api.ErrStatusGeneric,
			expectedResponse: *api.NewErrorFields(api.ErrStatusGeneric, "some error"),
		},
		{
			name:                 "error - no tx",
			txHandlerStatusFound: nil,
			txHandlerErr:         nil,

			expectedStatus:   api.ErrStatusNotFound,
			expectedResponse: *api.NewErrorFields(api.ErrStatusNotFound, "failed to find transaction"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rec, ctx := createEchoGetRequest("/v1/tx/c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46")

			txHandler := &test.TransactionHandlerMock{
				GetTransactionStatusFunc: func(ctx context.Context, txID string) (*transactionHandler.TransactionStatus, error) {
					return tc.txHandlerStatusFound, tc.txHandlerErr
				},
			}

			defaultHandler, err := NewDefault(p2p.TestLogger{}, txHandler, nil, WithNow(func() time.Time { return time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC) }))
			require.NoError(t, err)

			err = defaultHandler.GETTransactionStatus(ctx, "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46")
			require.NoError(t, err)

			assert.Equal(t, int(tc.expectedStatus), rec.Code)

			b := rec.Body.Bytes()

			switch v := tc.expectedResponse.(type) {
			case api.TransactionStatus:
				var txStatus api.TransactionStatus
				err = json.Unmarshal(b, &txStatus)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txStatus)
			case api.ErrorFields:
				var txErr api.ErrorFields
				err = json.Unmarshal(b, &txErr)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txErr)
			default:
				require.Fail(t, fmt.Sprintf("response type %T does not match any valid types", v))
			}
		})
	}
}

func TestPOSTTransaction(t *testing.T) { //nolint:funlen
	errFieldMissingInputs := *api.NewErrorFields(api.ErrStatusTxFormat, "parent transaction not found")
	errFieldMissingInputs.Txid = ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	errFieldSubmitTx := *api.NewErrorFields(api.ErrStatusGeneric, "failed to submit tx")
	errFieldSubmitTx.Txid = ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	errFieldValidation := *api.NewErrorFields(api.ErrStatusFees, "arc error 465: transaction fee of 0 sat is too low - minimum expected fee is 0 sat")
	errFieldValidation.Txid = ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	now := time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC)

	tt := []struct {
		name             string
		contentType      string
		txHexString      string
		getTx            []byte
		submitTxResponse *transactionHandler.TransactionStatus
		submitTxErr      error

		expectedStatus   api.StatusCode
		expectedResponse any
	}{
		{
			name:        "empty tx - text/plain",
			contentType: contentTypes[0],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "EOF"),
		},
		{
			name:        "invalid tx - application/json",
			contentType: contentTypes[1],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "unexpected end of JSON input"),
		},
		{
			name:        "empty tx - application/octet-stream",
			contentType: contentTypes[2],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "EOF"),
		},
		{
			name:        "invalid mime type",
			contentType: echo.MIMEApplicationXML,
			txHexString: validTx,

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "given content-type application/xml does not match any of the allowed content-types"),
		},
		{
			name:        "invalid tx - text/plain",
			contentType: contentTypes[0],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "encoding/hex: invalid byte: U+0074 't'"),
		},
		{
			name:        "invalid json - application/json",
			contentType: contentTypes[1],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "invalid character 'e' in literal true (expecting 'r')"),
		},
		{
			name:        "invalid tx - application/json",
			contentType: contentTypes[1],
			txHexString: fmt.Sprintf("{\"txHex\": \"%s\"}", validTx),

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "EOF"),
		},
		{
			name:        "invalid tx - application/octet-stream",
			contentType: contentTypes[2],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "could not read varint type: EOF"),
		},
		{
			name:        "valid tx - missing inputs, text/plain",
			contentType: contentTypes[0],
			txHexString: validTx,

			expectedStatus:   460,
			expectedResponse: errFieldMissingInputs,
		},
		{
			name:        "valid tx - fees too low",
			contentType: contentTypes[0],
			txHexString: validTx,
			getTx:       inputTxLowFeesBytes,

			expectedStatus:   465,
			expectedResponse: errFieldValidation,
		},
		{
			name:             "valid tx - submit error",
			contentType:      contentTypes[0],
			txHexString:      validExtendedTx,
			getTx:            inputTxLowFeesBytes,
			submitTxErr:      errors.New("failed to submit tx"),
			submitTxResponse: nil,

			expectedStatus:   409,
			expectedResponse: errFieldSubmitTx,
		},
		{
			name:        "valid tx - success",
			contentType: contentTypes[0],
			txHexString: validExtendedTx,
			getTx:       inputTxLowFeesBytes,

			submitTxResponse: &transactionHandler.TransactionStatus{
				TxID:        validTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   ptr.To(""),
				BlockHeight: ptr.To(uint64(0)),
				ExtraInfo:   ptr.To(""),
				MerklePath:  ptr.To(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        validTxID,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			inputTx := strings.NewReader(tc.txHexString)
			rec, ctx := createEchoPostRequest(inputTx, tc.contentType, "/v1/tx")

			txHandler := &test.TransactionHandlerMock{
				GetTransactionFunc: func(ctx context.Context, txID string) ([]byte, error) {
					return tc.getTx, nil
				},

				SubmitTransactionFunc: func(ctx context.Context, tx []byte, options *api.TransactionOptions) (*transactionHandler.TransactionStatus, error) {
					return tc.submitTxResponse, tc.submitTxErr
				},
			}

			defaultHandler, err := NewDefault(p2p.TestLogger{}, txHandler, defaultPolicy, WithNow(func() time.Time { return now }))
			require.NoError(t, err)

			err = defaultHandler.POSTTransaction(ctx, api.POSTTransactionParams{})
			require.NoError(t, err)

			assert.Equal(t, int(tc.expectedStatus), rec.Code)

			b := rec.Body.Bytes()

			switch v := tc.expectedResponse.(type) {
			case api.TransactionResponse:
				var txResponse api.TransactionResponse
				err = json.Unmarshal(b, &txResponse)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txResponse)
			case api.ErrorFields:
				var txErr api.ErrorFields
				err = json.Unmarshal(b, &txErr)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txErr)
			default:
				require.Fail(t, fmt.Sprintf("response type %T does not match any valid types", v))
			}
		})
	}
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
		rec, ctx := createEchoPostRequest(inputTx, echo.MIMETextPlain, "/v1/tx")
		defaultHandler, err := NewDefault(p2p.TestLogger{}, nil, defaultPolicy)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v1/tx", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMETextPlain)

		options := api.POSTTransactionsParams{
			XCallbackUrl:   ptr.To("callback.example.com"),
			XCallbackToken: ptr.To("test-token"),
			XWaitForStatus: ptr.To(4),
			XMerkleProof:   ptr.To("true"),
		}

		err = defaultHandler.POSTTransactions(ctx, options)
		require.NoError(t, err)
		assert.Equal(t, int(api.ErrStatusBadRequest), rec.Code)

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
		assert.Equal(t, int(api.ErrStatusBadRequest), rec.Code)
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
			rec, ctx := createEchoPostRequest(strings.NewReader("test"), contentType, "/v1/txs")
			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			assert.Equal(t, int(api.ErrStatusBadRequest), rec.Code)

			b := rec.Body.Bytes()
			var bErr api.ErrorBadRequest
			err = json.Unmarshal(b, &bErr)
			require.NoError(t, err)

			errBadRequest := api.NewErrorFields(api.ErrStatusBadRequest, "")

			assert.Equal(t, float64(errBadRequest.Status), bErr.Status)
			assert.Equal(t, errBadRequest.Title, bErr.Title)
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
			rec, ctx := createEchoPostRequest(inputTx, contentType, "/v1/txs")
			err = defaultHandler.POSTTransactions(ctx, api.POSTTransactionsParams{})
			require.NoError(t, err)
			assert.Equal(t, api.StatusOK, api.StatusCode(rec.Code))

			b := rec.Body.Bytes()
			var bErr []api.ErrorFields
			_ = json.Unmarshal(b, &bErr)

			assert.Equal(t, int(api.ErrStatusTxFormat), bErr[0].Status)
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
			rec, ctx := createEchoPostRequest(inputTx, contentType, "/v1/txs")
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

func createEchoPostRequest(inputTx io.Reader, contentType, target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, target, inputTx)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}

func createEchoGetRequest(target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
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
				XCallbackUrl:   ptr.To("http://api.callme.com"),
				XCallbackToken: ptr.To("1234"),
			},

			expectedOptions: &api.TransactionOptions{
				CallbackURL:   "http://api.callme.com",
				CallbackToken: "1234",
			},
		},
		{
			name: "invalid callback url",
			params: api.POSTTransactionParams{
				XCallbackUrl: ptr.To("api.callme.com"),
			},

			expectedErrorStr: "invalid callback URL",
		},
		{
			name: "merkle proof - true",
			params: api.POSTTransactionParams{
				XMerkleProof: ptr.To("true"),
			},

			expectedOptions: &api.TransactionOptions{
				MerkleProof: true,
			},
		},
		{
			name: "merkle proof - 1",
			params: api.POSTTransactionParams{
				XMerkleProof: ptr.To("1"),
			},

			expectedOptions: &api.TransactionOptions{
				MerkleProof: true,
			},
		},
		{
			name: "wait for status - 1",
			params: api.POSTTransactionParams{
				XWaitForStatus: ptr.To(1),
			},

			expectedOptions: &api.TransactionOptions{},
		},
		{
			name: "wait for status - 2",
			params: api.POSTTransactionParams{
				XWaitForStatus: ptr.To(2),
			},

			expectedOptions: &api.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
		},
		{
			name: "wait for status - 6",
			params: api.POSTTransactionParams{
				XWaitForStatus: ptr.To(6),
			},

			expectedOptions: &api.TransactionOptions{
				WaitForStatus: metamorph_api.Status_SENT_TO_NETWORK,
			},
		},
		{
			name: "wait for status - 7",
			params: api.POSTTransactionParams{
				XWaitForStatus: ptr.To(7),
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

func Test_handleError(t *testing.T) {
	tt := []struct {
		name        string
		submitError error

		expectedStatus api.StatusCode
		expectedArcErr *api.ErrorFields
	}{
		{
			name: "no error",

			expectedStatus: api.StatusOK,
			expectedArcErr: nil,
		},
		{
			name:        "generic error",
			submitError: errors.New("some error"),

			expectedStatus: api.ErrStatusGeneric,
			expectedArcErr: &api.ErrorFields{
				Detail:    "Transaction could not be processed",
				ExtraInfo: ptr.To("some error"),
				Title:     "Generic error",
				Type:      "https://arc.bitcoinsv.com/errors/409",
				Txid:      ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    409,
			},
		},
		{
			name: "validator error",
			submitError: &validator.Error{
				ArcErrorStatus: api.ErrStatusBadRequest,
				Err:            errors.New("validation failed"),
			},

			expectedStatus: api.ErrStatusBadRequest,
			expectedArcErr: &api.ErrorFields{
				Detail:    "The request seems to be malformed and cannot be processed",
				ExtraInfo: ptr.To("arc error 400: validation failed"),
				Title:     "Bad request",
				Type:      "https://arc.bitcoinsv.com/errors/400",
				Txid:      ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    400,
			},
		},
		{
			name:        "parent not found error",
			submitError: transactionHandler.ErrParentTransactionNotFound,

			expectedStatus: api.ErrStatusTxFormat,
			expectedArcErr: &api.ErrorFields{
				Detail:    "Transaction is not in extended format, missing input scripts",
				ExtraInfo: ptr.To("parent transaction not found"),
				Title:     "Not extended format",
				Type:      "https://arc.bitcoinsv.com/errors/460",
				Txid:      ptr.To("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    460,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			handler := ArcDefaultHandler{}

			btTx, err := bt.NewTxFromString(validTx)
			require.NoError(t, err)

			ctx := context.Background()
			status, arcErr := handler.handleError(ctx, btTx, tc.submitError)

			require.Equal(t, tc.expectedStatus, status)
			require.Equal(t, tc.expectedArcErr, arcErr)
		})
	}
}
