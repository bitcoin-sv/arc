package broadcaster

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
)

type APIBroadcaster struct {
	arcServer string
}

func NewHTTPBroadcaster(arcServer string) *APIBroadcaster {
	return &APIBroadcaster{
		arcServer: arcServer,
	}
}

func (a *APIBroadcaster) PutTransaction(ctx context.Context, tx *bt.Tx) (*metamorph_api.TransactionStatus, error) {

	arcClient, err := api.NewClientWithResponses(a.arcServer)
	if err != nil {
		return nil, err
	}

	params := &api.PostArcV1TxParams{
		XCallbackUrl:   nil,
		XCallbackToken: nil,
		XMerkleProof:   nil,
	}

	arcBody := api.PostArcV1TxJSONRequestBody{
		RawTx: hex.EncodeToString(tx.ExtendedBytes()),
	}

	var response *api.PostArcV1TxResponse
	response, err = arcClient.PostArcV1TxWithResponse(ctx, params, arcBody)
	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK && response.StatusCode() != http.StatusCreated {
		if response != nil && response.HTTPResponse != nil {
			// read body into json map
			var body map[string]interface{}
			err = json.Unmarshal(response.Body, &body)
			if err == nil {
				return nil, errors.New(body["detail"].(string))
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
