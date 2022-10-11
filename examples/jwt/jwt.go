package main

import (
	"context"
	"fmt"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/taal/mapi"
)

func main() {

	apiKeyProvider, apiKeyProviderErr := securityprovider.NewSecurityProviderBearerToken("token")
	if apiKeyProviderErr != nil {
		panic(apiKeyProviderErr)
	}

	c, err := mapi.NewClientWithResponses("mapi.taal.com", mapi.WithRequestEditorFn(apiKeyProvider.Intercept))
	if err != nil {
		panic(err.Error())
	}

	txID := ""
	var tx *mapi.GetMapiV2TxIdResponse
	if tx, err = c.GetMapiV2TxIdWithResponse(context.Background(), txID); err != nil {
		panic(err.Error())
	}

	if tx.StatusCode() == 200 {
		fmt.Println(tx.JSON200)
	} else {
		fmt.Println(tx.Status())
	}
}
