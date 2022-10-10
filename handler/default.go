package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/taal/mapi"
	"github.com/taal/mapi/bitcoin"
)

type MapiDefaultHandler handler

func NewDefault(opts ...Options) mapi.ServerInterface {

	// default to localhost node with rest service
	node := bitcoin.NewRest("http://locahost:8332")
	bitcoinHandler := &MapiDefaultHandler{
		options: &handlerOptions{
			minerID: "",
			nodes: []*bitcoin.Node{
				&node,
			},
		},
	}

	// Overwrite defaults with any custom options provided by the user
	for _, opt := range opts {
		opt(bitcoinHandler.options)
	}

	return bitcoinHandler
}

// GetMapiV2FeeQuote ...
func (m MapiDefaultHandler) GetMapiV2FeeQuote(ctx echo.Context) error {

	// TODO where does the fee quote come from?

	return ctx.JSON(http.StatusOK, mapi.FeeQuote{
		ApiVersion: &mapi.APIVersion,
		ExpiryTime: nil,
		Fees:       nil,
		MinerId:    &m.options.minerID,
		Timestamp:  nil,
	})
}

// GetMapiV2PolicyQuote ...
func (m MapiDefaultHandler) GetMapiV2PolicyQuote(ctx echo.Context) error {

	// TODO where does the policy come from?

	now := time.Now()
	return ctx.JSON(http.StatusOK, mapi.PolicyQuote{
		ApiVersion: &mapi.APIVersion,
		Callbacks:  nil,
		ExpiryTime: nil,
		Fees:       nil,
		MinerId:    &m.options.minerID,
		Policies:   nil,
		Timestamp:  &now,
	})
}

// PostMapiV2Tx ...
func (m MapiDefaultHandler) PostMapiV2Tx(ctx echo.Context) error {

	tx := ctx.Get("tx").(string)
	// TODO select a node at random? Submit to all nodes?
	node := *m.options.nodes[0]
	txID, err := node.SubmitTx(ctx.Request().Context(), []byte(tx))
	if err != nil {
		return err
	}

	now := time.Now()
	return ctx.JSON(http.StatusOK, mapi.SubmitTransactionResponse{
		ApiVersion:                &mapi.APIVersion,
		ConflictedWith:            nil,
		CurrentHighestBlockHash:   nil,
		CurrentHighestBlockHeight: nil,
		MinerId:                   &m.options.minerID,
		Status:                    nil,
		StatusCode:                nil,
		Timestamp:                 &now,
		TxSecondMempoolExpiry:     nil,
		Txid:                      &txID,
	})
}

// GetMapiV2TxId ...
func (m MapiDefaultHandler) GetMapiV2TxId(ctx echo.Context, id string) error {

	// TODO Get tx info

	now := time.Now()
	return ctx.JSON(http.StatusOK, mapi.TransactionStatus{
		ApiVersion:            &mapi.APIVersion,
		BlockHash:             nil,
		BlockHeight:           nil,
		Confirmations:         nil,
		MinerId:               &m.options.minerID,
		Status:                nil,
		StatusCode:            nil,
		Timestamp:             &now,
		TxSecondMempoolExpiry: nil,
		Txid:                  &id,
	})
}

// PostMapiV2Txs ...
func (m MapiDefaultHandler) PostMapiV2Txs(ctx echo.Context, params mapi.PostMapiV2TxsParams) error {

	// TODO process transactions into node

	now := time.Now()
	return ctx.JSON(http.StatusOK, mapi.SubmitTransactionResponse{
		ApiVersion:                &mapi.APIVersion,
		ConflictedWith:            nil,
		CurrentHighestBlockHash:   nil,
		CurrentHighestBlockHeight: nil,
		MinerId:                   &m.options.minerID,
		Status:                    nil,
		StatusCode:                nil,
		Timestamp:                 &now,
		TxSecondMempoolExpiry:     nil,
		Txid:                      nil,
	})
}
