package main

import (
	"net/http"
	"time"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/config"
	"github.com/TAAL-GmbH/mapi/handler"
	"github.com/labstack/echo/v4"
	"github.com/mrz1836/go-datastore"
	"github.com/ordishs/go-bitcoin"
)

// CustomHandler is our custom mapi handler
// Define a custom handler, that overwrites the policy request, but uses other mapi requests as is
type CustomHandler struct {
	handler.MapiDefaultHandler
	MyCustomVar string `json:"my_custom_var"`
}

func NewCustomHandler() (mapi.HandlerInterface, error) {

	// add a single bitcoin node
	node, err := bitcoin.New("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		return nil, err
	}

	// create a mapi client
	var c client.Interface
	c, err = client.New(
		client.WithMinerID(&config.MinerIDConfig{
			PrivateKey: "KznvCNc6Yf4iztSThoMH6oHWzH9EgjfodKxmeuUGPq5DEX5maspS",
		}),
		client.WithNode(node),
		client.WithSQL(datastore.PostgreSQL, []*datastore.SQLConfig{{
			CommonConfig: datastore.CommonConfig{
				TablePrefix: "mapi_",
			},
			Host: "localhost",
		}}),
	)
	if err != nil {
		return nil, err
	}

	bitcoinHandler := &CustomHandler{
		MapiDefaultHandler: handler.MapiDefaultHandler{
			Client: c,
		},
	}

	security := handler.WithSecurityConfig(&config.SecurityConfig{
		Type: config.SecurityTypeCustom,
		// when setting a custom security handler, it is highly recommended defining a custom user function
		CustomGetUser: func(ctx echo.Context) (*mapi.User, error) {
			return &mapi.User{
				ClientID: "test",
				Name:     "Test user",
				Admin:    false,
			}, nil
		},
	})
	security(&bitcoinHandler.MapiDefaultHandler.Options)

	return bitcoinHandler, nil
}

// GetMapiV2Policy our custom policy request handler
func (c *CustomHandler) GetMapiV2Policy(ctx echo.Context) error {

	mapiPolicy := mapi.Policy{
		ApiVersion: mapi.APIVersion,
		ExpiryTime: time.Now().Add(30 * time.Minute),
		MinerId:    c.Client.GetMinerID(),
		Timestamp:  time.Now(),
	}

	//
	// you can use c.Client.Datastore()... to access the database
	//
	// or run a query directly against the db using gorm
	//
	// db := c.Client.Datastore().Execute("SELECT ....")
	//

	mapiPolicy.Fees = &[]mapi.Fee{} // set the fees ...

	mapiPolicy.Policies = &map[string]interface{}{} // set the policies ...

	return ctx.JSON(http.StatusOK, mapiPolicy)
}
