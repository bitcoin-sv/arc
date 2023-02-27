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

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/api/transactionHandler"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/validator"
	defaultValidator "github.com/TAAL-GmbH/arc/validator/default"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

type ArcDefaultHandler struct {
	TransactionHandler transactionHandler.TransactionHandler
	NodePolicy         *api.NodePolicy
	logger             utils.Logger
}

func NewDefault(logger utils.Logger, transactionHandler transactionHandler.TransactionHandler) (api.HandlerInterface, error) {
	policy, err := getPolicy(logger)
	if err != nil {
		logger.Errorf("could not load policy, using default: %v", err)
	}

	handler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
		NodePolicy:         policy,
		logger:             logger,
	}

	return handler, nil
}

func (m ArcDefaultHandler) GETPolicy(ctx echo.Context) error {
	satoshis, bytes := calcFeesFromBSVPerKB(m.NodePolicy.MinMiningTxFee)

	return ctx.JSON(http.StatusOK, api.PolicyResponse{
		Policy: api.Policy{
			Maxscriptsizepolicy:     uint64(m.NodePolicy.MaxScriptSizePolicy),
			Maxtxsigopscountspolicy: uint64(m.NodePolicy.MaxTxSigopsCountsPolicy),
			Maxtxsizepolicy:         uint64(m.NodePolicy.MaxTxSizePolicy),
			MiningFee: api.FeeAmount{
				Bytes:    bytes,
				Satoshis: satoshis,
			},
		},
		Timestamp: time.Now().UTC(),
	})
}

func calcFeesFromBSVPerKB(feePerKB float64) (uint64, uint64) {
	bytes := uint64(1000)
	fSatoshis := feePerKB * 1e8
	satoshis := uint64(fSatoshis)

	// increment bytes and satoshis by a factor of 10 until satoshis is not a fraction
	for fSatoshis != float64(satoshis) {
		fSatoshis *= 10
		satoshis = uint64(fSatoshis)
		bytes *= 10
	}

	return satoshis, bytes
}

// POSTTransaction ...
func (m ArcDefaultHandler) POSTTransaction(ctx echo.Context, params api.POSTTransactionParams) error {
	span, tracingCtx := opentracing.StartSpanFromContext(ctx.Request().Context(), "ArcDefaultHandler:POSTTransactions")
	defer span.Finish()

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

	status, response, responseErr := m.processTransaction(tracingCtx, transaction, transactionOptions)
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
	span, tracingCtx := opentracing.StartSpanFromContext(ctx.Request().Context(), "ArcDefaultHandler:POSTTransactions")
	defer span.Finish()

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

		if len(body) == 0 {
			return ctx.JSON(int(api.ErrStatusMalformed), api.ErrBadRequest)
		}

		txString := strings.TrimSpace(strings.ReplaceAll(string(body), "\r", ""))
		txs := strings.Split(txString, "\n")
		transactions = make([]interface{}, len(txs))

		for index, tx := range txs {
			// process all the transactions in parallel
			wg.Add(1)
			go func(index int, tx string) {
				defer wg.Done()
				transactions[index] = m.getTransactionResponse(tracingCtx, tx, transactionOptions)
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
				transactions[index] = m.getTransactionResponse(tracingCtx, tx, transactionOptions)
			}(index, tx)
		}
		wg.Wait()
	case "application/octet-stream":
		reader := ctx.Request().Body

		transactions = make([]interface{}, 0)
		var bytesRead int64
		var err error
		limit := make(chan bool, 1024) // TODO make configurable how many concurrent process routines we can start
		totalBytesRead := int64(0)
		for {
			limit <- true

			btTx := new(bt.Tx)
			if bytesRead, err = btTx.ReadFrom(reader); err != nil {
				if !errors.Is(err, io.EOF) {
					e := api.ErrBadRequest
					errStr := err.Error()
					e.ExtraInfo = &errStr
					return ctx.JSON(api.ErrBadRequest.Status, e)
				}
			}
			if bytesRead == 0 {
				if totalBytesRead == 0 {
					// no transactions found in the request body
					return ctx.JSON(int(api.ErrStatusMalformed), api.ErrBadRequest)
				}
				// no more transaction data found, stop the loop
				break
			}

			totalBytesRead += bytesRead

			var mu sync.Mutex
			wg.Add(1)
			go func(transaction *bt.Tx) {
				defer func() {
					<-limit
					wg.Done()
				}()

				_, response, responseError := m.processTransaction(tracingCtx, transaction, transactionOptions)
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

func (m ArcDefaultHandler) getTransactionResponse(ctx context.Context, tx string, transactionOptions *api.TransactionOptions) interface{} {
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

func (m ArcDefaultHandler) processTransaction(ctx context.Context, transaction *bt.Tx, transactionOptions *api.TransactionOptions) (api.StatusCode, interface{}, error) {
	span, tracingCtx := opentracing.StartSpanFromContext(ctx, "ArcDefaultHandler:processTransaction")
	defer span.Finish()

	txValidator := defaultValidator.New(m.NodePolicy)

	// the validator expects an extended transaction
	// we must enrich the transaction with the missing data
	if !txValidator.IsExtended(transaction) {
		err := m.extendTransaction(tracingCtx, transaction)
		if err != nil {
			return m.handleError(tracingCtx, transaction, err)
		}
	}

	validateSpan, validateCtx := opentracing.StartSpanFromContext(tracingCtx, "ArcDefaultHandler:ValidateTransaction")
	if err := txValidator.ValidateTransaction(transaction); err != nil {
		validateSpan.Finish()
		return m.handleError(validateCtx, transaction, err)
	}
	validateSpan.Finish()

	tx, err := m.TransactionHandler.SubmitTransaction(tracingCtx, transaction.Bytes(), transactionOptions)
	if err != nil {
		return m.handleError(tracingCtx, transaction, err)
	}

	txID := tx.TxID
	if txID == "" {
		txID = transaction.TxID()
	}

	return api.StatusOK, api.TransactionResponse{
		Status:      int(api.StatusOK),
		Title:       "OK",
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    (*api.TransactionResponseTxStatus)(&tx.Status),
		Timestamp:   time.Now(),
		Txid:        &txID,
	}, nil
}

func (m ArcDefaultHandler) extendTransaction(ctx context.Context, transaction *bt.Tx) (err error) {
	parentTxBytes := make(map[string][]byte)
	var btParentTx *bt.Tx

	// get the missing input data for the transaction
	for _, input := range transaction.Inputs {
		parentTxIDStr := input.PreviousTxIDStr()
		b, ok := parentTxBytes[parentTxIDStr]
		if !ok {
			b, err = m.getTransaction(ctx, parentTxIDStr)
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

func (m ArcDefaultHandler) handleError(_ context.Context, transaction *bt.Tx, submitErr error) (api.StatusCode, interface{}, error) {
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
	txBytes, _ := m.TransactionHandler.GetTransaction(ctx, inputTxID)
	// ignore error, we try other options if we don't find it
	if txBytes != nil {
		return txBytes, nil
	}

	// get from node
	txBytes, _ = getTransactionFromNode(ctx, inputTxID)
	// we can ignore any error here, we just check whether we have the transaction
	if txBytes != nil {
		return txBytes, nil
	}

	// get from woc
	txBytes, _ = getTransactionFromWhatsOnChain(ctx, inputTxID)
	// we can ignore any error here, we just check whether we have the transaction
	if txBytes != nil {
		return txBytes, nil
	}

	return nil, transactionHandler.ErrParentTransactionNotFound
}

func getPolicy(logger utils.Logger) (policy *api.NodePolicy, err error) {
	policy, err = getPolicyFromNode()
	if err == nil {
		return policy, nil
	}
	// just print out the error, but do not stop, we will load the default policy instead
	logger.Errorf("could not load policy from bitcoin node, using default: %v", err)

	defaultPolicy, found := gocore.Config().Get("defaultPolicy")
	if found && defaultPolicy != "" {
		if err = json.Unmarshal([]byte(defaultPolicy), &policy); err != nil {
			// this is a fatal error, we cannot start the server without a valid default policy
			return nil, fmt.Errorf("error unmarshalling defaultPolicy: %v", err)
		}

		return policy, nil
	}

	return nil, fmt.Errorf("no policy found")
}

func getPolicyFromNode() (*api.NodePolicy, error) {
	bitcoinRpc, err, rpcFound := gocore.Config().GetURL("peer_rpc")
	if err == nil && rpcFound {
		// connect to bitcoin node and get the settings
		b, err := bitcoin.NewFromURL(bitcoinRpc, false)
		if err != nil {
			return nil, fmt.Errorf("error connecting to peer: %v", err)
		}

		settings, err := b.GetSettings()
		if err != nil {
			return nil, fmt.Errorf("error getting settings from peer: %v", err)
		}

		return &api.NodePolicy{
			ExcessiveBlockSize:              int(settings.ExcessiveBlockSize),
			BlockMaxSize:                    int(settings.BlockMaxSize),
			MaxTxSizePolicy:                 int(settings.MaxTxSizePolicy),
			MaxOrphanTxSize:                 int(settings.MaxOrphanTxSize),
			DataCarrierSize:                 int64(settings.DataCarrierSize),
			MaxScriptSizePolicy:             int(settings.MaxScriptSizePolicy),
			MaxOpsPerScriptPolicy:           int64(settings.MaxOpsPerScriptPolicy),
			MaxScriptNumLengthPolicy:        settings.MaxScriptNumLengthPolicy,
			MaxPubKeysPerMultisigPolicy:     int64(settings.MaxPubKeysPerMultisigPolicy),
			MaxTxSigopsCountsPolicy:         int64(settings.MaxTxSigopsCountsPolicy),
			MaxStackMemoryUsagePolicy:       settings.MaxStackMemoryUsagePolicy,
			MaxStackMemoryUsageConsensus:    settings.MaxStackMemoryUsageConsensus,
			LimitAncestorCount:              settings.LimitAncestorCount,
			LimitCPFPGroupMembersCount:      settings.LimitCPFPGroupMembersCount,
			MaxMempool:                      settings.MaxMempool,
			MaxMempoolSizedisk:              settings.MaxMempoolSizedisk,
			MempoolMaxPercentCPFP:           settings.MempoolMaxPercentCPFP,
			AcceptNonStdOutputs:             settings.AcceptNonStdOutputs,
			DataCarrier:                     settings.DataCarrier,
			MinMiningTxFee:                  float64(settings.MinMiningTxFee),
			MaxStdTxValidationDuration:      settings.MaxStdTxValidationDuration,
			MaxNonStdTxValidationDuration:   settings.MaxNonStdTxValidationDuration,
			MaxTxChainValidationBudget:      settings.MaxTxChainValidationBudget,
			ValidationClockCpu:              bool(settings.ValidationClockCpu),
			MinConsolidationFactor:          settings.MinConsolidationFactor,
			MaxConsolidationInputScriptSize: settings.MaxConsolidationInputScriptSize,
			MinConfConsolidationInput:       settings.MinConfConsolidationInput,
			MinConsolidationInputMaturity:   settings.MinConsolidationInputMaturity,
			AcceptNonStdConsolidationInput:  settings.AcceptNonStdConsolidationInput,
		}, nil
	}

	return nil, nil
}
