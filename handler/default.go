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

	"github.com/TAAL-GmbH/arc"
	"github.com/TAAL-GmbH/arc/client"
	"github.com/TAAL-GmbH/arc/models"
	"github.com/TAAL-GmbH/arc/validator"
	defaultValidator "github.com/TAAL-GmbH/arc/validator/default"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/mrz1836/go-datastore"
)

type ArcDefaultHandler struct {
	Client  client.Interface
	Options handlerOptions
}

func NewDefault(c client.Interface, opts ...Options) (arc.HandlerInterface, error) {
	bitcoinHandler := &ArcDefaultHandler{Client: c}

	for _, opt := range opts {
		opt(&bitcoinHandler.Options)
	}

	return bitcoinHandler, nil
}

// GetArcV1Policy ...
func (m ArcDefaultHandler) GetArcV1Fees(ctx echo.Context) error {
	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(arc.ErrStatusGeneric), e)
	}

	var fees *arc.FeesResponse
	fees, err = getFees(ctx, m.Client, user.ClientID)
	if err != nil {
		status, response, responseErr := m.handleError(ctx, nil, err)
		if responseErr != nil {
			// if an error is returned, the processing failed and we should return a 500 error
			return responseErr
		}
		return ctx.JSON(int(status), response)
	}

	if fees == nil {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}

	return ctx.JSON(http.StatusOK, fees)
}

// PostArcV2Tx ...
func (m ArcDefaultHandler) PostArcV1Tx(ctx echo.Context, params arc.PostArcV1TxParams) error {
	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(arc.ErrStatusGeneric), e)
	}

	var body []byte
	body, err = io.ReadAll(ctx.Request().Body)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrBadRequest
		e.ExtraInfo = &errStr
		return ctx.JSON(http.StatusBadRequest, e)
	}

	var transaction *bt.Tx
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		if transaction, err = bt.NewTxFromString(string(body)); err != nil {
			errStr := err.Error()
			e := arc.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(arc.ErrStatusMalformed), e)
		}
	case "application/json":
		var txHex string
		if err = json.Unmarshal(body, &txHex); err != nil {
			errStr := err.Error()
			e := arc.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(arc.ErrStatusMalformed), e)
		}

		if transaction, err = bt.NewTxFromString(txHex); err != nil {
			errStr := err.Error()
			e := arc.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(arc.ErrStatusMalformed), e)
		}
	case "application/octet-stream":
		if transaction, err = bt.NewTxFromBytes(body); err != nil {
			errStr := err.Error()
			e := arc.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(arc.ErrStatusMalformed), e)
		}
	default:
		return ctx.JSON(arc.ErrBadRequest.Status, arc.ErrBadRequest)
	}
	transactionOptions := &arc.TransactionOptions{
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

// GetArcV2TxStatusId ...
func (m ArcDefaultHandler) GetArcV1TxStatusId(ctx echo.Context, id string) error {

	tx, err := m.getTransactionStatus(ctx.Request().Context(), id)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(arc.ErrStatusGeneric), e)
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, arc.ErrNotFound)
	}

	return ctx.JSON(http.StatusOK, arc.TransactionStatus{
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    &tx.Status,
		Timestamp:   time.Now(),
		Txid:        tx.TxID,
	})
}

// GetArcV2TxId Similar to GetArcV2TxStatusId, but also returns the whole transaction
func (m ArcDefaultHandler) GetArcV1TxId(ctx echo.Context, id string) error {

	tx, err := m.getTransaction(ctx.Request().Context(), id)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(arc.ErrStatusGeneric), e)
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, arc.ErrNotFound)
	}

	return ctx.JSON(http.StatusOK, arc.TransactionResponse{
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		// TODO TxStatus:    &tx.Status,
		Timestamp: time.Now(),
		Txid:      &tx.TxID,
	})
}

// PostArcV2Txs ...
func (m ArcDefaultHandler) PostArcV1Txs(ctx echo.Context, params arc.PostArcV1TxsParams) error {

	user, err := GetEchoUser(ctx, m.Options.security)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(arc.ErrStatusGeneric), e)
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
			e := arc.ErrBadRequest
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
			e := arc.ErrBadRequest
			e.ExtraInfo = &errStr
			return ctx.JSON(http.StatusBadRequest, e)
		}

		var txHex []string
		if err = json.Unmarshal(body, &txHex); err != nil {
			return ctx.JSON(int(arc.ErrStatusMalformed), arc.ErrBadRequest)
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
					e := arc.ErrBadRequest
					errStr := err.Error()
					e.ExtraInfo = &errStr
					return ctx.JSON(arc.ErrBadRequest.Status, e)
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
						e := arc.ErrGeneric
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
		return ctx.JSON(arc.ErrBadRequest.Status, arc.ErrBadRequest)
	}

	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission
	return ctx.JSON(http.StatusOK, transactions)
}

func (m ArcDefaultHandler) getTransactionResponse(ctx echo.Context, tx string, transactionOptions *arc.TransactionOptions) interface{} {
	transaction, err := bt.NewTxFromString(tx)
	if err != nil {
		errStr := err.Error()
		e := arc.ErrMalformed
		e.ExtraInfo = &errStr
		return e
	}

	_, response, responseError := m.processTransaction(ctx, transaction, transactionOptions)
	if responseError != nil {
		// what to do here, the transaction failed due to server failure?
		e := arc.ErrGeneric
		errStr := responseError.Error()
		e.ExtraInfo = &errStr
		return e
	}

	return response
}

func getTransactionOptions(params arc.PostArcV1TxsParams) *arc.TransactionOptions {
	transactionOptions := &arc.TransactionOptions{}
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

func (m ArcDefaultHandler) processTransaction(ctx echo.Context, transaction *bt.Tx, transactionOptions *arc.TransactionOptions) (arc.ErrorCode, interface{}, error) {
	policy, err := getFees(ctx, m.Client, transactionOptions.ClientID)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	txValidator := defaultValidator.New(policy)
	if err = txValidator.ValidateTransaction(transaction); err != nil {
		return m.handleError(ctx, transaction, err)
	}

	// now that we have validated the transaction, fire it off to the mempool and try to get it mined
	// this should return a 200 or 201 if 1 or more of the nodes accept the transaction
	node := m.Client.GetRandomNode()
	var tx *client.TransactionStatus
	tx, err = node.SubmitTransaction(ctx.Request().Context(), transaction.Bytes(), transactionOptions)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	// TODO differentiate between 200 and 201
	return arc.StatusAddedBlockTemplate, arc.TransactionResponse{
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		// TODO TxStatus:    &tx.Status,
		Timestamp: time.Now(),
		Txid:      &tx.TxID,
	}, nil
}

func (m ArcDefaultHandler) getTransaction(ctx context.Context, id string) (*client.RawTransaction, error) {
	node := m.Client.GetRandomNode()

	tx, err := node.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (*client.TransactionStatus, error) {
	node := m.Client.GetRandomNode()

	tx, err := node.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m ArcDefaultHandler) handleError(_ echo.Context, transaction *bt.Tx, submitErr error) (arc.ErrorCode, interface{}, error) {
	status := arc.ErrStatusGeneric
	isArcError, ok := submitErr.(*validator.Error)
	if ok {
		status = isArcError.ArcErrorStatus
	}

	// enrich the response with the error details
	arcError := arc.ErrByStatus[status]
	if arcError == nil {
		return arc.ErrStatusGeneric, arc.ErrGeneric, nil
	}

	if transaction != nil {
		txID := transaction.TxID()
		arcError.Txid = &txID
	}
	if submitErr != nil {
		extraInfo := submitErr.Error()
		arcError.ExtraInfo = &extraInfo
	}

	/* TODO log error
	logError := models.NewLogError(models.WithClient(m.Client), models.New())
	logError.Error = arcError
	logError.TxID = transaction.ID

	if accessLog, ok := ctx.Get("access_log").(*models.LogAccess); ok {
		logError.LogAccessID = accessLog.ID
		logError.ClientID = accessLog.ClientID
	}
	*/

	return status, arcError, nil
}

func getFees(ctx echo.Context, c client.Interface, clientID string) (*arc.FeesResponse, error) {
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
		// TODO change to arc error
		return nil, validator.NewError(fmt.Errorf("no policy found for client"), http.StatusNotFound)
	}

	feesResponse := &arc.FeesResponse{
		Timestamp: time.Now(),
	}

	fees := make([]arc.Fee, len(policy.Fees))
	for index, fee := range policy.Fees {
		fees[index] = fee
	}
	feesResponse.Fees = &fees

	return feesResponse, nil
}
