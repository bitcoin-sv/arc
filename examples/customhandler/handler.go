package main

import (
	"context"
	"net/http"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/api/handler"
	"github.com/bitcoin-sv/arc/pkg/api/transaction_handler"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
)

// CustomHandler is our custom arc handler
// Define a custom handler, that overwrites the policy request, but uses other arc requests as is
type CustomHandler struct {
	handler.ArcDefaultHandler
	MyCustomVar string `json:"my_custom_var"`
}

type CustomMerkleRootsVerifier struct{}

func (c *CustomMerkleRootsVerifier) VerifyMerkleRoots(ctx context.Context, merkleRootVerificationRequest []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
	// Custom Merkle Roots Verification Logic
	return nil, nil
}

func NewCustomHandler() (api.ServerInterface, error) {
	// add a single bitcoin node
	node, err := transaction_handler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		return nil, err
	}

	// add blocktx, header service or custom implementation of merkle roots verifier
	merkleRootVerifier := &CustomMerkleRootsVerifier{}

	bitcoinHandler := &CustomHandler{
		ArcDefaultHandler: handler.ArcDefaultHandler{
			TransactionHandler:  node,
			MerkleRootsVerifier: merkleRootVerifier,
		},
	}

	return bitcoinHandler, nil
}

// GetArcV1Policy our custom policy request handler
func (c *CustomHandler) GetArcV1Policy(ctx echo.Context) error {
	arcPolicy := bitcoin.Settings{}

	//
	// you can use c.Client.Datastore()... to access the database
	//
	// or run a query directly against the db using gorm
	//
	// db := c.Client.Datastore().Execute("SELECT ....")
	//

	return ctx.JSON(http.StatusOK, arcPolicy)
}
