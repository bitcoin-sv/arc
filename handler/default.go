package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mrz1836/go-datastore"
	"github.com/taal/mapi"
	"github.com/taal/mapi/client"
	"github.com/taal/mapi/dictionary"
)

type MapiDefaultHandler struct {
	Client client.Interface
}

func NewDefault(c client.Interface, opts ...client.Options) (mapi.HandlerInterface, error) {
	bitcoinHandler := &MapiDefaultHandler{Client: c}
	return bitcoinHandler, nil
}

// GetMapiV2FeeQuote ...
func (m MapiDefaultHandler) GetMapiV2FeeQuote(ctx echo.Context) error {

	// should be coming from JWT
	clientID := ctx.QueryParam("client_id")

	mapiFees, err := mapi.GetFeesForClient(ctx.Request().Context(), m.Client, clientID)
	if err != nil {
		dictionaryError := dictionary.GetError(dictionary.ErrorMAPIFeesNotFound)
		return ctx.JSON(http.StatusOK, map[string]interface{}{
			"code":    dictionaryError.Code,
			"message": dictionaryError.PublicMessage,
		})
	}

	if len(mapiFees) == 0 {
		return ctx.JSON(http.StatusOK, map[string]interface{}{
			"code":    0,
			"message": datastore.ErrNoResults.Error(),
		})
	}

	now := time.Now()
	expiryTime := time.Now().Add(30 * time.Minute)
	minerID := m.Client.GetMinerID()
	return ctx.JSON(http.StatusOK, mapi.FeeQuote{
		ApiVersion: &mapi.APIVersion,
		ExpiryTime: &expiryTime,
		Fees:       &mapiFees,
		MinerId:    &minerID,
		Timestamp:  &now,
	})
}

// GetMapiV2PolicyQuote ...
func (m MapiDefaultHandler) GetMapiV2PolicyQuote(ctx echo.Context) error {

	// TODO where does the policy come from?

	now := time.Now()
	minerID := m.Client.GetMinerID()
	return ctx.JSON(http.StatusOK, mapi.PolicyQuote{
		ApiVersion: &mapi.APIVersion,
		Callbacks:  nil,
		ExpiryTime: nil,
		Fees:       nil,
		MinerId:    &minerID,
		Policies:   nil,
		Timestamp:  &now,
	})
}

// PostMapiV2Tx ...
func (m MapiDefaultHandler) PostMapiV2Tx(ctx echo.Context) error {

	tx := ctx.Get("tx").(string)
	node := *m.Client.GetNode(0)
	txID, err := node.SubmitTx(ctx.Request().Context(), []byte(tx))
	if err != nil {
		return err
	}

	now := time.Now()
	minerID := m.Client.GetMinerID()
	return ctx.JSON(http.StatusOK, mapi.SubmitTransactionResponse{
		ApiVersion:                &mapi.APIVersion,
		ConflictedWith:            nil,
		CurrentHighestBlockHash:   nil,
		CurrentHighestBlockHeight: nil,
		MinerId:                   &minerID,
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
	minerID := m.Client.GetMinerID()
	return ctx.JSON(http.StatusOK, mapi.TransactionStatus{
		ApiVersion:            &mapi.APIVersion,
		BlockHash:             nil,
		BlockHeight:           nil,
		Confirmations:         nil,
		MinerId:               &minerID,
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
	minerID := m.Client.GetMinerID()
	return ctx.JSON(http.StatusOK, mapi.SubmitTransactionResponse{
		ApiVersion:                &mapi.APIVersion,
		ConflictedWith:            nil,
		CurrentHighestBlockHash:   nil,
		CurrentHighestBlockHeight: nil,
		MinerId:                   &minerID,
		Status:                    nil,
		StatusCode:                nil,
		Timestamp:                 &now,
		TxSecondMempoolExpiry:     nil,
		Txid:                      nil,
	})
}
