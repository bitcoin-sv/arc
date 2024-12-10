package handler

import (
	"log"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	middleware "github.com/oapi-codegen/echo-middleware"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/dictionary"
)

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
