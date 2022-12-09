package main

import (
	"net/http"
	"time"

	"github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/client"
	"github.com/TAAL-GmbH/arc/config"
	"github.com/TAAL-GmbH/arc/handler"
	"github.com/labstack/echo/v4"
	"github.com/mrz1836/go-datastore"
)

// CustomHandler is our custom arc handler
// Define a custom handler, that overwrites the policy request, but uses other arc requests as is
type CustomHandler struct {
	handler.ArcDefaultHandler
	MyCustomVar string `json:"my_custom_var"`
}

func NewCustomHandler() (arc.HandlerInterface, error) {
	// add a single bitcoin node
	node, err := client.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		return nil, err
	}

	// create a arc client
	var c client.Interface
	c, err = client.New(
		client.WithMinerID(&config.MinerIDConfig{
			PrivateKey: "KznvCNc6Yf4iztSThoMH6oHWzH9EgjfodKxmeuUGPq5DEX5maspS",
		}),
		client.WithNode(node),
		client.WithSQL(datastore.PostgreSQL, []*datastore.SQLConfig{{
			CommonConfig: datastore.CommonConfig{
				TablePrefix: "arc_",
			},
			Host: "localhost",
		}}),
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
		CustomGetUser: func(ctx echo.Context) (*arc.User, error) {
			return &arc.User{
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

	arcPolicy := arc.FeesResponse{
		Timestamp: time.Now(),
	}

	//
	// you can use c.Client.Datastore()... to access the database
	//
	// or run a query directly against the db using gorm
	//
	// db := c.Client.Datastore().Execute("SELECT ....")
	//

	arcPolicy.Fees = &[]arc.Fee{} // set the fees ...

	return ctx.JSON(http.StatusOK, arcPolicy)
}
