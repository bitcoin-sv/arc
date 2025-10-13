package main

import (
	"context"
	"net/http"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"

	"github.com/bitcoin-sv/arc/internal/api/handler"
	apiHandlerMocks "github.com/bitcoin-sv/arc/internal/api/handler/mocks"
	"github.com/bitcoin-sv/arc/internal/api/transaction_handler"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/global/mocks"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/api"
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

	dv := &apiHandlerMocks.DefaultValidatorMock{}
	bv := &apiHandlerMocks.BeefValidatorMock{
		ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
			return nil, nil
		},
	}

	// create default handler
	defaultHandler, _ := handler.NewDefault(
		nil,
		node,
		merkleRootVerifier,
		nil,
		dv,
		bv,
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
