package broadcaster

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
)

type APIBroadcaster struct {
	arcServer string
	auth      *Auth
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

func NewHTTPBroadcaster(arcServer string, auth *Auth) *APIBroadcaster {
	return &APIBroadcaster{
		arcServer: arcServer,
		auth:      auth,
	}
}

func (a *APIBroadcaster) BroadcastTransactions(ctx context.Context, txs []*bt.Tx, waitFor metamorph_api.Status, callbackURL string, callbackToken string, fullStatusUpdates bool) ([]*metamorph_api.TransactionStatus, error) {
	arcClient, err := a.getArcClient()
	if err != nil {
		return nil, err
	}

	waitForStatus := api.WaitForStatus(waitFor)
	params := &api.POSTTransactionsParams{
		XWaitForStatus: &waitForStatus,
	}

	if callbackURL != "" {
		params.XCallbackUrl = &callbackURL
	}
	if callbackToken != "" {
		params.XCallbackToken = &callbackToken
	}
	params.XFullStatusUpdates = &fullStatusUpdates

	body := make([]api.TransactionRequest, len(txs))
	for i := range txs {
		tx := txs[i]
		newApiTransactionRequest := api.TransactionRequest{
			RawTx: hex.EncodeToString(tx.ExtendedBytes()),
		}
		body[i] = newApiTransactionRequest
	}

	var response *http.Response
	response, err = arcClient.POSTTransactions(ctx, params, body)
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
		return nil, fmt.Errorf("failed to broadcast transactions: %s", string(bodyBytes))
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
			fmt.Printf("Error broadcasting tx: %#v\n", tx)
			// set version to 0 to indicate that the transaction was not sent to the network
			txs[idx].Version = 0
		}
	}

	return txStatuses, nil
}

func (a *APIBroadcaster) BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitFor metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error) {
	arcClient, err := a.getArcClient()
	if err != nil {
		return nil, err
	}

	waitForStatus := api.WaitForStatus(waitFor)
	params := &api.POSTTransactionParams{
		XWaitForStatus: &waitForStatus,
	}

	if callbackURL != "" {
		params.XCallbackUrl = &callbackURL
	}

	arcBody := api.POSTTransactionJSONRequestBody{
		RawTx: hex.EncodeToString(tx.ExtendedBytes()),
	}

	var response *api.POSTTransactionResponse
	response, err = arcClient.POSTTransactionWithResponse(ctx, params, arcBody)
	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusCreated {
		if response != nil && response.HTTPResponse != nil {
			// read body into json map
			var body map[string]interface{}
			err = json.Unmarshal(response.Body, &body)
			if err == nil {
				responseBody, ok := body["detail"].(string)
				if ok {
					return nil, errors.New(responseBody)
				}
				return nil, fmt.Errorf("error: %s", string(response.Body))
			}
		}
		return nil, errors.New("error: " + response.Status())
	}

	bodyResponse := response.JSON200

	blockHeight := *bodyResponse.BlockHeight
	txStatus := bodyResponse.TxStatus

	res := &metamorph_api.TransactionStatus{
		Txid:        bodyResponse.Txid,
		Status:      metamorph_api.Status(metamorph_api.Status_value[txStatus]),
		BlockHeight: blockHeight,
		BlockHash:   *bodyResponse.BlockHash,
	}

	if bodyResponse.ExtraInfo != nil {
		res.RejectReason = *bodyResponse.ExtraInfo
	}

	return res, nil
}

func (a *APIBroadcaster) getArcClient() (*api.ClientWithResponses, error) {

	opts := make([]api.ClientOption, 0)

	if a.auth.Authorization != "" {
		// custom provider
		opts = append(opts, api.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", a.auth.Authorization)
			return nil
		}))
	}

	arcClient, err := api.NewClientWithResponses(a.arcServer, opts...)
	if err != nil {
		return nil, err
	}

	return arcClient, nil
}

func (a *APIBroadcaster) GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error) {
	return nil, nil
}
