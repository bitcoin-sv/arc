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
	beefValidator "github.com/bitcoin-sv/arc/internal/validator/beef"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/internal/woc_client"
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

	txFinder validator.TxFinderI
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

	// TODO: I don't like it
	var wocClient *woc_client.WocClient
	if apiConfig != nil {
		wocClient = woc_client.New(woc_client.WithAuth(apiConfig.WocApiKey))
	} else {
		wocClient = woc_client.New()
	}

	finder := txFinder{
		th:         transactionHandler,
		pc:         peerRpcConfig,
		l:          logger,
		w:          wocClient,
		useMainnet: true, // TODO: refactor in scope of ARCO-147
	}

	handler := &ArcDefaultHandler{
		TransactionHandler:  transactionHandler,
		MerkleRootsVerifier: merkleRootsVerifier,
		NodePolicy:          policy,
		logger:              logger,
		now:                 time.Now,
		peerRpcConfig:       peerRpcConfig,
		apiConfig:           apiConfig,
		txFinder:            &finder,
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

	txHex, err := parseTransactionFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

	reqCtx := ctx.Request().Context()
	txs, successes, fails, e := m.processTransactions(reqCtx, txHex, transactionOptions)

	if e != nil {
		// if an error is returned, the processing failed
		return ctx.JSON(e.Status, e)
	}

	if len(fails) > 0 {
		// if an fail result is returned, the processing/validation failed
		e = fails[0]
		return ctx.JSON(e.Status, e)
	}

	sizingCtx := context.WithValue(reqCtx, ContextSizings, prepareSizingInfo(txs))
	ctx.SetRequest(ctx.Request().WithContext(sizingCtx))

	response := successes[0]
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

	txsHex, err := parseTransactionsFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

	reqCtx := ctx.Request().Context()
	txs, successes, fails, e := m.processTransactions(reqCtx, txsHex, transactionOptions)
	if e != nil {
		return ctx.JSON(e.Status, e)
	}

	sizingCtx := context.WithValue(reqCtx, ContextSizings, prepareSizingInfo(txs))
	ctx.SetRequest(ctx.Request().WithContext(sizingCtx))
	// we cannot really return any other status here
	// each transaction in the slice will have the result of the transaction submission

	// merge success and fail results
	responses := make([]any, 0, len(successes)+len(fails))
	for _, o := range successes {
		responses = append(responses, o)
	}
	for _, fo := range fails {
		responses = append(responses, fo)
	}

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

// processTransactions validates all the transactions in the array and submits to metamorph for processing.
func (m ArcDefaultHandler) processTransactions(ctx context.Context, txsHex []byte, options *metamorph.TransactionOptions) (
	submittedTxs []*bt.Tx, successes []*api.TransactionResponse, fails []*api.ErrorFields, processingErr *api.ErrorFields) {
	m.logger.Info("Starting to process transactions")

	// decode and validate txs
	var txIds []string

	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)

		if hexFormat == validator.BeefHex {
			beefTx, remainingBytes, err := beef.DecodeBEEF(txsHex)

			if err != nil {
				errStr := fmt.Sprintf("error decoding BEEF: %s", err.Error())
				return nil, nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			txsHex = remainingBytes

			v := beefValidator.New(m.NodePolicy)
			if arcError := m.validateBEEFTransaction(ctx, v, beefTx, options); arcError != nil {
				fails = append(fails, arcError)
				continue
			}

			for _, tx := range beefTx.Transactions {
				if !tx.IsMined() {
					submittedTxs = append(submittedTxs, tx.Transaction)
				}
			}

			txIds = append(txIds, beefTx.GetLatestTx().TxID())
		} else {
			transaction, bytesUsed, err := bt.NewTxFromStream(txsHex)
			if err != nil {
				return nil, nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			}

			txsHex = txsHex[bytesUsed:]

			v := defaultValidator.New(m.NodePolicy, m.txFinder)
			if arcError := m.validateEFTransaction(ctx, v, transaction, options); arcError != nil {
				fails = append(fails, arcError)
				continue
			}

			submittedTxs = append(submittedTxs, transaction)
			txIds = append(txIds, transaction.TxID())
		}
	}

	if len(submittedTxs) == 0 {
		return nil, nil, fails, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(options.MaxTimeout+2)*time.Second)
	defer cancel()

	// submit valid transactions to metamorph
	txStatuses, e := m.submitTransactions(timeoutCtx, submittedTxs, options)
	if e != nil {
		return nil, nil, nil, e
	}

	// prepare success results
	txStatuses = filterStatusesByTxIDs(txIds, txStatuses)

	now := m.now()
	successes = make([]*api.TransactionResponse, 0, len(submittedTxs))

	for idx, tx := range txStatuses {
		txID := tx.TxID
		if txID == "" {
			txID = submittedTxs[idx].TxID()
		}
		successes = append(successes, &api.TransactionResponse{
			Status:      int(api.StatusOK),
			Title:       "OK",
			BlockHash:   &tx.BlockHash,
			BlockHeight: &tx.BlockHeight,
			TxStatus:    tx.Status,
			ExtraInfo:   &tx.ExtraInfo,
			Timestamp:   now,
			Txid:        txID,
			MerklePath:  &tx.MerklePath,
		})
	}

	return submittedTxs, successes, fails, nil
}

func (m ArcDefaultHandler) validateEFTransaction(ctx context.Context, txValidator validator.DefaultValidator, transaction *bt.Tx, options *metamorph.TransactionOptions) *api.ErrorFields {
	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	if err := txValidator.ValidateTransaction(ctx, transaction, feeOpts, scriptOpts); err != nil {
		statusCode, arcError := m.handleError(ctx, transaction, err)
		m.logger.Error("failed to validate transaction", slog.String("id", transaction.TxID()), slog.Int("id", int(statusCode)), slog.String("err", err.Error()))
		return arcError
	}

	return nil
}

func (m ArcDefaultHandler) validateBEEFTransaction(ctx context.Context, txValidator validator.BeefValidator, beefTx *beef.BEEF, options *metamorph.TransactionOptions) *api.ErrorFields {
	// TODO: wait for the decision from the managment
	// if transactionOptions.SkipTxValidation {
	// 	return nil
	// }

	feeOpts, scriptOpts := toValidationOpts(options)

	if errTx, err := txValidator.ValidateTransaction(ctx, beefTx, feeOpts, scriptOpts); err != nil {
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

func (m ArcDefaultHandler) submitTransactions(ctx context.Context, txs []*bt.Tx, options *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, *api.ErrorFields) {
	var submitStatuses []*metamorph.TransactionStatus

	if len(txs) == 1 {
		tx := txs[0]

		// SubmitTransaction() used to avoid performance issue
		status, err := m.TransactionHandler.SubmitTransaction(ctx, tx, options)

		if err != nil {
			statusCode, arcError := m.handleError(ctx, tx, err)
			m.logger.Error("failed to submit transaction", slog.String("id", tx.TxID()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}

		submitStatuses = append(submitStatuses, status)

	} else {
		var err error
		submitStatuses, err = m.TransactionHandler.SubmitTransactions(ctx, txs, options)

		if err != nil {
			statusCode, arcError := m.handleError(ctx, nil, err)
			m.logger.Error("failed to submit transactions", slog.Int("txs", len(txs)), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}
	}

	return submitStatuses, nil
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
	}

	// enrich the response with the error details
	arcError := api.NewErrorFields(status, submitErr.Error())

	if transaction != nil {
		arcError.Txid = PtrTo(transaction.TxID())
	}

	return status, arcError
}

func prepareSizingInfo(txs []*bt.Tx) [][]uint64 {
	sizingInfo := make([][]uint64, 0, len(txs))
	for _, btTx := range txs {
		normalBytes, dataBytes, feeAmount := getSizings(btTx)
		sizingInfo = append(sizingInfo, []uint64{normalBytes, dataBytes, feeAmount})
	}

	return sizingInfo
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

func toValidationOpts(opts *metamorph.TransactionOptions) (validator.FeeValidation, validator.ScriptValidation) {
	fv := validator.StandardFeeValidation
	if opts.SkipFeeValidation {
		fv = validator.NoneFeeValidation
	}

	sv := validator.StandardScriptValidation
	if opts.SkipScriptValidation {
		sv = validator.NoneScriptValidation
	}

	return fv, sv
}
