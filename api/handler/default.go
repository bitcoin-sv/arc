package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/validator"
	defaultValidator "github.com/TAAL-GmbH/arc/validator/default"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

type ArcDefaultHandler struct {
	TransactionHandler transactionHandler.TransactionHandler
}

func NewDefault(transactionHandler transactionHandler.TransactionHandler) (api.HandlerInterface, error) {
	handler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
	}

	return handler, nil
}

// GETFees ...
func (m ArcDefaultHandler) GETFees(ctx echo.Context) error {
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

// POSTTransaction ...
func (m ArcDefaultHandler) POSTTransaction(ctx echo.Context, params api.POSTTransactionParams) error {
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
		var txBody api.POSTTransactionJSONBody
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

	transactionOptions := getTransactionOptions(params)

	status, response, responseErr := m.processTransaction(ctx, transaction, transactionOptions)
	if responseErr != nil {
		// if an error is returned, the processing failed, and we should return a 500 error
		return responseErr
	}

	return ctx.JSON(int(status), response)
}

// GETTransactionStatus ...
func (m ArcDefaultHandler) GETTransactionStatus(ctx echo.Context, id string) error {

	tx, err := m.getTransactionStatus(ctx.Request().Context(), id)
	if err != nil {
		if errors.Is(err, transactionHandler.ErrTransactionNotFound) {
			e := api.ErrNotFound
			e.Detail = err.Error()
			return echo.NewHTTPError(http.StatusNotFound, e)
		}

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

// POSTTransactions ...
func (m ArcDefaultHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) error {

	// set the globals for all transactions in this request
	transactionOptions := getTransactionsOptions(params)

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

func getTransactionOptions(params api.POSTTransactionParams) *api.TransactionOptions {
	return getTransactionsOptions(api.POSTTransactionsParams(params))
}

func getTransactionsOptions(params api.POSTTransactionsParams) *api.TransactionOptions {
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
	if params.XWaitForStatus != nil {
		if *params.XWaitForStatus >= 2 && *params.XWaitForStatus <= 6 {
			transactionOptions.WaitForStatus = metamorph_api.Status(*params.XWaitForStatus)
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

	// the validator expects an extended transaction
	// we must enrich the transaction with the missing data
	if !txValidator.IsExtended(transaction) {
		err = m.extendTransaction(ctx, transaction)
		if err != nil {
			return m.handleError(ctx, transaction, err)
		}
	}

	if err = txValidator.ValidateTransaction(transaction); err != nil {
		return m.handleError(ctx, transaction, err)
	}

	var tx *transactionHandler.TransactionStatus
	tx, err = m.TransactionHandler.SubmitTransaction(ctx.Request().Context(), transaction.Bytes(), transactionOptions)
	if err != nil {
		return m.handleError(ctx, transaction, err)
	}

	txID := tx.TxID
	if txID == "" {
		txID = transaction.TxID()
	}

	// TODO differentiate between 200 and 201
	return api.StatusAddedBlockTemplate, api.TransactionResponse{
		Status:      int(api.StatusAddedBlockTemplate),
		Title:       api.StatusText[api.StatusAddedBlockTemplate],
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    (*api.TransactionResponseTxStatus)(&tx.Status),
		Timestamp:   time.Now(),
		Txid:        &txID,
	}, nil
}

func (m ArcDefaultHandler) extendTransaction(ctx echo.Context, transaction *bt.Tx) (err error) {
	parentTxBytes := make(map[string][]byte)
	var btParentTx *bt.Tx

	// get the missing input data for the transaction
	for _, input := range transaction.Inputs {
		parentTxIDStr := input.PreviousTxIDStr()
		b, ok := parentTxBytes[parentTxIDStr]
		if !ok {
			b, err = m.getTransaction(ctx.Request().Context(), parentTxIDStr)
			if err != nil {
				return err
			}
			parentTxBytes[parentTxIDStr] = b
		}

		btParentTx, err = bt.NewTxFromBytes(b)
		if err != nil {
			return err
		}

		if len(btParentTx.Outputs) < int(input.PreviousTxOutIndex) {
			return fmt.Errorf("output %d not found in transaction %s", input.PreviousTxOutIndex, parentTxIDStr)
		}
		output := btParentTx.Outputs[input.PreviousTxOutIndex]

		input.PreviousTxScript = output.LockingScript
		input.PreviousTxSatoshis = output.Satoshis
	}

	return nil
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

	if errors.Is(submitErr, transactionHandler.ErrParentTransactionNotFound) {
		status = api.ErrStatusTxFormat
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

// getTransaction returns the transaction with the given id from a store
func (m ArcDefaultHandler) getTransaction(ctx context.Context, inputTxID string) ([]byte, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ArcDefaultHandler:getTransaction")
	defer span.Finish()

	// get from our transaction handler
	txBytes, err := m.TransactionHandler.GetTransaction(ctx, inputTxID)
	if err != nil && !errors.Is(err, transactionHandler.ErrTransactionNotFound) {
		return nil, err
	}
	if txBytes != nil {
		return txBytes, nil
	}

	// get from our node, if configured
	peerURL, _, peerURLFound := gocore.Config().GetURL("peer_rpc")
	if peerURLFound {
		// get the transaction from the bitcoin node rpc
		port, _ := strconv.Atoi(peerURL.Port())
		var node *bitcoin.Bitcoind
		password, _ := peerURL.User.Password()
		node, err = bitcoin.New(peerURL.Hostname(), port, peerURL.User.Username(), password, false)
		if err == nil {
			var tx *bitcoin.RawTransaction
			tx, err = node.GetRawTransaction(inputTxID)
			if err == nil {
				txBytes, err = hex.DecodeString(tx.Hex)
				if err == nil {
					return txBytes, nil
				}
			}
		}
	}

	// get from woc
	wocApiKey, _ := gocore.Config().Get("wocApiKey")
	if wocApiKey != "" {
		wocURL := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/tx/%s/hex", "main", inputTxID)
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, wocURL, nil)
		if err == nil {
			req.Header.Set("Authorization", wocApiKey)

			var resp *http.Response
			resp, err = http.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					return nil, transactionHandler.ErrParentTransactionNotFound
				}

				var txHexBytes []byte
				txHexBytes, err = io.ReadAll(resp.Body)
				if err == nil {
					txHex := string(txHexBytes)
					txBytes, err = hex.DecodeString(txHex)
					if err == nil {
						return txBytes, nil
					}
				}
			}
		}
	}

	return nil, transactionHandler.ErrParentTransactionNotFound
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
