package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/TAAL-GmbH/arc/validator"
	defaultValidator "github.com/TAAL-GmbH/arc/validator/default"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
)

type ArcDefaultHandler struct {
	TransactionHandler transactionHandler.TransactionHandler
}

func NewDefault(transactionHandler transactionHandler.TransactionHandler) (api.HandlerInterface, error) {
	bitcoinHandler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
	}

	return bitcoinHandler, nil
}

// GetArcV1Fees ...
func (m ArcDefaultHandler) GetArcV1Fees(ctx echo.Context) error {
	fees, err := getFees(ctx)
	if err != nil {
		status, response, responseErr := m.handleError(ctx, nil, err)
		if responseErr != nil {
			// if an error is returned, the processing failed, and we should return a 500 error
			return responseErr
		}
		return ctx.JSON(int(status), response)
	}

	if fees == nil {
		return echo.NewHTTPError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}

	return ctx.JSON(http.StatusOK, fees)
}

// PostArcV1Tx ...
func (m ArcDefaultHandler) PostArcV1Tx(ctx echo.Context, params api.PostArcV1TxParams) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		errStr := err.Error()
		e := api.ErrBadRequest
		e.ExtraInfo = &errStr
		return ctx.JSON(http.StatusBadRequest, e)
	}

	var transaction *bt.Tx
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		if transaction, err = bt.NewTxFromString(string(body)); err != nil {
			errStr := err.Error()
			e := api.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(api.ErrStatusMalformed), e)
		}
	case "application/json":
		var txHex string
		var txBody api.PostArcV1TxJSONBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			errStr := err.Error()
			e := api.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(api.ErrStatusMalformed), e)
		}
		txHex = txBody.RawTx

		if transaction, err = bt.NewTxFromString(txHex); err != nil {
			errStr := err.Error()
			e := api.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(api.ErrStatusMalformed), e)
		}
	case "application/octet-stream":
		if transaction, err = bt.NewTxFromBytes(body); err != nil {
			errStr := err.Error()
			e := api.ErrMalformed
			e.ExtraInfo = &errStr
			return ctx.JSON(int(api.ErrStatusMalformed), e)
		}
	default:
		return ctx.JSON(api.ErrBadRequest.Status, api.ErrBadRequest)
	}
	transactionOptions := &api.TransactionOptions{}
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
		// if an error is returned, the processing failed, and we should return a 500 error
		return responseErr
	}

	return ctx.JSON(int(status), response)
}

// GetArcV1TxId Similar to GetArcV2TxStatusId, but also returns the whole transaction
func (m ArcDefaultHandler) GetArcV1TxId(ctx echo.Context, id string) error {

	tx, err := m.getTransactionStatus(ctx.Request().Context(), id)
	if err != nil {
		errStr := err.Error()
		e := api.ErrGeneric
		e.ExtraInfo = &errStr
		return ctx.JSON(int(api.ErrStatusGeneric), e)
	}

	if tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, api.ErrNotFound)
	}

	return ctx.JSON(http.StatusOK, api.TransactionStatus{
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    &tx.Status,
		Timestamp:   time.Now(),
		Txid:        tx.TxID,
	})
}

// PostArcV1Txs ...
func (m ArcDefaultHandler) PostArcV1Txs(ctx echo.Context, params api.PostArcV1TxsParams) error {

	// set the globals for all transactions in this request
	transactionOptions := getTransactionOptions(params)

	var wg sync.WaitGroup
	var transactions []interface{}
	switch ctx.Request().Header.Get("Content-Type") {
	case "text/plain":
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			errStr := err.Error()
			e := api.ErrBadRequest
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
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			errStr := err.Error()
			e := api.ErrBadRequest
			e.ExtraInfo = &errStr
			return ctx.JSON(http.StatusBadRequest, e)
		}

		var txHex []string
		if err = json.Unmarshal(body, &txHex); err != nil {
			return ctx.JSON(int(api.ErrStatusMalformed), api.ErrBadRequest)
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
		var err error
		limit := make(chan bool, 64) // TODO make configurable how many concurrent process routines we can start
		for {
			limit <- true

			if bytesRead, err = btTx.ReadFrom(reader); err != nil {
				if !errors.Is(err, io.ErrShortBuffer) {
					e := api.ErrBadRequest
					errStr := err.Error()
					e.ExtraInfo = &errStr
					return ctx.JSON(api.ErrBadRequest.Status, e)
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
					e := api.ErrGeneric
					errStr := responseError.Error()
					e.ExtraInfo = &errStr
					mu.Lock()
					transactions = append(transactions, e)
					mu.Unlock()
				} else {
					mu.Lock()
					transactions = append(transactions, response)
					mu.Unlock()
				}
			}(btTx)
		}
		wg.Wait()
	default:
		return ctx.JSON(api.ErrBadRequest.Status, api.ErrBadRequest)
	}

	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission
	return ctx.JSON(http.StatusOK, transactions)
}

func (m ArcDefaultHandler) getTransactionResponse(ctx echo.Context, tx string, transactionOptions *api.TransactionOptions) interface{} {
	transaction, err := bt.NewTxFromString(tx)
	if err != nil {
		errStr := err.Error()
		e := api.ErrMalformed
		e.ExtraInfo = &errStr
		return e
	}

	_, response, responseError := m.processTransaction(ctx, transaction, transactionOptions)
	if responseError != nil {
		// what to do here, the transaction failed due to server failure?
		e := api.ErrGeneric
		errStr := responseError.Error()
		e.ExtraInfo = &errStr
		return e
	}

	return response
}

func getTransactionOptions(params api.PostArcV1TxsParams) *api.TransactionOptions {
	transactionOptions := &api.TransactionOptions{}
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

func (m ArcDefaultHandler) processTransaction(ctx echo.Context, transaction *bt.Tx, transactionOptions *api.TransactionOptions) (api.StatusCode, interface{}, error) {
	fees, err := getFees(ctx)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	txValidator := defaultValidator.New(fees)
	if err = txValidator.ValidateTransaction(transaction); err != nil {
		return m.handleError(ctx, transaction, err)
	}

	var tx *transactionHandler.TransactionStatus
	tx, err = m.TransactionHandler.SubmitTransaction(ctx.Request().Context(), transaction.Bytes(), transactionOptions)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	// TODO differentiate between 200 and 201
	return api.StatusAddedBlockTemplate, api.TransactionResponse{
		Status:      int(api.StatusAddedBlockTemplate),
		Title:       api.StatusText[api.StatusAddedBlockTemplate],
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    (*api.TransactionResponseTxStatus)(&tx.Status),
		Timestamp:   time.Now(),
		Txid:        &tx.TxID,
	}, nil
}

func (m ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (*transactionHandler.TransactionStatus, error) {
	tx, err := m.TransactionHandler.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m ArcDefaultHandler) handleError(_ echo.Context, transaction *bt.Tx, submitErr error) (api.StatusCode, interface{}, error) {
	status := api.ErrStatusGeneric
	isArcError, ok := submitErr.(*validator.Error)
	if ok {
		status = isArcError.ArcErrorStatus
	}

	// enrich the response with the error details
	arcError := api.ErrByStatus[status]
	if arcError == nil {
		return api.ErrStatusGeneric, api.ErrGeneric, nil
	}

	if transaction != nil {
		txID := transaction.TxID()
		arcError.Txid = &txID
	}
	if submitErr != nil {
		extraInfo := submitErr.Error()
		arcError.ExtraInfo = &extraInfo
	}

	return status, arcError, nil
}

func getFees(_ echo.Context) (*api.FeesResponse, error) {

	// TODO get from node
	defaultFees := []api.Fee{
		{
			FeeType: "data",
			MiningFee: api.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
			RelayFee: api.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
		},
		{
			FeeType: "standard",
			MiningFee: api.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
			RelayFee: api.FeeAmount{
				Satoshis: 5,
				Bytes:    1000,
			},
		},
	}

	feesResponse := &api.FeesResponse{
		Timestamp: time.Now(),
		Fees:      &defaultFees,
	}

	return feesResponse, nil
}
