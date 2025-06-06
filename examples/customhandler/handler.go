package main

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"

	"github.com/bitcoin-sv/arc/internal/api/handler"
	apimocks "github.com/bitcoin-sv/arc/internal/api/mocks"
	"github.com/bitcoin-sv/arc/internal/api/transaction_handler"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/bdk/module/gobdk/script"
)

// CustomHandler is our custom arc handler
// Define a custom handler, that overwrites the policy request, but uses other arc requests as is
type CustomHandler struct {
	h           handler.ArcDefaultHandler
	MyCustomVar string `json:"my_custom_var"`
}

type CustomMerkleRootsVerifier struct{}

func (c *CustomMerkleRootsVerifier) VerifyMerkleRoots(_ context.Context, _ []blocktx.MerkleRootVerificationRequest) ([]uint64, error) {
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
	merkleRootVerifier := &mocks.ClientMock{}

	scriptVerifierMock := &apimocks.ScriptVerifierMock{
		VerifyScriptFunc: func(_ []byte, _ []int32, _ int32, _ bool) script.ScriptError {
			return nil
		},
	}
	// create default handler
	defaultHandler, _ := handler.NewDefault(
		nil,
		node,
		merkleRootVerifier,
		nil,
		nil,
		scriptVerifierMock,
		handler.GenesisForkBlockTest,
	)

	defaultHandler.StartUpdateCurrentBlockHeight()

	// create custom handler
	bitcoinHandler := &CustomHandler{
		h: *defaultHandler,
	}

	return bitcoinHandler, nil
}
func (c *CustomHandler) GETPolicy(ctx echo.Context) error {
	// custom get policy logic
	arcPolicy := bitcoin.Settings{}

	return ctx.JSON(http.StatusOK, arcPolicy)
}

func (c *CustomHandler) GETHealth(ctx echo.Context) error {
	return c.h.GETHealth(ctx)
}

func (c *CustomHandler) POSTTransaction(ctx echo.Context, params api.POSTTransactionParams) error {
	return c.h.POSTTransaction(ctx, params)
}

func (c *CustomHandler) GETTransactionStatus(ctx echo.Context, txid string) error {
	return c.h.GETTransactionStatus(ctx, txid)
}

func (c *CustomHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) error {
	return c.h.POSTTransactions(ctx, params)
}
