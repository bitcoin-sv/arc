package handler

import (
	"fmt"
	"log"
	"net/url"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/dictionary"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
)

func getTransactionFromNode(peerRpc *config.PeerRpcConfig, inputTxID string) (*bitcoin.RawTransaction, error) {
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpc.User, peerRpc.Password, peerRpc.Host, peerRpc.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to parse rpc URL: %v", err)
	}
	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, err
	}

	return node.GetRawTransaction(inputTxID)
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

func filterStatusesByTxIDs(txIDs []string, statuses []*metamorph.TransactionStatus) []*metamorph.TransactionStatus {
	if len(txIDs) == 1 && len(statuses) == 1 { // optimization for a common scenario
		if statuses[0].TxID == txIDs[0] {
			return statuses
		}

		return make([]*metamorph.TransactionStatus, 0)
	}

	idsMap := make(map[string]struct{})
	for _, id := range txIDs {
		idsMap[id] = struct{}{}
	}

	filteredStatuses := make([]*metamorph.TransactionStatus, 0)
	for _, txStatus := range statuses {
		if _, ok := idsMap[txStatus.TxID]; ok {
			filteredStatuses = append(filteredStatuses, txStatus)
		}
	}

	return filteredStatuses
}
