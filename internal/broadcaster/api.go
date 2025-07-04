package broadcaster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/api"
)

var (
	ErrStatusNotSupported   = errors.New("status code not supported")
	ErrFailedToBroadcastTxs = errors.New("failed to broadcast transactions")
	ErrFailedToBroadcastTx  = errors.New("failed to broadcast transaction")
	ErrInvalidARCUrl        = errors.New("arcUrl is not a valid url")
)

type APIBroadcaster struct {
	arcClient api.ClientInterface
	logger    *slog.Logger
}

type Auth struct {
	Authorization string
}

type Response struct {
	Txid        string `json:"txid"`
	Status      int    `json:"status"`
	ExtraInfo   string `json:"extraInfo"`
	TxStatus    string `json:"txStatus"`
	BlockHeight uint64 `json:"blockHeight"`
	BlockHash   string `json:"blockHash"`
}

func NewHTTPBroadcaster(arcClient api.ClientInterface, logger *slog.Logger) (*APIBroadcaster, error) {
	return &APIBroadcaster{arcClient: arcClient, logger: logger}, nil
}

func (a *APIBroadcaster) BroadcastTransactions(ctx context.Context, txs sdkTx.Transactions, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool, skipFeeValidation bool) ([]*metamorph_api.TransactionStatus, error) {
	waitFor, ok := metamorph_api.Status_name[int32(waitForStatus)]
	if !ok {
		return nil, errors.Join(ErrStatusNotSupported, fmt.Errorf("status: %d", waitForStatus))
	}
	params := &api.POSTTransactionsParams{
		XWaitFor: &waitFor,
	}

	if callbackURL != "" {
		params.XCallbackUrl = &callbackURL
	}

	if callbackToken != "" {
		params.XCallbackToken = &callbackToken
	}
	params.XFullStatusUpdates = &fullStatusUpdates
	params.XSkipFeeValidation = &skipFeeValidation

	body := make([]api.TransactionRequest, len(txs))
	for i := range txs {
		tx := txs[i]
		rawTx, err := tx.EFHex()
		if err != nil {
			return nil, err
		}
		newAPITransactionRequest := api.TransactionRequest{
			RawTx: rawTx,
		}
		body[i] = newAPITransactionRequest
	}

	var response *http.Response
	response, err := a.arcClient.POSTTransactions(ctx, params, body)
	if err != nil {
		return nil, err
	}

	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		_, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return nil, ErrFailedToBroadcastTxs
	}

	var bodyResponse []Response
	err = json.Unmarshal(bodyBytes, &bodyResponse)
	if err != nil {
		return nil, err
	}

	txStatuses := make([]*metamorph_api.TransactionStatus, len(bodyResponse))

	for idx, tx := range bodyResponse {
		txStatuses[idx] = &metamorph_api.TransactionStatus{
			Txid:         tx.Txid,
			Status:       metamorph_api.Status(metamorph_api.Status_value[tx.TxStatus]),
			RejectReason: tx.ExtraInfo,
			BlockHeight:  tx.BlockHeight,
			BlockHash:    tx.BlockHash,
		}

		// check whether we got an error and the transaction was actually not sent to the network
		if tx.Status != 200 {
			a.logger.Error("failed to broadcast transaction", slog.String("txid", tx.Txid), slog.String("extraInfo", tx.ExtraInfo))
			// set version to 0 to indicate that the transaction was not sent to the network
			txs[idx].Version = 0
		}
	}

	return txStatuses, nil
}

func (a *APIBroadcaster) BroadcastTransaction(ctx context.Context, tx *sdkTx.Transaction, waitForStatus metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error) {
	waitFor, ok := metamorph_api.Status_name[int32(waitForStatus)]
	if !ok {
		return nil, errors.Join(ErrStatusNotSupported, fmt.Errorf("status: %d", waitForStatus))
	}
	params := &api.POSTTransactionParams{
		XWaitFor: &waitFor,
	}

	params.XCallbackUrl = &callbackURL

	rawTx, err := tx.EFHex()
	if err != nil {
		return nil, err
	}
	arcBody := api.POSTTransactionJSONRequestBody{
		RawTx: rawTx,
	}

	response, err := a.arcClient.POSTTransaction(ctx, params, arcBody)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return readJSONBody(response)
	}

	var bodyResponse Response
	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bodyBytes, &bodyResponse)
	if err != nil {
		return nil, err
	}

	res := &metamorph_api.TransactionStatus{
		Txid:         bodyResponse.Txid,
		Status:       metamorph_api.Status(metamorph_api.Status_value[bodyResponse.TxStatus]),
		BlockHeight:  bodyResponse.BlockHeight,
		BlockHash:    bodyResponse.BlockHash,
		RejectReason: bodyResponse.ExtraInfo,
	}

	return res, nil
}

func readJSONBody(response *http.Response) (*metamorph_api.TransactionStatus, error) {
	if response.Body != nil {
		var bodyBytes []byte
		// read body into json map
		var body map[string]interface{}
		err := json.Unmarshal(bodyBytes, &body)
		if err == nil {
			responseBody, ok := body["detail"].(string)
			if ok {
				return nil, errors.Join(ErrFailedToBroadcastTx, errors.New(responseBody))
			}
			return nil, errors.Join(ErrFailedToBroadcastTx, errors.New(string(bodyBytes)))
		}
	}
	return nil, errors.Join(ErrFailedToBroadcastTx, fmt.Errorf("status: %s", response.Status))
}

func GetArcClient(arcServer string, auth *Auth) (*api.Client, error) {
	_, err := url.Parse(arcServer)
	if arcServer == "" || err != nil {
		return nil, ErrInvalidARCUrl
	}

	opts := make([]api.ClientOption, 0)

	if auth != nil && auth.Authorization != "" {
		// custom provider
		opts = append(opts, api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Add("Authorization", auth.Authorization)
			return nil
		}))
	}

	arcClient, err := api.NewClient(arcServer, opts...)
	if err != nil {
		return nil, err
	}

	return arcClient, nil
}

func (a *APIBroadcaster) GetTransactionStatus(_ context.Context, _ string) (*metamorph_api.TransactionStatus, error) {
	return nil, nil
}
