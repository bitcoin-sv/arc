package main

import (
	"net/http"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/handler"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/labstack/echo/v4"
)

// CustomHandler is our custom arc handler
// Define a custom handler, that overwrites the policy request, but uses other arc requests as is
type CustomHandler struct {
	handler.ArcDefaultHandler
	MyCustomVar string `json:"my_custom_var"`
}

func NewCustomHandler() (api.HandlerInterface, error) {
	// add a single bitcoin node
	node, err := transactionHandler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		return nil, err
	}

	bitcoinHandler := &CustomHandler{
		ArcDefaultHandler: handler.ArcDefaultHandler{
			TransactionHandler: node,
		},
	}

	return bitcoinHandler, nil
}

// GetArcV1Policy our custom policy request handler
func (c *CustomHandler) GetArcV1Policy(ctx echo.Context) error {

	arcPolicy := api.NodePolicy{}

	//
	// you can use c.Client.Datastore()... to access the database
	//
	// or run a query directly against the db using gorm
	//
	// db := c.Client.Datastore().Execute("SELECT ....")
	//

	return ctx.JSON(http.StatusOK, arcPolicy)
}
