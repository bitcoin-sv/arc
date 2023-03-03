package broadcaster

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/libsv/go-bt/v2"
)

type APIBroadcaster struct {
	arcServer string
	auth      *Auth
}

type Auth struct {
	ApiKey        string
	Bearer        string
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

func (a *APIBroadcaster) BroadcastTransactions(ctx context.Context, txs []*bt.Tx, waitFor metamorph_api.Status) ([]*metamorph_api.TransactionStatus, error) {
	arcClient, err := a.getArcClient()
	if err != nil {
		return nil, err
	}

	waitForStatus := api.WaitForStatus(waitFor)
	params := &api.POSTTransactionsParams{
		XWaitForStatus: &waitForStatus,
	}

	var body []byte
	for _, tx := range txs {
		body = append(body, tx.ExtendedBytes()...)
	}

	bodyReader := bytes.NewReader(body)

	contentType := "application/octet-stream"
	var response *http.Response
	response, err = arcClient.POSTTransactionsWithBody(ctx, params, contentType, bodyReader)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return nil, errors.New("failed to broadcast transactions")
	}

	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
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

func (a *APIBroadcaster) BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitFor metamorph_api.Status) (*metamorph_api.TransactionStatus, error) {
	arcClient, err := a.getArcClient()
	if err != nil {
		return nil, err
	}

	waitForStatus := api.WaitForStatus(waitFor)
	params := &api.POSTTransactionParams{
		XWaitForStatus: &waitForStatus,
	}

	arcBody := api.POSTTransactionJSONBody{
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
	if bodyResponse == nil {
		bodyResponse = response.JSON201
	}

	blockHeight := *bodyResponse.BlockHeight
	txStatus := string(*bodyResponse.TxStatus)

	res := &metamorph_api.TransactionStatus{
		Txid:        *bodyResponse.Txid,
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

	if a.auth.ApiKey != "" {
		// See: https://swagger.io/docs/specification/authentication/api-keys/
		apiKeyProvider, apiKeyProviderErr := securityprovider.NewSecurityProviderApiKey("query", "Api-Key", a.auth.ApiKey)
		if apiKeyProviderErr != nil {
			panic(apiKeyProviderErr)
		}
		opts = append(opts, api.WithRequestEditorFn(apiKeyProvider.Intercept))
	} else if a.auth.Bearer != "" {
		// See: https://swagger.io/docs/specification/authentication/bearer-authentication/
		bearerTokenProvider, bearerTokenProviderErr := securityprovider.NewSecurityProviderBearerToken(a.auth.Bearer)
		if bearerTokenProviderErr != nil {
			panic(bearerTokenProviderErr)
		}
		opts = append(opts, api.WithRequestEditorFn(bearerTokenProvider.Intercept))
	} else if a.auth.Authorization != "" {
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
