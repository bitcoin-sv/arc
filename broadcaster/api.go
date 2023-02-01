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

	type Response []map[string]interface{}

	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	bodyResponse := Response{}
	err = json.Unmarshal(bodyBytes, &bodyResponse)
	if err != nil {
		return nil, err
	}

	txStatuses := make([]*metamorph_api.TransactionStatus, len(bodyResponse))
	for idx, tx := range bodyResponse {
		txStatuses[idx] = &metamorph_api.TransactionStatus{
			Txid:        tx["txid"].(string),
			Status:      metamorph_api.Status(metamorph_api.Status_value[tx["txStatus"].(string)]),
			BlockHeight: int32(tx["blockHeight"].(float64)),
			BlockHash:   tx["blockHash"].(string),
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
	return &metamorph_api.TransactionStatus{
		Txid:        *bodyResponse.Txid,
		Status:      metamorph_api.Status(metamorph_api.Status_value[txStatus]),
		BlockHeight: int32(blockHeight),
		BlockHash:   *bodyResponse.BlockHash,
	}, nil
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
