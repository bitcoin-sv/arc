package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/mapi"
	"github.com/TAAL-GmbH/mapi/client"
	"github.com/TAAL-GmbH/mapi/models"
	"github.com/TAAL-GmbH/mapi/validator"
	defaultValidator "github.com/TAAL-GmbH/mapi/validator/default"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/mrz1836/go-datastore"
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
		errStr := err.Error()
		e := mapi.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(mapi.ErrStatusGeneric), e)
	}

	var policy *mapi.Policy
	policy, err = getPolicy(ctx, m.Client, user.ClientID)
	if err != nil {
		status, response, responseErr := m.handleError(ctx, nil, err)
		if responseErr != nil {
			// if an error is returned, the processing failed and we should return a 500 error
			return responseErr
		}
		return ctx.JSON(int(status), response)
	}

	if policy == nil {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}

	return ctx.JSON(http.StatusOK, policy)
}

// PostMapiV2Tx ...
func (m MapiDefaultHandler) PostMapiV2Tx(ctx echo.Context, params mapi.PostMapiV2TxParams) error {
	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(mapi.ErrStatusGeneric), e)
	}

	var body []byte
	body, err = io.ReadAll(ctx.Request().Body)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrBadRequest
		e.ExtraInfo = &errStr
		return ctx.JSON(http.StatusBadRequest, e)
	}

	var transaction *bt.Tx
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		if transaction, err = bt.NewTxFromString(string(body)); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(mapi.ErrStatusMalformed), e)
		}
	case "application/json":
		var txHex string
		if err = json.Unmarshal(body, &txHex); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(mapi.ErrStatusMalformed), e)
		}

		if transaction, err = bt.NewTxFromString(txHex); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(mapi.ErrStatusMalformed), e)
		}
	case "application/octet-stream":
		if transaction, err = bt.NewTxFromBytes(body); err != nil {
			errStr := err.Error()
			e := mapi.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(mapi.ErrStatusMalformed), e)
		}
	default:
		return ctx.JSON(mapi.ErrBadRequest.Status, mapi.ErrBadRequest)
	}
	transactionOptions := &mapi.TransactionOptions{
		ClientID: user.ClientID,
	}
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

	status, response, responseErr := m.processTransaction(ctx, transaction, transactionOptions)
	if responseErr != nil {
		// if an error is returned, the processing failed and we should return a 500 error
		return responseErr
	}

	return ctx.JSON(int(status), response)
}

// GetMapiV2TxStatusId ...
func (m MapiDefaultHandler) GetMapiV2TxStatusId(ctx echo.Context, id string) error {

	tx, err := m.getTransactionStatus(ctx.Request().Context(), id)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(mapi.ErrStatusGeneric), e)
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, mapi.ErrNotFound)
	}

	return ctx.JSON(http.StatusOK, mapi.TransactionStatus{
		ApiVersion:  mapi.APIVersion,
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		MinerId:     m.Client.GetMinerID(),
		TxStatus:    &tx.Status,
		Timestamp:   time.Now(),
		Txid:        tx.TxID,
	})
}

// GetMapiV2TxId Similar to GetMapiV2TxStatusId, but also returns the whole transaction
func (m MapiDefaultHandler) GetMapiV2TxId(ctx echo.Context, id string) error {

	tx, err := m.getTransaction(ctx.Request().Context(), id)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(mapi.ErrStatusGeneric), e)
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, mapi.ErrNotFound)
	}

	return ctx.JSON(http.StatusOK, mapi.Transaction{
		ApiVersion:  mapi.APIVersion,
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		MinerId:     m.Client.GetMinerID(),
		TxStatus:    &tx.Status,
		Timestamp:   time.Now(),
		Tx:          tx.Hex,
		Txid:        tx.TxID,
	})
}

// PostMapiV2Txs ...
func (m MapiDefaultHandler) PostMapiV2Txs(ctx echo.Context, params mapi.PostMapiV2TxsParams) error {

	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(mapi.ErrStatusGeneric), e)
	}

	// set the globals for all transactions in this request
	transactionOptions := getTransactionOptions(params)
	transactionOptions.ClientID = user.ClientID

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

		txString := strings.ReplaceAll(string(body), "\r", "")
		txs := strings.Split(txString, "\n")
		transactions = make([]interface{}, len(txs))

		for index, tx := range txs {
			// process all the transactions in parallel
			wg.Add(1)
			go func(index int, tx string) {
				defer wg.Done()
				transactions[index] = m.getTransactionResponse(ctx, tx, transactionOptions)
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
			return ctx.JSON(int(mapi.ErrStatusMalformed), mapi.ErrBadRequest)
		}

		transactions = make([]interface{}, len(txHex))
		for index, tx := range txHex {
			// process all the transactions in parallel
			wg.Add(1)
			go func(index int, tx string) {
				defer wg.Done()
				transactions[index] = m.getTransactionResponse(ctx, tx, transactionOptions)
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
			go func(transaction *bt.Tx) {
				defer func() {
					<-limit
					wg.Done()
				}()

				_, response, responseError := m.processTransaction(ctx, transaction, transactionOptions)
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

func (m MapiDefaultHandler) getTransactionResponse(ctx echo.Context, tx string, transactionOptions *mapi.TransactionOptions) interface{} {
	transaction, err := bt.NewTxFromString(tx)
	if err != nil {
		errStr := err.Error()
		e := mapi.ErrMalformed
		e.ExtraInfo = &errStr
		return e
	}

	_, response, responseError := m.processTransaction(ctx, transaction, transactionOptions)
	if responseError != nil {
		// what to do here, the transaction failed due to server failure?
		e := mapi.ErrGeneric
		errStr := responseError.Error()
		e.ExtraInfo = &errStr
		return e
	}

	return response
}

func getTransactionOptions(params mapi.PostMapiV2TxsParams) *mapi.TransactionOptions {
	transactionOptions := &mapi.TransactionOptions{}
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

func (m MapiDefaultHandler) processTransaction(ctx echo.Context, transaction *bt.Tx, transactionOptions *mapi.TransactionOptions) (mapi.ErrStatus, interface{}, error) {
	policy, err := getPolicy(ctx, m.Client, transactionOptions.ClientID)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	txValidator := defaultValidator.New(policy)
	if err = txValidator.ValidateTransaction(transaction); err != nil {
		return m.handleError(ctx, transaction, err)
	}

	// now that we have validated the transaction, fire it off to the mempool and try to get it mined
	// this should return a 200 or 201 if 1 or more of the nodes accept the transaction
	var conflictedWith []string
	node := m.Client.GetRandomNode()
	var tx *client.TransactionStatus
	tx, err = node.SubmitTransaction(ctx.Request().Context(), transaction.Bytes(), transactionOptions)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	// TODO differentiate between 200 and 201
	return mapi.StatusAddedBlockTemplate, mapi.TransactionResponse{
		ApiVersion:     mapi.APIVersion,
		BlockHash:      &tx.BlockHash,
		BlockHeight:    &tx.BlockHeight,
		ConflictedWith: &conflictedWith,
		MinerId:        m.Client.GetMinerID(),
		TxStatus:       &tx.Status,
		Timestamp:      time.Now(),
		Txid:           &tx.TxID,
	}, nil
}

func (m MapiDefaultHandler) getTransaction(ctx context.Context, id string) (*client.RawTransaction, error) {
	node := m.Client.GetRandomNode()

	tx, err := node.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m MapiDefaultHandler) getTransactionStatus(ctx context.Context, id string) (*client.TransactionStatus, error) {
	node := m.Client.GetRandomNode()

	tx, err := node.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m MapiDefaultHandler) handleError(_ echo.Context, transaction *bt.Tx, submitErr error) (mapi.ErrStatus, interface{}, error) {
	status := mapi.ErrStatusGeneric
	isMapiError, ok := submitErr.(*validator.Error)
	if ok {
		status = isMapiError.MapiErrorStatus
	}

	// enrich the response with the error details
	mapiError := mapi.ErrByStatus[status]
	if mapiError == nil {
		return mapi.ErrStatusGeneric, mapi.ErrGeneric, nil
	}

	if transaction != nil {
		txID := transaction.TxID()
		mapiError.Txid = &txID
	}
	if submitErr != nil {
		extraInfo := submitErr.Error()
		mapiError.ExtraInfo = &extraInfo
	}

	/* TODO log error
	logError := models.NewLogError(models.WithClient(m.Client), models.New())
	logError.Error = mapiError
	logError.TxID = transaction.ID

	if accessLog, ok := ctx.Get("access_log").(*models.LogAccess); ok {
		logError.LogAccessID = accessLog.ID
		logError.ClientID = accessLog.ClientID
	}
	*/

	return status, mapiError, nil
}

func getPolicy(ctx echo.Context, c client.Interface, clientID string) (*mapi.Policy, error) {
	// TODO add caching for the clientID, so we don't have to query the DB for every request

	policy, err := models.GetPolicyForClient(ctx.Request().Context(), clientID, models.WithClient(c))
	if err != nil && !errors.Is(err, datastore.ErrNoResults) {
		return nil, err
	}
	if policy == nil {
		policy, err = models.GetDefaultPolicy(ctx.Request().Context(), models.WithClient(c))
		if err != nil {
			if errors.Is(err, datastore.ErrNoResults) {
				return nil, validator.NewError(fmt.Errorf("no policy found for client"), http.StatusNotFound)
			}
			return nil, err
		}
	}

	if policy == nil {
		// TODO change to mapi error
		return nil, validator.NewError(fmt.Errorf("no policy found for client"), http.StatusNotFound)
	}

	mapiPolicy := &mapi.Policy{
		ApiVersion: mapi.APIVersion,
		ExpiryTime: time.Now().Add(30 * time.Second), // TODO get from config
		MinerId:    c.GetMinerID(),
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

	return mapiPolicy, nil
}
