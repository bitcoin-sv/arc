package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/labstack/echo/v4"
	"github.com/mrz1836/go-datastore"
	"github.com/ordishs/go-bitcoin"
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

	user, err := GetEchoUser(ctx, m.options.security)
	if err != nil {
		return err
	}

	var policy *models.Policy
	policy, err = models.GetPolicyForClient(ctx.Request().Context(), user.ClientID, models.WithClient(m.Client))
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

	user, err := GetEchoUser(ctx, m.options.security)
	if err != nil {
		return err
	}

	var body []byte
	body, err = io.ReadAll(ctx.Request().Body)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrBadRequest
		e.ExtraInfo = &errStr
		return ctx.JSON(http.StatusBadRequest, e)
	}

	var transaction *models.Transaction
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		if transaction, err = models.NewTransactionFromHex(string(body), models.WithClient(m.Client), models.New()); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(mapi.ErrStatusMalformed, e)
		}
	case "application/json":
		var txHex string
		if err = json.Unmarshal(body, &txHex); err != nil {
			return ctx.JSON(mapi.ErrStatusMalformed, mapi.ErrBadRequest)
		}
		if transaction, err = models.NewTransactionFromHex(txHex, models.WithClient(m.Client), models.New()); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(mapi.ErrStatusMalformed, e)
		}
	case "application/octet-stream":
		if transaction, err = models.NewTransactionFromBytes(body, models.WithClient(m.Client), models.New()); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(mapi.ErrStatusMalformed, e)
		}
	default:
		return ctx.JSON(mapi.ErrBadRequest.Status, mapi.ErrBadRequest)
	}
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

	var status int

	// Check whether we have already processed this transaction and saved it in our DB
	// no need to do any further validation. If it is not processed yet, it will be.
	var existingTransaction *models.Transaction
	existingTransaction, err = models.GetTransaction(ctx.Request().Context(), transaction.ID, models.WithClient(m.Client))
	if err == nil && existingTransaction != nil {

		switch existingTransaction.Status {
		case models.TransactionStatusNew:
			// TODO this should never be the case. Why would we store an invalid transaction
			// should we return another error message here?
			status = mapi.StatusAddedBlockTemplate
		case models.TransactionStatusValid:
			status = mapi.StatusAddedBlockTemplate
		case models.TransactionStatusMempool:
			status = mapi.StatusAlreadyInMempool
		case models.TransactionStatusMined:
			status = mapi.StatusAlreadyMined
		case models.TransactionStatusError:
			// return the actual error status stored on the transaction record
			status = existingTransaction.ErrStatus
		default:
			// TODO should we send a different status code here?
			status = http.StatusOK
		}

		if status >= 400 {
			// an error was previously thrown, just rethrow the same error
			mapiError := mapi.ErrByStatus[status]
			mapiError.Txid = &existingTransaction.ID
			return ctx.JSON(status, mapiError)
		}

		return ctx.JSON(status, mapi.TransactionResponse{
			ApiVersion:  mapi.APIVersion,
			BlockHash:   &existingTransaction.BlockHash,
			BlockHeight: &existingTransaction.BlockHeight,
			MinerId:     m.Client.GetMinerID(),
			Timestamp:   time.Now(),
			Txid:        &existingTransaction.ID,
		})
	}

	if status, err = transaction.Validate(); err != nil || status >= 400 {
		return m.handleError(ctx, status, transaction, err)
	}

	// the transaction has been validated, store in our DB and continue processing
	transaction.Status = models.TransactionStatusValid
	if err = transaction.Save(ctx.Request().Context()); err != nil {
		return err
	}

	// now that the transaction is stored in our DB, fire it off to the mempool and try to get it mined
	// this should return a 200 or 201 if 1 or more of the nodes accept the transaction
	var conflictedWith []string
	status, conflictedWith, err = transaction.SubmitToNodes()
	if status >= 400 || err != nil {
		return m.handleError(ctx, status, transaction, err)
	}

	return ctx.JSON(status, mapi.TransactionResponse{
		ApiVersion:     mapi.APIVersion,
		BlockHash:      &transaction.BlockHash,
		BlockHeight:    &transaction.BlockHeight,
		ConflictedWith: &conflictedWith,
		MinerId:        m.Client.GetMinerID(),
		Timestamp:      time.Now(),
		Txid:           &transaction.ID,
	})
}

// GetMapiV2TxStatusId ...
func (m MapiDefaultHandler) GetMapiV2TxStatusId(ctx echo.Context, id string) error {

	tx, err := models.GetTransaction(ctx.Request().Context(), id, models.WithClient(m.Client))
	if err != nil && !errors.Is(err, datastore.ErrNoResults) {
		return err
	}

	if tx == nil {
		var rawTx *bitcoin.RawTransaction
		if rawTx, err = m.Client.GetTransactionFromNodes(id); err != nil {
			return err
		}
		if rawTx == nil {
			return ctx.JSON(http.StatusNotFound, nil)
		}
		tx = &models.Transaction{
			ID:          rawTx.TxID,
			BlockHash:   rawTx.BlockHash,
			BlockHeight: rawTx.BlockHeight,
		}
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

	tx, err := models.GetTransaction(ctx.Request().Context(), id, models.WithClient(m.Client))
	if err != nil && !errors.Is(err, datastore.ErrNoResults) {
		return err
	}

	var txHex string
	if tx == nil {
		var rawTx *bitcoin.RawTransaction
		if rawTx, err = m.Client.GetTransactionFromNodes(id); err != nil {
			return err
		}
		if rawTx == nil {
			return ctx.JSON(http.StatusNotFound, nil)
		}
		tx = &models.Transaction{
			ID:          rawTx.TxID,
			BlockHash:   rawTx.BlockHash,
			BlockHeight: rawTx.BlockHeight,
		}
		txHex = rawTx.Hex
	} else {
		txHex = hex.EncodeToString(tx.Tx)
	}

	return ctx.JSON(http.StatusOK, mapi.Transaction{
		ApiVersion:  mapi.APIVersion,
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		MinerId:     m.Client.GetMinerID(),
		Timestamp:   time.Now(),
		Tx:          txHex,
		Txid:        tx.ID,
	})
}

// PostMapiV2Txs ...
func (m MapiDefaultHandler) PostMapiV2Txs(ctx echo.Context, params mapi.PostMapiV2TxsParams) error {

	return ctx.JSON(http.StatusNotImplemented, mapi.TransactionResponses{})
}

func (m MapiDefaultHandler) handleError(ctx echo.Context, status int, transaction *models.Transaction, submitErr error) error {

	// enrich the response with the error details
	mapiError := mapi.ErrByStatus[status]
	if mapiError == nil {
		return ctx.JSON(mapi.ErrGeneric.Status, mapi.ErrGeneric)
	}

	mapiError.Txid = &transaction.ID
	if submitErr != nil {
		extraInfo := submitErr.Error()
		mapiError.ExtraInfo = &extraInfo
	}

	logError := models.NewLogError(models.WithClient(m.Client), models.New())
	logError.Error = mapiError
	logError.TxID = transaction.ID

	if accessLog, ok := ctx.Get("access_log").(*models.LogAccess); ok {
		logError.LogAccessID = accessLog.ID
		logError.ClientID = accessLog.ClientID
	}

	if err := logError.Save(ctx.Request().Context()); err != nil {
		return err
	}

	if transaction != nil {
		transaction.Status = models.TransactionStatusError
		transaction.ErrStatus = mapiError.Status
		transaction.ErrInstanceID = logError.ID
		transaction.ErrExtraInfo = *mapiError.ExtraInfo
		if err := transaction.Save(ctx.Request().Context()); err != nil {
			return err
		}
	}

	return ctx.JSON(status, mapiError)
}
