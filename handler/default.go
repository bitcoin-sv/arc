package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mrz1836/go-datastore"
	"github.com/taal/mapi"
	"github.com/taal/mapi/client"
	"github.com/taal/mapi/models"
)

type MapiDefaultHandler struct {
	Client  client.Interface
	options handlerOptions
}

func NewDefault(c client.Interface, opts ...Options) (mapi.HandlerInterface, error) {
	bitcoinHandler := &MapiDefaultHandler{Client: c}

	for _, opt := range opts {
		opt(&bitcoinHandler.options)
	}

	return bitcoinHandler, nil
}

// GetMapiV2Policy ...
func (m MapiDefaultHandler) GetMapiV2Policy(ctx echo.Context) error {

	user := GetEchoUser(ctx, m.options.security)

	policy, err := models.GetPolicyForClient(ctx.Request().Context(), user.ClientID, models.WithClient(m.Client))
	if err != nil && !errors.Is(err, datastore.ErrNoResults) {
		return err
	}
	if policy == nil {
		policy, err = models.GetDefaultPolicy(ctx.Request().Context(), models.WithClient(m.Client))
		if err != nil {
			if errors.Is(err, datastore.ErrNoResults) {
				return echo.NewHTTPError(http.StatusNotFound, "not found")
			}
			return err
		}
	}

	mapiPolicy := mapi.Policy{
		ApiVersion: mapi.APIVersion,
		ExpiryTime: time.Now().Add(30 * time.Second), // TODO get from config
		MinerId:    m.Client.GetMinerID(),
		Timestamp:  time.Now(),
	}

	fees := make([]mapi.Fee, len(policy.Fees))
	for index, fee := range policy.Fees {
		fees[index] = fee
	}
	mapiPolicy.Fees = &fees

	policies := mapi.Policy_Policies{}
	for key, p := range policy.Policies {
		policies.Set(key, p)
	}
	mapiPolicy.Policies = &policies

	return ctx.JSON(http.StatusOK, mapiPolicy)
}

// PostMapiV2Tx ...
func (m MapiDefaultHandler) PostMapiV2Tx(ctx echo.Context, params mapi.PostMapiV2TxParams) error {

	user := GetEchoUser(ctx, m.options.security)

	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errStr := err.Error()
		e := ErrBadRequest
		e.ExtraInfo = &errStr
		return ctx.JSON(http.StatusBadRequest, e)
	}

	// TODO check for an exising transaction in our DB

	var transaction *models.Transaction
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		if transaction, err = models.NewTransactionFromHex(string(body)); err != nil {
			errStr := err.Error()
			e := ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(StatusMalformed, e)
		}
	case "application/json":
		var txHex string
		if err = json.Unmarshal(body, &txHex); err != nil {
			return ctx.JSON(StatusMalformed, ErrBadRequest)
		}
		if transaction, err = models.NewTransactionFromHex(txHex); err != nil {
			errStr := err.Error()
			e := ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(StatusMalformed, e)
		}
	case "application/octet-stream":
		if transaction, err = models.NewTransactionFromBytes(body); err != nil {
			errStr := err.Error()
			e := ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(StatusMalformed, e)
		}
	}

	if status, mapiError := transaction.Validate(); mapiError != nil {
		return ctx.JSON(status, mapiError)
	}

	// the transaction has been validated, store in our DB and continue processing
	transaction.Status = models.TransactionStatusValid
	transaction.ClientID = user.ClientID

	if params.XCallbackUrl != nil {
		transaction.CallbackURL = *params.XCallbackUrl
		if params.XCallbackToken != nil {
			transaction.CallbackToken = *params.XCallbackToken
		}
	}

	transaction.MerkleProof = false
	if params.XMerkleProof != nil {
		if *params.XMerkleProof == "true" || *params.XMerkleProof == "1" {
			transaction.MerkleProof = true
		}
	}

	if err = transaction.Save(ctx.Request().Context()); err != nil {
		// just throw a 500 error
		return err
	}

	// now that the transaction is stored in our DB, fire it off to the mempool and try to get mined
	status, conflictedWith, submitErr := transaction.SubmitToNodes()

	if status >= 400 {
		transaction.Status = models.TransactionStatusError
		_ = transaction.Save(ctx.Request().Context())
		return m.handleError(ctx, status, transaction, submitErr)
	}

	var blockHash *string
	var blockHeight *uint64
	if transaction.BlockHeight > 0 {
		blockHash = &transaction.BlockHash
		blockHeight = &transaction.BlockHeight
	}

	return ctx.JSON(int(status), mapi.TransactionResponse{
		ApiVersion:     mapi.APIVersion,
		BlockHash:      blockHash,
		BlockHeight:    blockHeight,
		ConflictedWith: &conflictedWith,
		MinerId:        m.Client.GetMinerID(),
		Timestamp:      time.Now(),
		Txid:           &transaction.ID,
	})
}

// GetMapiV2TxStatusId ...
func (m MapiDefaultHandler) GetMapiV2TxStatusId(ctx echo.Context, id string) error {

	tx, err := models.GetTransaction(ctx.Request().Context(), id)
	if err != nil {
		if errors.Is(err, datastore.ErrNoResults) {
			return ctx.JSON(http.StatusNotFound, nil)
		}
		return err
	}

	return ctx.JSON(http.StatusOK, mapi.TransactionStatus{
		ApiVersion:  mapi.APIVersion,
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		MinerId:     m.Client.GetMinerID(),
		Timestamp:   time.Now(),
		Txid:        tx.ID,
	})
}

// GetMapiV2TxId Similar to GetMapiV2TxStatusId, but also returns the whole transaction
func (m MapiDefaultHandler) GetMapiV2TxId(ctx echo.Context, id string) error {

	tx, err := models.GetTransaction(ctx.Request().Context(), id)
	if err != nil {
		if errors.Is(err, datastore.ErrNoResults) {
			return ctx.JSON(http.StatusNotFound, nil)
		}
		return err
	}

	return ctx.JSON(http.StatusOK, mapi.Transaction{
		ApiVersion:  mapi.APIVersion,
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		MinerId:     m.Client.GetMinerID(),
		Timestamp:   time.Now(),
		Tx:          hex.EncodeToString(tx.Tx),
		Txid:        tx.ID,
	})
}

// PostMapiV2Txs ...
func (m MapiDefaultHandler) PostMapiV2Txs(ctx echo.Context, params mapi.PostMapiV2TxsParams) error {

	return ctx.JSON(http.StatusNotImplemented, mapi.TransactionResponses{})
}

func (m MapiDefaultHandler) handleError(ctx echo.Context, status uint, transaction *models.Transaction, submitErr error) error {

	// enrich the response with the error details
	mapiError := ErrByStatus[status]
	if mapiError == nil {
		return ctx.JSON(int(ErrGeneric.Status), ErrGeneric)
	}

	mapiError.Txid = &transaction.ID
	if submitErr != nil {
		extraInfo := submitErr.Error()
		mapiError.ExtraInfo = &extraInfo
	}

	logError := models.NewLogError(models.WithClient(m.Client))
	logError.Error = mapiError
	logError.Status = status
	logError.TxID = transaction.ID

	if logAccess, ok := ctx.Get("access_log").(*models.LogAccess); ok {
		logError.LogAccessID = logAccess.ID
	}

	if err := logError.Save(ctx.Request().Context()); err != nil {
		return err
	}

	return ctx.JSON(int(status), mapiError)
}
