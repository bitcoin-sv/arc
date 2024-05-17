package handler

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/validator"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-bitcoin"
)

const (
	maxTimeout = 30
)

type ArcDefaultHandler struct {
	TransactionHandler            metamorph.TransactionHandler
	NodePolicy                    *bitcoin.Settings
	logger                        *slog.Logger
	now                           func() time.Time
	rejectedCallbackUrlSubstrings []string
}

func WithNow(nowFunc func() time.Time) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.now = nowFunc
	}
}

func WithCallbackUrlRestrictions(rejectedCallbackUrlSubstrings []string) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.rejectedCallbackUrlSubstrings = rejectedCallbackUrlSubstrings
	}
}

type Option func(f *ArcDefaultHandler)

func NewDefault(logger *slog.Logger, transactionHandler metamorph.TransactionHandler, policy *bitcoin.Settings, opts ...Option) (api.ServerInterface, error) {
	handler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
		NodePolicy:         policy,
		logger:             logger,
		now:                time.Now,
	}

	// apply options
	for _, opt := range opts {
		opt(handler)
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
		Timestamp: m.now().UTC(),
	})
}

func (m ArcDefaultHandler) GETHealth(ctx echo.Context) error {
	err := m.TransactionHandler.Health(ctx.Request().Context())
	if err != nil {
		reason := err.Error()
		return ctx.JSON(http.StatusOK, api.Health{
			Healthy: PtrTo(false),
			Reason:  &reason,
		})
	}

	return ctx.JSON(http.StatusOK, api.Health{
		Healthy: PtrTo(true),
		Reason:  nil,
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
	transactionOptions, err := getTransactionOptions(params, m.rejectedCallbackUrlSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		return ctx.JSON(e.Status, e)
	}

	transactionHex, e := parseTransactionFromRequest(ctx.Request())
	if e != nil {
		return ctx.JSON(e.Status, e)
	}

	transaction, response, responseErr := m.processTransaction(ctx.Request().Context(), transactionHex, transactionOptions)
	if responseErr != nil {
		// if an error is returned, the processing failed, and we should return a 500 error
		return ctx.JSON(response.Status, response)
	}

	sizingInfo := make([][]uint64, 1)
	normalBytes, dataBytes, feeAmount := getSizings(transaction)
	sizingInfo[0] = []uint64{normalBytes, dataBytes, feeAmount}
	sizingCtx := context.WithValue(ctx.Request().Context(), ContextSizings, sizingInfo)
	ctx.SetRequest(ctx.Request().WithContext(sizingCtx))

	return ctx.JSON(response.Status, response)
}

// GETTransactionStatus ...
func (m ArcDefaultHandler) GETTransactionStatus(ctx echo.Context, id string) error {
	tx, err := m.getTransactionStatus(ctx.Request().Context(), id)
	if err != nil {
		if errors.Is(err, metamorph.ErrTransactionNotFound) {
			e := api.NewErrorFields(api.ErrStatusNotFound, err.Error())
			return ctx.JSON(e.Status, e)
		}

		e := api.NewErrorFields(api.ErrStatusGeneric, err.Error())
		return ctx.JSON(e.Status, e)
	}

	if tx == nil {
		e := api.NewErrorFields(api.ErrStatusNotFound, "failed to find transaction")
		return ctx.JSON(e.Status, e)
	}

	return ctx.JSON(http.StatusOK, api.TransactionStatus{
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    &tx.Status,
		Timestamp:   m.now(),
		Txid:        tx.TxID,
		MerklePath:  &tx.MerklePath,
		ExtraInfo:   PtrTo(""),
	})
}

// POSTTransactions ...
func (m ArcDefaultHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) error {
	// set the globals for all transactions in this request
	transactionOptions, err := getTransactionsOptions(params, m.rejectedCallbackUrlSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		return ctx.JSON(e.Status, e)
	}

	// Set the transaction reader function to read a text/plain by default.
	// If the mimetype is application/octet-stream, then we will replace this
	// function with one that reads the raw bytes in the switch statement below.
	transactionReaderFn := func(r io.Reader) (*bt.Tx, error) {
		reader := bufio.NewReader(r)
		b, _, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}

		return bt.NewTxFromString(string(b))
	}

	var transactions []interface{}
	sizingInfo := make([][]uint64, 0)

	var transactionInputs []*bt.Tx
	var sizingMap map[string][]uint64

	var status api.StatusCode

	contentType := ctx.Request().Header.Get("Content-Type")
	switch contentType {
	case "application/json":
		body, err := io.ReadAll(ctx.Request().Body)
		if err != nil {
			e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			return ctx.JSON(e.Status, e)
		}

		var txBody api.POSTTransactionsJSONBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			return ctx.JSON(e.Status, e)
		}

		sizingMap = make(map[string][]uint64)
		transactionInputs = make([]*bt.Tx, 0, len(txBody))
		for index, tx := range txBody {
			transaction, err := bt.NewTxFromString(tx.RawTx)
			if err != nil {
				e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
				transactions[index] = e
				return ctx.JSON(e.Status, e)
			}
			transactionInputs = append(transactionInputs, transaction)
			normalBytes, dataBytes, feeAmount := getSizings(transaction)
			sizingMap[transaction.TxID()] = []uint64{normalBytes, dataBytes, feeAmount}
		}
	case "application/octet-stream":
		transactionReaderFn = func(r io.Reader) (*bt.Tx, error) {
			btTx := new(bt.Tx)
			if _, err := btTx.ReadFrom(r); err != nil {
				return nil, err
			}
			return btTx, nil
		}
		fallthrough
	case "text/plain":
		reader := ctx.Request().Body
		transactionInputs = make([]*bt.Tx, 0)
		sizingMap = make(map[string][]uint64)

		isFirstTransaction := true

		// parse each transaction from request body and prepare
		// slice of transactions to process before submitting to metamorph
		for {
			btTx, err := transactionReaderFn(reader)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
					return ctx.JSON(e.Status, e)
				}
			}

			// we reached the end of request body
			if btTx == nil {
				if isFirstTransaction {
					// no transactions found in the request body
					e := api.NewErrorFields(api.ErrStatusBadRequest, "no transactions found in the request body")
					return ctx.JSON(e.Status, e)
				}
				// no more transaction data found, stop the loop
				break
			}

			isFirstTransaction = false
			transactionInputs = append(transactionInputs, btTx)
			normalBytes, dataBytes, feeAmount := getSizings(btTx)
			sizingMap[btTx.TxID()] = []uint64{normalBytes, dataBytes, feeAmount}
		}

	default:
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("given content-type %s does not match any of the allowed content-types", contentType))
		return ctx.JSON(e.Status, e)
	}

	// process all transactions
	status, transactions, err = m.processTransactions(ctx.Request().Context(), transactionInputs, transactionOptions)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusGeneric, fmt.Errorf("failed to process transactions: %v", err).Error())
		return ctx.JSON(e.Status, e)
	}

	for _, btTx := range transactions {
		if tx, ok := btTx.(api.TransactionResponse); ok {
			sizing, found := sizingMap[tx.Txid]
			if !found {
				m.logger.Warn("tx id not found in sizing map", slog.String("id", tx.Txid))
			}

			sizingInfo = append(sizingInfo, sizing)
		}
	}
	sizingCtx := context.WithValue(ctx.Request().Context(), ContextSizings, sizingInfo)
	ctx.SetRequest(ctx.Request().WithContext(sizingCtx))
	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission
	return ctx.JSON(int(status), transactions)
}

func getTransactionOptions(params api.POSTTransactionParams, rejectedCallbackUrlSubstrings []string) (*metamorph.TransactionOptions, error) {
	return getTransactionsOptions(api.POSTTransactionsParams(params), rejectedCallbackUrlSubstrings)
}

func ValidateCallbackURL(callbackURL string, rejectedCallbackUrlSubstrings []string) error {
	_, err := url.ParseRequestURI(callbackURL)
	if err != nil {
		return fmt.Errorf("invalid callback URL [%w]", err)
	}

	for _, substring := range rejectedCallbackUrlSubstrings {
		if strings.Contains(callbackURL, substring) {
			return fmt.Errorf("callback url not acceptable %s", callbackURL)
		}
	}
	return nil
}

func getTransactionsOptions(params api.POSTTransactionsParams, rejectedCallbackUrlSubstrings []string) (*metamorph.TransactionOptions, error) {
	transactionOptions := &metamorph.TransactionOptions{}
	if params.XCallbackUrl != nil {
		if err := ValidateCallbackURL(*params.XCallbackUrl, rejectedCallbackUrlSubstrings); err != nil {
			return nil, err
		}

		transactionOptions.CallbackURL = *params.XCallbackUrl
		if params.XCallbackToken != nil {
			transactionOptions.CallbackToken = *params.XCallbackToken
		}
	}
	if params.XWaitForStatus != nil {
		transactionOptions.WaitForStatus = metamorph_api.Status(*params.XWaitForStatus)
	}
	if params.XSkipFeeValidation != nil {
		transactionOptions.SkipFeeValidation = *params.XSkipFeeValidation
	}

	if params.XSkipScriptValidation != nil {
		transactionOptions.SkipScriptValidation = *params.XSkipScriptValidation
	}
	if params.XSkipTxValidation != nil {
		transactionOptions.SkipTxValidation = *params.XSkipTxValidation
	}

	if params.XMaxTimeout != nil {
		if *params.XMaxTimeout > maxTimeout {
			return nil, fmt.Errorf("max timeout %d can not be higher than %d", *params.XMaxTimeout, maxTimeout)
		}

		transactionOptions.MaxTimeout = *params.XMaxTimeout
	}

	if params.XFullStatusUpdates != nil {
		transactionOptions.FullStatusUpdates = *params.XFullStatusUpdates
	}

	return transactionOptions, nil
}

func parseTransactionFromRequest(request *http.Request) ([]byte, *api.ErrorFields) {
	requestBody := request.Body
	contentType := request.Header.Get("Content-Type")

	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
	}

	var txHex []byte

	switch contentType {
	case "text/plain":
		txHex, err = hex.DecodeString(string(body))
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
	case "application/json":
		var txBody api.POSTTransactionJSONRequestBody
		if err = json.Unmarshal(body, &txBody); err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}

		txHex, err = hex.DecodeString(txBody.RawTx)
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
	case "application/octet-stream":
		txHex = body
	default:
		return nil, api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("given content-type %s does not match any of the allowed content-types", contentType))
	}

	return txHex, nil
}

func (m ArcDefaultHandler) processTransaction(ctx context.Context, transactionHex []byte, transactionOptions *metamorph.TransactionOptions) (*bt.Tx, *api.TransactionResponse, *api.ErrorFields) {
	txValidator := defaultValidator.New(m.NodePolicy)
	isBeefFormat := txValidator.IsBeef(transactionHex)

	var transaction *bt.Tx

	if isBeefFormat {
		var beefTx *beef.BEEF
		var err error

		transaction, beefTx, err = beef.DecodeBEEF(transactionHex)
		if err != nil {
			return nil, nil, api.NewErrorFields(api.ErrStatusTxFormat, err.Error())
		}

		if err := txValidator.ValidateBeef(beefTx); err != nil {
			_, arcError := m.handleError(ctx, transaction, err)
			return nil, nil, arcError
		}
	} else {
		transaction, err := bt.NewTxFromBytes(transactionHex)
		if err != nil {
			return nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}

		if arcError := m.validateEFTransaction(ctx, txValidator, transaction, transactionOptions); arcError != nil {
			return nil, nil, arcError
		}
	}

	tx, err := m.TransactionHandler.SubmitTransaction(ctx, transaction.Bytes(), transactionOptions)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, transaction, err)
		m.logger.Error("failed to submit transaction", slog.String("id", transaction.TxID()), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
		return nil, nil, arcError
	}

	txID := tx.TxID
	if txID == "" {
		txID = transaction.TxID()
	}

	var extraInfo string
	if tx.ExtraInfo != "" {
		extraInfo = tx.ExtraInfo
	}

	return transaction, &api.TransactionResponse{
		Status:      int(api.StatusOK),
		Title:       "OK",
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    tx.Status,
		ExtraInfo:   &extraInfo,
		Timestamp:   m.now(),
		Txid:        txID,
		MerklePath:  &tx.MerklePath,
	}, nil
}

// processTransactions validates all the transactions in the array and submits to metamorph for processing.
func (m ArcDefaultHandler) processTransactions(ctx context.Context, transactions []*bt.Tx, transactionOptions *metamorph.TransactionOptions) (api.StatusCode, []interface{}, error) {
	m.logger.Info(fmt.Sprintf("Starting to process %d transactions", len(transactions)))

	// validate before submitting array of transactions to metamorph
	transactionsInput := make([][]byte, 0, len(transactions))
	txErrors := make([]interface{}, 0, len(transactions))

	for _, transaction := range transactions {
		txValidator := defaultValidator.New(m.NodePolicy)

		// the validator expects an extended transaction
		// we must enrich the transaction with the missing data
		if !txValidator.IsExtended(transaction) {
			err := m.extendTransaction(ctx, transaction)
			if err != nil {
				statusCode, arcError := m.handleError(ctx, transaction, err)
				m.logger.Error("failed to extend transaction", slog.String("id", transaction.TxID()), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
				txErrors = append(txErrors, arcError)
				continue
			}
		}

		if !transactionOptions.SkipTxValidation {
			// validate transaction
			if err := txValidator.ValidateTransaction(transaction, transactionOptions.SkipFeeValidation, transactionOptions.SkipScriptValidation); err != nil {
				_, arcError := m.handleError(ctx, transaction, err)
				txErrors = append(txErrors, arcError)
				continue
			}
		}
		transactionsInput = append(transactionsInput, transaction.Bytes())
	}

	// submit all the validated array of transactions to metamorph endpoint
	txStatuses, err := m.TransactionHandler.SubmitTransactions(ctx, transactionsInput, transactionOptions)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, nil, err)
		m.logger.Error("failed to submit transactions", slog.Int("txs", len(transactions)), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
		return statusCode, []interface{}{arcError}, err
	}

	// process returned transaction statuses and return to user
	transactionOutput := make([]interface{}, 0, len(transactions))

	for ind, tx := range txStatuses {
		transactionOutput = append(transactionOutput, api.TransactionResponse{
			Status:      int(api.StatusOK),
			Title:       "OK",
			BlockHash:   &txStatuses[ind].BlockHash,
			BlockHeight: &txStatuses[ind].BlockHeight,
			TxStatus:    tx.Status,
			ExtraInfo:   &txStatuses[ind].ExtraInfo,
			Timestamp:   m.now(),
			Txid:        transactions[ind].TxID(),
			MerklePath:  &txStatuses[ind].MerklePath,
		})
	}

	transactionOutput = append(transactionOutput, txErrors...)

	return api.StatusOK, transactionOutput, nil
}

func (m ArcDefaultHandler) validateEFTransaction(ctx context.Context, txValidator validator.Validator, transaction *bt.Tx, transactionOptions *metamorph.TransactionOptions) *api.ErrorFields {
	// the validator expects an extended transaction
	// we must enrich the transaction with the missing data
	if !txValidator.IsExtended(transaction) {
		err := m.extendTransaction(ctx, transaction)
		if err != nil {
			statusCode, arcError := m.handleError(ctx, transaction, err)
			m.logger.Error("failed to extend transaction", slog.String("id", transaction.TxID()), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
			return arcError
		}
	}

	if !transactionOptions.SkipTxValidation {
		if err := txValidator.ValidateTransaction(transaction, transactionOptions.SkipFeeValidation, transactionOptions.SkipScriptValidation); err != nil {
			statusCode, arcError := m.handleError(ctx, transaction, err)
			m.logger.Error("failed to validate transaction", slog.String("id", transaction.TxID()), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
			return arcError
		}
	}

	return nil
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

func (m ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (*metamorph.TransactionStatus, error) {
	tx, err := m.TransactionHandler.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (ArcDefaultHandler) handleError(_ context.Context, transaction *bt.Tx, submitErr error) (api.StatusCode, *api.ErrorFields) {
	if submitErr == nil {
		return api.StatusOK, nil
	}

	status := api.ErrStatusGeneric

	var validatorErr *validator.Error
	ok := errors.As(submitErr, &validatorErr)
	if ok {
		status = validatorErr.ArcErrorStatus
	} else if errors.Is(submitErr, metamorph.ErrParentTransactionNotFound) {
		status = api.ErrStatusTxFormat
	}

	// enrich the response with the error details
	arcError := api.NewErrorFields(status, submitErr.Error())

	if transaction != nil {
		arcError.Txid = PtrTo(transaction.TxID())
	}

	return status, arcError
}

// getTransaction returns the transaction with the given id from a store.
func (m ArcDefaultHandler) getTransaction(ctx context.Context, inputTxID string) ([]byte, error) {
	// get from our transaction handler
	txBytes, _ := m.TransactionHandler.GetTransaction(ctx, inputTxID)
	// ignore error, we try other options if we don't find it
	if txBytes != nil {
		return txBytes, nil
	}

	// get from node
	txBytes, err := getTransactionFromNode(ctx, inputTxID)
	if err != nil {
		m.logger.Warn("failed to get transaction from node", slog.String("id", inputTxID), slog.String("err", err.Error()))
	}
	// we can ignore any error here, we just check whether we have the transaction
	if txBytes != nil {
		return txBytes, nil
	}

	// get from woc
	txBytes, err = getTransactionFromWhatsOnChain(ctx, inputTxID)
	if err != nil {
		m.logger.Warn("failed to get transaction from WhatsOnChain", slog.String("id", inputTxID), slog.String("err", err.Error()))
	}
	// we can ignore any error here, we just check whether we have the transaction
	if txBytes != nil {
		return txBytes, nil
	}

	return nil, metamorph.ErrParentTransactionNotFound
}

func getSizings(tx *bt.Tx) (uint64, uint64, uint64) {
	var feeAmount uint64

	for _, in := range tx.Inputs {
		feeAmount += in.PreviousTxSatoshis
	}

	var dataBytes uint64
	for _, out := range tx.Outputs {
		if feeAmount >= out.Satoshis {
			feeAmount -= out.Satoshis
		} else {
			feeAmount = 0
		}

		script := *out.LockingScript
		if out.Satoshis == 0 && len(script) > 0 && (script[0] == 0x6a || (script[0] == 0x00 && script[1] == 0x6a)) {
			dataBytes += uint64(len(script))
		}
	}

	normalBytes := uint64(len(tx.Bytes())) - dataBytes

	return normalBytes, dataBytes, feeAmount
}

// ContextKey type.
type ContextKey int

const (
	ContextSizings ContextKey = iota
)

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}
