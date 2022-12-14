package main

import (
	"net/http"
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/client"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/handler"
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
	node, err := client.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		return nil, err
	}

	// create a arc client
	var c client.Interface
	c, err = client.New(
		client.WithNode(node),
	)
	if err != nil {
		return nil, err
	}

	bitcoinHandler := &CustomHandler{
		ArcDefaultHandler: handler.ArcDefaultHandler{
			Client: c,
		},
	}

	security := handler.WithSecurityConfig(&config.SecurityConfig{
		Type: config.SecurityTypeCustom,
		// when setting a custom security handler, it is highly recommended defining a custom user function
		CustomGetUser: func(ctx echo.Context) (*api.User, error) {
			return &api.User{
				ClientID: "test",
				Name:     "Test user",
				Admin:    false,
			}, nil
		},
	})
	security(&bitcoinHandler.ArcDefaultHandler.Options)

	return bitcoinHandler, nil
}

// GetArcV1Policy our custom policy request handler
func (c *CustomHandler) GetArcV1Policy(ctx echo.Context) error {

	arcPolicy := api.FeesResponse{
		Timestamp: time.Now(),
	}

	//
	// you can use c.Client.Datastore()... to access the database
	//
	// or run a query directly against the db using gorm
	//
	// db := c.Client.Datastore().Execute("SELECT ....")
	//

	arcPolicy.Fees = &[]api.Fee{} // set the fees ...

	return ctx.JSON(http.StatusOK, arcPolicy)
}
