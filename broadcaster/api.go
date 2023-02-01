package broadcaster

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

func (a *APIBroadcaster) BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitFor metamorph_api.Status) (*metamorph_api.TransactionStatus, error) {

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

	waitForStatus := api.WaitForStatus(waitFor)
	params := &api.POSTTransactionParams{
		XCallbackUrl:   nil,
		XCallbackToken: nil,
		XMerkleProof:   nil,
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

func (a *APIBroadcaster) GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error) {
	return nil, nil
}
