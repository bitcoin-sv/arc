package handler

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/dictionary"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
)

func getTransactionFromNode(peerRpc *config.PeerRpcConfig, inputTxID string) ([]byte, error) {
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpc.User, peerRpc.Password, peerRpc.Host, peerRpc.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to parse rpc URL: %v", err)
	}
	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, err
	}

	var tx *bitcoin.RawTransaction
	tx, err = node.GetRawTransaction(inputTxID)
	if err != nil {
		return nil, err
	}

	var txBytes []byte
	txBytes, err = hex.DecodeString(tx.Hex)
	if err != nil {
		return nil, err
	}

	if txBytes != nil {
		return txBytes, nil
	}

	return nil, metamorph.ErrParentTransactionNotFound
}

func getTransactionFromWhatsOnChain(ctx context.Context, wocApiKey, inputTxID string) ([]byte, error) {
	wocURL := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", "main", inputTxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wocURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", wocApiKey)

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, metamorph.ErrParentTransactionNotFound
	}

	var txHexBytes []byte
	txHexBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	txHex := string(txHexBytes)

	var txBytes []byte
	txBytes, err = hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}

	if txBytes != nil {
		return txBytes, nil
	}

	return nil, metamorph.ErrParentTransactionNotFound
}

// CheckSwagger validates the request against the swagger definition.
func CheckSwagger(e *echo.Echo) *openapi3.T {
	swagger, err := api.GetSwagger()
	if err != nil {
		log.Fatalf(dictionary.GetInternalMessage(dictionary.ErrorLoadingSwaggerSpec), err.Error())
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil
	// Clear out the security requirements, we check this ourselves
	swagger.Security = nil

	// Use our validation middleware to check all requests against the OpenAPI schema.
	e.Use(middleware.OapiRequestValidator(swagger))

	return swagger
}

func convertMerkleRootsRequest(beefMerkleRoots []beef.MerkleRootVerificationRequest) []blocktx.MerkleRootVerificationRequest {
	merkleRoots := make([]blocktx.MerkleRootVerificationRequest, 0)

	for _, mr := range beefMerkleRoots {
		merkleRoots = append(merkleRoots, blocktx.MerkleRootVerificationRequest{
			BlockHeight: mr.BlockHeight,
			MerkleRoot:  mr.MerkleRoot,
		})
	}

	return merkleRoots
}

func findLastStatus(lastTxId string, statuses []*metamorph.TransactionStatus) *metamorph.TransactionStatus {
	for _, status := range statuses {
		if status.TxID == lastTxId {
			return status
		}
	}
	return nil
}

func filterStatusesOfInterest(txIds []string, allStatuses []*metamorph.TransactionStatus) []*metamorph.TransactionStatus {
	idsMap := make(map[string]struct{})
	for _, id := range txIds {
		idsMap[id] = struct{}{}
	}

	filteredStatuses := make([]*metamorph.TransactionStatus, 0)
	for _, txStatus := range allStatuses {
		if _, ok := idsMap[txStatus.TxID]; ok {
			filteredStatuses = append(filteredStatuses, txStatus)
		}
	}

	return filteredStatuses
}
