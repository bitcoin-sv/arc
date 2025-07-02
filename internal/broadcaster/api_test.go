package broadcaster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bitcoin-sv/arc/internal/api/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPBroadcaster(t *testing.T) {
	broadcaster, err := NewHTTPBroadcaster("arc:9090", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, broadcaster)
}

func TestBroadcastTransactions(t *testing.T) {
	testCases := []struct {
		name               string
		httpStatusCode     int
		postTxResponses    []*api.TransactionResponse
		postTxError        *api.ErrorFields
		expectedTxStatuses []*metamorph_api.TransactionStatus
		expectedError      error
	}{
		{
			name:            "error marlformed",
			httpStatusCode:  int(api.ErrStatusMalformed),
			postTxResponses: nil,
			postTxError: &api.ErrorFields{
				Detail: "Transaction is malformed and cannot be processed",
			},
			expectedTxStatuses: nil,
			expectedError:      ErrFailedToBroadcastTxs,
		},
		{
			name:           "valid tx",
			httpStatusCode: int(api.StatusOK),
			postTxResponses: []*api.TransactionResponse{
				{
					BlockHash:   ptrTo("0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd"),
					BlockHeight: ptrTo(uint64(736228)),
					TxStatus:    "SEEN_ON_NETWORK",
					Txid:        "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a",
				},
			},
			postTxError: nil,
			expectedTxStatuses: []*metamorph_api.TransactionStatus{
				{
					Txid:         "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a",
					Status:       metamorph_api.Status_SEEN_ON_NETWORK,
					BlockHeight:  736228,
					BlockHash:    "0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd",
					RejectReason: "",
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var body interface{}

			if tc.postTxError == nil {
				body = tc.postTxResponses
			} else {
				body = tc.postTxError
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)

			response := &http.Response{
				StatusCode: tc.httpStatusCode,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}

			arcClientMock := &mocks.ClientInterfaceMock{
				POSTTransactionsFunc: func(_ context.Context, _ *api.POSTTransactionsParams, _ []api.TransactionRequest, _ ...api.RequestEditorFn) (*http.Response, error) {
					return response, nil
				},
			}

			sut := &APIBroadcaster{
				arcClient: arcClientMock,
			}

			// random valid transaction
			tx, err := sdkTx.NewTransactionFromHex("020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000")
			require.NoError(t, err)

			// when
			actualStatus, actualError := sut.BroadcastTransactions(context.Background(), sdkTx.Transactions{tx}, metamorph_api.Status_SEEN_ON_NETWORK, "", "", false, false)

			assert.Equal(t, tc.expectedTxStatuses, actualStatus)
			if tc.expectedError != nil {
				assert.ErrorIs(t, actualError, tc.expectedError)
			}
		})
	}
}

func TestBroadcastTransaction(t *testing.T) {
	testCases := []struct {
		name             string
		httpStatusCode   int
		postTxResponse   *api.TransactionResponse
		postTxError      *api.ErrorFields
		expectedTxStatus *metamorph_api.TransactionStatus
		expectedError    error
	}{
		{
			name:           "error marlformed",
			httpStatusCode: int(api.ErrStatusMalformed),
			postTxResponse: nil,
			postTxError: &api.ErrorFields{
				Detail: "Transaction is malformed and cannot be processed",
			},
			expectedTxStatus: nil,
			expectedError:    ErrFailedToBroadcastTx,
		},
		{
			name:           "valid tx",
			httpStatusCode: int(api.StatusOK),
			postTxResponse: &api.TransactionResponse{
				BlockHash:   ptrTo("0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd"),
				BlockHeight: ptrTo(uint64(736228)),
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a",
			},
			postTxError: nil,
			expectedTxStatus: &metamorph_api.TransactionStatus{
				Txid:         "b68b064b336b9a4abdb173f3e32f27b38a222cb2102f51b8c92563e816b12b4a",
				Status:       metamorph_api.Status_SEEN_ON_NETWORK,
				BlockHeight:  736228,
				BlockHash:    "0000000000000aac89fbed163ed60061ba33bc0ab9de8e7fd8b34ad94c2414cd",
				RejectReason: "",
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var body interface{}

			if tc.postTxError == nil {
				body = tc.postTxResponse
			} else {
				body = tc.postTxError
			}

			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)

			response := &http.Response{
				StatusCode: tc.httpStatusCode,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
			}

			arcClientMock := &mocks.ClientInterfaceMock{
				POSTTransactionFunc: func(_ context.Context, _ *api.POSTTransactionParams, _ api.TransactionRequest, _ ...api.RequestEditorFn) (*http.Response, error) {
					return response, nil
				},
			}

			sut := &APIBroadcaster{
				arcClient: arcClientMock,
			}

			// random valid transaction
			tx, err := sdkTx.NewTransactionFromHex("020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000")
			require.NoError(t, err)

			// when
			actualStatus, actualError := sut.BroadcastTransaction(context.Background(), tx, metamorph_api.Status_SEEN_ON_NETWORK, "")

			// then
			assert.Equal(t, tc.expectedTxStatus, actualStatus)
			if tc.expectedError != nil {
				assert.ErrorIs(t, actualError, tc.expectedError)
			}
		})
	}
}

func TestGetArcClient(t *testing.T) {
	testCases := []struct {
		name          string
		url           string
		expectedError error
	}{
		{
			name:          "empty url",
			url:           "",
			expectedError: errors.New("arcUrl is not a valid url"),
		},
		{
			name:          "invalid url",
			url:           ":/invalid_url",
			expectedError: errors.New("arcUrl is not a valid url"),
		},
		{
			name:          "valid url",
			url:           "http://localhost:9090",
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			actualArcClient, actualError := getArcClient(tc.url, nil)

			// then
			assert.Equal(t, tc.expectedError, actualError)
			if tc.expectedError == nil {
				assert.NotNil(t, actualArcClient)
			}
		})
	}
}

// PtrTo returns a pointer to the given value.
func ptrTo[T any](v T) *T {
	return &v
}
