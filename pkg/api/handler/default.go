package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/validator"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/labstack/echo/v4"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-bitcoin"
)

const (
	maxTimeout               = 30
	maxTimeoutSecondsDefault = 5
)

type ArcDefaultHandler struct {
	TransactionHandler            metamorph.TransactionHandler
	MerkleRootsVerifier           blocktx.MerkleRootsVerifier
	NodePolicy                    *bitcoin.Settings
	logger                        *slog.Logger
	now                           func() time.Time
	rejectedCallbackUrlSubstrings []string
	peerRpcConfig                 *config.PeerRpcConfig
	apiConfig                     *config.ApiConfig
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

func NewDefault(
	logger *slog.Logger,
	transactionHandler metamorph.TransactionHandler,
	merkleRootsVerifier blocktx.MerkleRootsVerifier,
	policy *bitcoin.Settings,
	peerRpcConfig *config.PeerRpcConfig,
	apiConfig *config.ApiConfig,
	opts ...Option,
) (api.ServerInterface, error) {
	handler := &ArcDefaultHandler{
		TransactionHandler:  transactionHandler,
		MerkleRootsVerifier: merkleRootsVerifier,
		NodePolicy:          policy,
		logger:              logger,
		now:                 time.Now,
		peerRpcConfig:       peerRpcConfig,
		apiConfig:           apiConfig,
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
			Version: &version.Version,
			Reason:  &reason,
		})
	}

	return ctx.JSON(http.StatusOK, api.Health{
		Healthy: PtrTo(true),
		Version: &version.Version,
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

	transactionHex, err := parseTransactionFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

	var transaction *bt.Tx
	var response *api.TransactionResponse
	var responseErr *api.ErrorFields

	if beef.CheckBeefFormat(transactionHex) {
		transaction, response, responseErr = m.processBEEFTransaction(ctx.Request().Context(), transactionHex, transactionOptions)
	} else {
		transaction, response, responseErr = m.processEFTransaction(ctx.Request().Context(), transactionHex, transactionOptions)
	}

	if responseErr != nil {
		// if an error is returned, the processing failed, and we should return a 500 error
		return ctx.JSON(responseErr.Status, responseErr)
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

	txsHexes, err := parseTransactionsFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

	// process all transactions
	transactions, responses, e := m.processTransactions(ctx.Request().Context(), txsHexes, transactionOptions)
	if e != nil {
		return ctx.JSON(e.Status, e)
	}

	sizingInfo := make([][]uint64, 0)
	for _, btTx := range transactions {
		normalBytes, dataBytes, feeAmount := getSizings(btTx)
		sizingInfo = append(sizingInfo, []uint64{normalBytes, dataBytes, feeAmount})
	}
	sizingCtx := context.WithValue(ctx.Request().Context(), ContextSizings, sizingInfo)
	ctx.SetRequest(ctx.Request().WithContext(sizingCtx))
	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission
	return ctx.JSON(int(api.StatusOK), responses)
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
	transactionOptions := &metamorph.TransactionOptions{
		MaxTimeout: maxTimeoutSecondsDefault,
	}
	if params.XCallbackUrl != nil {
		if err := ValidateCallbackURL(*params.XCallbackUrl, rejectedCallbackUrlSubstrings); err != nil {
			return nil, err
		}

		transactionOptions.CallbackURL = *params.XCallbackUrl
	}

	if params.XCallbackToken != nil {
		transactionOptions.CallbackToken = *params.XCallbackToken
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

func (m ArcDefaultHandler) processEFTransaction(ctx context.Context, transactionHex []byte, transactionOptions *metamorph.TransactionOptions) (*bt.Tx, *api.TransactionResponse, *api.ErrorFields) {
	txValidator := defaultValidator.New(m.NodePolicy)

	transaction, err := bt.NewTxFromBytes(transactionHex)
	if err != nil {
		return nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
	}

	if arcError := m.validateEFTransaction(ctx, txValidator, transaction, transactionOptions); arcError != nil {
		return nil, nil, arcError
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(transactionOptions.MaxTimeout+2)*time.Second)
	defer cancel()

	tx, err := m.TransactionHandler.SubmitTransaction(timeoutCtx, transaction, transactionOptions)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, transaction, err)
		m.logger.Error("failed to submit transaction", slog.String("id", transaction.TxID()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

		return nil, nil, arcError
	}

	return transaction, &api.TransactionResponse{
		Status:      int(api.StatusOK),
		Title:       "OK",
		BlockHash:   &tx.BlockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    tx.Status,
		ExtraInfo:   &tx.ExtraInfo,
		Timestamp:   m.now(),
		Txid:        transaction.TxID(),
		MerklePath:  &tx.MerklePath,
	}, nil
}

func (m ArcDefaultHandler) processBEEFTransaction(ctx context.Context, transactionHex []byte, transactionOptions *metamorph.TransactionOptions) (*bt.Tx, *api.TransactionResponse, *api.ErrorFields) {
	txValidator := defaultValidator.New(m.NodePolicy)

	beefTx, _, err := beef.DecodeBEEF(transactionHex)
	if err != nil {
		errStr := fmt.Sprintf("error decoding BEEF: %s", err.Error())
		return nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
	}

	if err := m.validateBEEFTransaction(ctx, txValidator, beefTx, transactionOptions); err != nil {
		return nil, nil, err
	}

	transactions := make([]*bt.Tx, 0)

	for _, tx := range beefTx.Transactions {
		if !tx.IsMined() {
			transactions = append(transactions, tx.Transaction)
		}
	}

	if len(transactions) == 0 {
		return nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, "all transactions in BEEF are mined")
	}

	txStatuses, err := m.TransactionHandler.SubmitTransactions(ctx, transactions, transactionOptions)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, nil, err)
		m.logger.Error("failed to submit transactions", slog.Int("txs", len(transactions)), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
		return nil, nil, arcError
	}

	lastStatus := findStatusByTxID(beefTx.GetLatestTx().TxID(), txStatuses)
	if lastStatus == nil {
		return nil, nil, api.NewErrorFields(api.ErrStatusNotFound, "last tx of BEEF not found in metamorph submit response")
	}

	return beefTx.GetLatestTx(), &api.TransactionResponse{
		Status:      int(api.StatusOK),
		Title:       "OK",
		BlockHash:   &lastStatus.BlockHash,
		BlockHeight: &lastStatus.BlockHeight,
		TxStatus:    lastStatus.Status,
		ExtraInfo:   &lastStatus.ExtraInfo,
		Timestamp:   m.now(),
		Txid:        lastStatus.TxID,
		MerklePath:  &lastStatus.MerklePath,
	}, nil
}

// processTransactions validates all the transactions in the array and submits to metamorph for processing.
func (m ArcDefaultHandler) processTransactions(ctx context.Context, transactionsHexes []byte, transactionOptions *metamorph.TransactionOptions) ([]*bt.Tx, []interface{}, *api.ErrorFields) {
	m.logger.Info("Starting to process transactions")

	// validate before submitting array of transactions to metamorph
	transactions := make([]*bt.Tx, 0)
	txIds := make([]string, 0)
	txErrors := make([]interface{}, 0)

	txValidator := defaultValidator.New(m.NodePolicy)

	for len(transactionsHexes) != 0 {
		isBeefFormat := txValidator.IsBeef(transactionsHexes)

		if isBeefFormat {
			beefTx, remainingBytes, err := beef.DecodeBEEF(transactionsHexes)
			if err != nil {
				errStr := fmt.Sprintf("error decoding BEEF: %s", err.Error())
				return nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			transactionsHexes = remainingBytes

			if errTx, err := txValidator.ValidateBeef(beefTx, transactionOptions.SkipFeeValidation, transactionOptions.SkipScriptValidation); err != nil {
				_, arcError := m.handleError(ctx, errTx, err)
				txErrors = append(txErrors, arcError)
				continue
			}

			for _, tx := range beefTx.Transactions {
				if !tx.IsMined() {
					transactions = append(transactions, tx.Transaction)
				}
			}
			transactions = append(transactions, beefTx.GetLatestTx())
			txIds = append(txIds, beefTx.GetLatestTx().TxID())
		} else {
			transaction, bytesUsed, err := bt.NewTxFromStream(transactionsHexes)
			if err != nil {
				return nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			}

			transactionsHexes = transactionsHexes[bytesUsed:]

			if arcError := m.validateEFTransaction(ctx, txValidator, transaction, transactionOptions); arcError != nil {
				txErrors = append(txErrors, arcError)
				continue
			}

			transactions = append(transactions, transaction)
			txIds = append(txIds, transaction.TxID())
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(transactionOptions.MaxTimeout+2)*time.Second)
	defer cancel()

	// submit all the validated array of transactions to metamorph endpoint
	txStatuses, err := m.TransactionHandler.SubmitTransactions(timeoutCtx, transactions, transactionOptions)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, nil, err)
		m.logger.Error("failed to submit transactions", slog.Int("txs", len(transactions)), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
		return nil, nil, arcError
	}

	txStatuses = filterStatusesByTxIDs(txIds, txStatuses)

	// process returned transaction statuses and return to user
	transactionsOutputs := make([]interface{}, 0, len(transactions))

	for ind, tx := range txStatuses {
		txID := tx.TxID
		if txID == "" {
			txID = transactions[ind].TxID()
		}
		transactionsOutputs = append(transactionsOutputs, api.TransactionResponse{
			Status:      int(api.StatusOK),
			Title:       "OK",
			BlockHash:   &txStatuses[ind].BlockHash,
			BlockHeight: &txStatuses[ind].BlockHeight,
			TxStatus:    tx.Status,
			ExtraInfo:   &txStatuses[ind].ExtraInfo,
			Timestamp:   m.now(),
			Txid:        txID,
			MerklePath:  &txStatuses[ind].MerklePath,
		})
	}

	transactionsOutputs = append(transactionsOutputs, txErrors...)

	return transactions, transactionsOutputs, nil
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
		if err := txValidator.ValidateEFTransaction(transaction, transactionOptions.SkipFeeValidation, transactionOptions.SkipScriptValidation); err != nil {
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

func (m ArcDefaultHandler) validateBEEFTransaction(ctx context.Context, txValidator validator.Validator, beefTx *beef.BEEF, transactionOptions *metamorph.TransactionOptions) *api.ErrorFields {
	if errTx, err := txValidator.ValidateBeef(beefTx, transactionOptions.SkipFeeValidation, transactionOptions.SkipTxValidation); err != nil {
		_, arcError := m.handleError(ctx, errTx, err)
		return arcError
	}

	merkleRoots, err := beef.CalculateMerkleRootsFromBumps(beefTx.BUMPs)
	if err != nil {
		return api.NewErrorFields(api.ErrStatusCalculatingMerkleRoots, err.Error())
	}

	merkleRootsRequest := convertMerkleRootsRequest(merkleRoots)

	unverifiedBlockHeights, err := m.MerkleRootsVerifier.VerifyMerkleRoots(ctx, merkleRootsRequest)
	if err != nil {
		return api.NewErrorFields(api.ErrStatusValidatingMerkleRoots, err.Error())
	}

	if len(unverifiedBlockHeights) > 0 {
		err := fmt.Errorf("unable to verify BUMPs with block heights: %v", unverifiedBlockHeights)
		return api.NewErrorFields(api.ErrStatusValidatingMerkleRoots, err.Error())
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
	txBytes, err := getTransactionFromNode(m.peerRpcConfig, inputTxID)
	if err != nil {
		m.logger.Warn("failed to get transaction from node", slog.String("id", inputTxID), slog.String("err", err.Error()))
	}
	// we can ignore any error here, we just check whether we have the transaction
	if txBytes != nil {
		return txBytes, nil
	}

	// get from woc
	txBytes, err = getTransactionFromWhatsOnChain(ctx, m.apiConfig.WocApiKey, inputTxID)
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
