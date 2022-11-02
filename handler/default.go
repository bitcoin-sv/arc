package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/mrz1836/go-datastore"
	"github.com/ordishs/go-bitcoin"
)

type MapiDefaultHandler struct {
	Client  client.Interface
	Options handlerOptions
}

func NewDefault(c client.Interface, opts ...Options) (mapi.HandlerInterface, error) {
	bitcoinHandler := &MapiDefaultHandler{Client: c}

	for _, opt := range opts {
		opt(&bitcoinHandler.Options)
	}

	return bitcoinHandler, nil
}

// GetMapiV2Policy ...
func (m MapiDefaultHandler) GetMapiV2Policy(ctx echo.Context) error {

	user, err := GetEchoUser(ctx, m.Options.security)
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
				return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
			}
			return err
		}
	}

	if policy == nil {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
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

	policies := map[string]interface{}{}
	for key, p := range policy.Policies {
		policies[key] = p
	}
	mapiPolicy.Policies = &policies

	return ctx.JSON(http.StatusOK, mapiPolicy)
}

// PostMapiV2Tx ...
func (m MapiDefaultHandler) PostMapiV2Tx(ctx echo.Context, params mapi.PostMapiV2TxParams) error {

	user, err := GetEchoUser(ctx, m.Options.security)
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
		var txJson mapi.PostMapiV2TxJSONBody
		if err = json.Unmarshal(body, &txJson); err != nil {
			return ctx.JSON(mapi.ErrStatusMalformed, mapi.ErrBadRequest)
		}
		if transaction, err = models.NewTransactionFromHex(txJson.RawTx, models.WithClient(m.Client), models.New()); err != nil {
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

	status, response, responseErr := m.processTransaction(ctx, transaction)
	if responseErr != nil {
		// if an error is returned, the processing failed and we should return a 500 error
		return responseErr
	}

	return ctx.JSON(status, response)
}

// GetMapiV2TxStatusId ...
func (m MapiDefaultHandler) GetMapiV2TxStatusId(ctx echo.Context, id string) error {

	tx, _, err := m.getTransaction(ctx, id)
	if err != nil {
		return err
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
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

	tx, txHex, err := m.getTransaction(ctx, id)
	if err != nil {
		return err
	}

	if txHex == "" {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
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

	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		return err
	}

	// set the globals for all transactions in this request
	transactionOptions := getTransactionOptions(params)

	var wg sync.WaitGroup
	var transactions []interface{}
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		var body []byte
		body, err = io.ReadAll(ctx.Request().Body)
		if err != nil {
			errStr := err.Error()
			e := mapi.ErrBadRequest
			e.ExtraInfo = &errStr
			return ctx.JSON(http.StatusBadRequest, e)
		}

		txString := strings.Replace(string(body), "\r", "", -1)
		txs := strings.Split(txString, "\n")
		transactions = make([]interface{}, len(txs))

		for index, tx := range txs {
			// process all the transactions in parallel
			wg.Add(1)
			go func(index int, tx string) {
				defer wg.Done()
				transactions[index] = m.getTransactionResponse(ctx, tx, transactionOptions, user)
			}(index, tx)
		}
		wg.Wait()
	case "application/json":
		var body []byte
		body, err = io.ReadAll(ctx.Request().Body)
		if err != nil {
			errStr := err.Error()
			e := mapi.ErrBadRequest
			e.ExtraInfo = &errStr
			return ctx.JSON(http.StatusBadRequest, e)
		}

		var txHex []string
		if err = json.Unmarshal(body, &txHex); err != nil {
			return ctx.JSON(mapi.ErrStatusMalformed, mapi.ErrBadRequest)
		}

		transactions = make([]interface{}, len(txHex))
		for index, tx := range txHex {
			// process all the transactions in parallel
			wg.Add(1)
			go func(index int, tx string) {
				defer wg.Done()
				transactions[index] = m.getTransactionResponse(ctx, tx, transactionOptions, user)
			}(index, tx)
		}
		wg.Wait()
	case "application/octet-stream":
		reader := ctx.Request().Body
		btTx := new(bt.Tx)

		transactions = make([]interface{}, 0)
		var bytesRead int64
		limit := make(chan bool, 64) // TODO make configurable how many concurrent process routines we can start
		for {
			limit <- true

			if bytesRead, err = btTx.ReadFrom(reader); err != nil {
				if !errors.Is(err, io.ErrShortBuffer) {
					e := mapi.ErrBadRequest
					errStr := err.Error()
					e.ExtraInfo = &errStr
					return ctx.JSON(mapi.ErrBadRequest.Status, e)
				}
			}
			if bytesRead == 0 {
				// no more transaction data found, stop the loop
				break
			}

			var mu sync.Mutex
			wg.Add(1)
			go func(btTx *bt.Tx) {
				defer func() {
					<-limit
					wg.Done()
				}()

				var transaction *models.Transaction
				if transaction, err = models.NewTransactionFromBytes(btTx.Bytes()); err != nil {
					e := mapi.ErrGeneric
					errStr := err.Error()
					e.ExtraInfo = &errStr
					mu.Lock()
					transactions = append(transactions, e)
					mu.Unlock()
				} else {
					transaction.CallbackURL = transactionOptions.CallbackURL
					transaction.CallbackToken = transactionOptions.CallbackToken
					transaction.MerkleProof = transactionOptions.MerkleProof
					transaction.ClientID = user.ClientID
					_, response, responseError := m.processTransaction(ctx, transaction)
					if responseError != nil {
						// what to do here, the transaction failed due to server failure?
						if response == nil {
							e := mapi.ErrGeneric
							errStr := responseError.Error()
							e.ExtraInfo = &errStr
							mu.Lock()
							transactions = append(transactions, e)
							mu.Unlock()
						}
					} else {
						mu.Lock()
						transactions = append(transactions, response)
						mu.Unlock()
					}
				}
			}(btTx)
		}
		wg.Wait()
	default:
		return ctx.JSON(mapi.ErrBadRequest.Status, mapi.ErrBadRequest)
	}

	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission
	return ctx.JSON(http.StatusOK, transactions)
}

func (m MapiDefaultHandler) getTransactionResponse(ctx echo.Context, tx string, transactionOptions TransactionOptions, user *mapi.User) interface{} {

	transaction, err := models.NewTransactionFromHex(tx, models.WithClient(m.Client), models.New())
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrMalformed
		e.ExtraInfo = &errStr
		return e
	}

	transaction.CallbackURL = transactionOptions.CallbackURL
	transaction.CallbackToken = transactionOptions.CallbackToken
	transaction.MerkleProof = transactionOptions.MerkleProof
	transaction.ClientID = user.ClientID
	_, response, responseError := m.processTransaction(ctx, transaction)
	if responseError != nil {
		// what to do here, the transaction failed due to server failure?
		if response == nil {
			e := mapi.ErrGeneric
			errStr := responseError.Error()
			e.ExtraInfo = &errStr
			return e
		}
	}

	return response
}

func getTransactionOptions(params mapi.PostMapiV2TxsParams) TransactionOptions {
	transactionOptions := TransactionOptions{}
	if params.XCallbackUrl != nil {
		transactionOptions.CallbackURL = *params.XCallbackUrl
		if params.XCallbackToken != nil {
			transactionOptions.CallbackToken = *params.XCallbackToken
		}
	}
	if params.XMerkleProof != nil {
		if *params.XMerkleProof == "true" || *params.XMerkleProof == "1" {
			transactionOptions.MerkleProof = true
		}
	}
	return transactionOptions
}

func (m MapiDefaultHandler) processTransaction(ctx echo.Context, transaction *models.Transaction) (int, interface{}, error) {

	var status int

	// Check whether we have already processed this transaction and saved it in our DB
	// no need to do any further validation. If it is not processed yet, it will be.
	existingTransaction, err := models.GetTransaction(ctx.Request().Context(), transaction.ID, models.WithClient(m.Client))
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
			if existingTransaction.ErrExtraInfo != "" {
				mapiError.ExtraInfo = &existingTransaction.ErrExtraInfo
			}
			return status, mapiError, nil
		}

		return status, mapi.TransactionResponse{
			ApiVersion:  mapi.APIVersion,
			BlockHash:   &existingTransaction.BlockHash,
			BlockHeight: &existingTransaction.BlockHeight,
			MinerId:     m.Client.GetMinerID(),
			Timestamp:   time.Now(),
			Txid:        &existingTransaction.ID,
		}, nil
	}

	if status, err = transaction.Validate(ctx.Request().Context()); err != nil || status >= 400 {
		return m.handleError(ctx, status, transaction, err)
	}

	// the transaction has been validated, store in our DB and continue processing
	transaction.Status = models.TransactionStatusValid
	if err = transaction.Save(ctx.Request().Context()); err != nil {
		return 0, nil, err
	}

	// now that the transaction is stored in our DB, fire it off to the mempool and try to get it mined
	// this should return a 200 or 201 if 1 or more of the nodes accept the transaction
	var conflictedWith []string
	status, conflictedWith, err = transaction.Submit()
	if status >= 400 || err != nil {
		return m.handleError(ctx, status, transaction, err)
	}

	return status, mapi.TransactionResponse{
		ApiVersion:     mapi.APIVersion,
		BlockHash:      &transaction.BlockHash,
		BlockHeight:    &transaction.BlockHeight,
		ConflictedWith: &conflictedWith,
		MinerId:        m.Client.GetMinerID(),
		Timestamp:      time.Now(),
		Txid:           &transaction.ID,
	}, nil
}

func (m MapiDefaultHandler) getTransaction(ctx echo.Context, id string) (*models.Transaction, string, error) {

	tx, err := models.GetTransaction(ctx.Request().Context(), id, models.WithClient(m.Client))
	if err != nil && !errors.Is(err, datastore.ErrNoResults) {
		return nil, "", err
	}

	var txHex string
	if tx == nil {
		var rawTx *bitcoin.RawTransaction
		if rawTx, err = m.Client.GetTransactionFromNodes(id); err != nil {
			return nil, "", err
		}
		if rawTx == nil {
			return nil, "", nil
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

	return tx, txHex, nil
}

func (m MapiDefaultHandler) handleError(ctx echo.Context, status int, transaction *models.Transaction, submitErr error) (int, interface{}, error) {

	// enrich the response with the error details
	mapiError := mapi.ErrByStatus[status]
	if mapiError == nil {
		return mapi.ErrGeneric.Status, mapi.ErrGeneric, nil
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
		return 0, nil, err
	}

	if transaction != nil {
		transaction.Status = models.TransactionStatusError
		transaction.ErrStatus = mapiError.Status
		transaction.ErrInstanceID = logError.ID
		if mapiError.ExtraInfo != nil {
			transaction.ErrExtraInfo = *mapiError.ExtraInfo
		}
		if err := transaction.Save(ctx.Request().Context()); err != nil {
			return 0, nil, err
		}
	}

	return status, mapiError, nil
}
