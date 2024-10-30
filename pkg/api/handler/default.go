package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/validator"
	beefValidator "github.com/bitcoin-sv/arc/internal/validator/beef"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/api"
	merkleverifier "github.com/bitcoin-sv/arc/pkg/api/handler/internal/MerkeVerifier"
	txfinder "github.com/bitcoin-sv/arc/pkg/api/handler/internal/TxFinder"
	"github.com/bitcoin-sv/arc/pkg/blocktx"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
)

const (
	maxTimeout               = 30
	maxTimeoutSecondsDefault = 5
)

var (
	ErrInvalidCallbackURL       = errors.New("invalid callback URL")
	ErrCallbackURLNotAcceptable = errors.New("callback URL not acceptable")
	ErrStatusNotSupported       = errors.New("status not supported")
	ErrDecodingBeef             = errors.New("error while decoding BEEF")

	ErrMaxTimeoutExceeded = fmt.Errorf("max timeout can not be higher than %d", maxTimeout)
)

type ArcDefaultHandler struct {
	TransactionHandler metamorph.TransactionHandler
	NodePolicy         *bitcoin.Settings

	logger                        *slog.Logger
	now                           func() time.Time
	rejectedCallbackURLSubstrings []string
	txFinder                      validator.TxFinderI
	mrVerifier                    validator.MerkleVerifierI
	tracingEnabled                bool
}

func WithNow(nowFunc func() time.Time) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.now = nowFunc
	}
}

func WithCallbackURLRestrictions(rejectedCallbackURLSubstrings []string) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.rejectedCallbackURLSubstrings = rejectedCallbackURLSubstrings
	}
}

func WithTracer() func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.tracingEnabled = true
	}
}

type Option func(f *ArcDefaultHandler)

func NewDefault(
	logger *slog.Logger,
	transactionHandler metamorph.TransactionHandler,
	merkleRootsVerifier blocktx.MerkleRootsVerifier,
	policy *bitcoin.Settings,
	peerRPCConfig *config.PeerRPCConfig,
	apiConfig *config.APIConfig,
	opts ...Option,
) (*ArcDefaultHandler, error) {
	var wocClient *woc_client.WocClient
	if apiConfig != nil {
		wocClient = woc_client.New(apiConfig.WocMainnet, woc_client.WithAuth(apiConfig.WocAPIKey))
	} else {
		wocClient = woc_client.New(false)
	}

	finder := txfinder.NewCached(transactionHandler, peerRPCConfig, wocClient, logger)
	mr := merkleverifier.New(merkleRootsVerifier)

	handler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
		NodePolicy:         policy,
		logger:             logger,
		now:                time.Now,
		txFinder:           &finder,
		mrVerifier:         mr,
	}

	// apply options
	for _, opt := range opts {
		opt(handler)
	}

	return handler, nil
}

func (m ArcDefaultHandler) GETPolicy(ctx echo.Context) error {
	_, span := m.startTracing(ctx.Request().Context(), "GETPolicy")
	defer m.endTracing(span)

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
	reqCtx := ctx.Request().Context()
	reqCtx, span := m.startTracing(reqCtx, "GETHealth")
	defer m.endTracing(span)

	err := m.TransactionHandler.Health(reqCtx)
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
	reqCtx := ctx.Request().Context()
	reqCtx, span := m.startTracing(reqCtx, "POSTTransaction")
	defer m.endTracing(span)

	transactionOptions, err := getTransactionOptions(params, m.rejectedCallbackURLSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		return ctx.JSON(e.Status, e)
	}

	txHex, err := parseTransactionFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

	txs, successes, fails, e := m.processTransactions(reqCtx, txHex, transactionOptions)

	if e != nil {
		// if an error is returned, the processing failed
		return ctx.JSON(e.Status, e)
	}

	if len(fails) > 0 {
		// if a fail result is returned, the processing/validation failed
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
	reqCtx := ctx.Request().Context()
	reqCtx, span := m.startTracing(reqCtx, "GETTransactionStatus")
	defer m.endTracing(span)

	tx, err := m.getTransactionStatus(reqCtx, id)
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
		BlockHash:    &tx.BlockHash,
		BlockHeight:  &tx.BlockHeight,
		TxStatus:     (api.TransactionStatusTxStatus)(tx.Status),
		Timestamp:    m.now(),
		Txid:         tx.TxID,
		MerklePath:   &tx.MerklePath,
		ExtraInfo:    &tx.ExtraInfo,
		CompetingTxs: &tx.CompetingTxs,
	})
}

// POSTTransactions ...
func (m ArcDefaultHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) error {
	reqCtx := ctx.Request().Context()
	reqCtx, span := m.startTracing(reqCtx, "POSTTransactions")
	defer m.endTracing(span)

	// set the globals for all transactions in this request
	transactionOptions, err := getTransactionsOptions(params, m.rejectedCallbackURLSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		return ctx.JSON(e.Status, e)
	}

	txsHex, err := parseTransactionsFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		return ctx.JSON(e.Status, e)
	}

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

func getTransactionOptions(params api.POSTTransactionParams, rejectedCallbackURLSubstrings []string) (*metamorph.TransactionOptions, error) {
	return getTransactionsOptions(api.POSTTransactionsParams(params), rejectedCallbackURLSubstrings)
}

func ValidateCallbackURL(callbackURL string, rejectedCallbackURLSubstrings []string) error {
	_, err := url.ParseRequestURI(callbackURL)
	if err != nil {
		return errors.Join(ErrInvalidCallbackURL, err)
	}

	for _, substring := range rejectedCallbackURLSubstrings {
		if strings.Contains(callbackURL, substring) {
			return ErrCallbackURLNotAcceptable
		}
	}
	return nil
}

func getTransactionsOptions(params api.POSTTransactionsParams, rejectedCallbackURLSubstrings []string) (*metamorph.TransactionOptions, error) {
	transactionOptions := &metamorph.TransactionOptions{
		MaxTimeout: maxTimeoutSecondsDefault,
	}
	if params.XCallbackUrl != nil {
		if err := ValidateCallbackURL(*params.XCallbackUrl, rejectedCallbackURLSubstrings); err != nil {
			return nil, err
		}

		transactionOptions.CallbackURL = *params.XCallbackUrl
	}

	if params.XCallbackToken != nil {
		transactionOptions.CallbackToken = *params.XCallbackToken
	}

	if params.XCallbackBatch != nil {
		transactionOptions.CallbackBatch = *params.XCallbackBatch
	}

	// NOTE: deprecated header, to be removed soon
	if params.XWaitForStatus != nil {
		oldStatusesMap := map[int]string{
			1:   "QUEUED",
			2:   "RECEIVED",
			3:   "STORED",
			4:   "ANNOUNCED_TO_NETWORK",
			5:   "REQUESTED_BY_NETWORK",
			6:   "SENT_TO_NETWORK",
			7:   "ACCEPTED_BY_NETWORK",
			8:   "SEEN_ON_NETWORK",
			9:   "MINED",
			10:  "SEEN_IN_ORPHAN_MEMPOOL",
			108: "CONFIRMED",
			109: "REJECTED",
		}
		statusString, ok := oldStatusesMap[*params.XWaitForStatus]
		if !ok {
			return nil, errors.Join(ErrStatusNotSupported, fmt.Errorf("status: %d", *params.XWaitForStatus))
		}
		newStatusValue, ok := metamorph_api.Status_value[statusString]
		if !ok {
			return nil, errors.Join(ErrStatusNotSupported, fmt.Errorf("status: %s", statusString))
		}
		transactionOptions.WaitForStatus = metamorph_api.Status(newStatusValue)
	}

	if params.XWaitFor != nil {
		value, ok := metamorph_api.Status_value[*params.XWaitFor]
		if !ok {
			return nil, errors.Join(ErrStatusNotSupported, fmt.Errorf("status: %s", *params.XWaitFor))
		}
		transactionOptions.WaitForStatus = metamorph_api.Status(value)
	}

	if params.XSkipFeeValidation != nil {
		transactionOptions.SkipFeeValidation = *params.XSkipFeeValidation
	}
	if params.XCumulativeFeeValidation != nil {
		transactionOptions.CumulativeFeeValidation = *params.XCumulativeFeeValidation
	}
	if params.XSkipScriptValidation != nil {
		transactionOptions.SkipScriptValidation = *params.XSkipScriptValidation
	}
	if params.XSkipTxValidation != nil {
		transactionOptions.SkipTxValidation = *params.XSkipTxValidation
	}

	if params.XMaxTimeout != nil {
		if *params.XMaxTimeout > maxTimeout {
			return nil, ErrMaxTimeoutExceeded
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
	submittedTxs []*sdkTx.Transaction, successes []*api.TransactionResponse, fails []*api.ErrorFields, processingErr *api.ErrorFields,
) {
	ctx, span := m.startTracing(ctx, "processTransactions")
	defer m.endTracing(span)

	// decode and validate txs
	var txIDs []string

	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)

		if hexFormat == validator.BeefHex {
			beefTx, remainingBytes, err := beef.DecodeBEEF(txsHex)
			if err != nil {
				errStr := errors.Join(ErrDecodingBeef, err).Error()
				return nil, nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			txsHex = remainingBytes

			v := beefValidator.New(m.NodePolicy, m.mrVerifier)
			if arcError := m.validateBEEFTransaction(ctx, v, beefTx, options); arcError != nil {
				fails = append(fails, arcError)
				continue
			}

			for _, tx := range beefTx.Transactions {
				if !tx.IsMined() {
					submittedTxs = append(submittedTxs, tx.Transaction)
				}
			}

			txIDs = append(txIDs, beefTx.GetLatestTx().TxID())
		} else {
			transaction, bytesUsed, err := sdkTx.NewTransactionFromStream(txsHex)
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
			txIDs = append(txIDs, transaction.TxID())
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
	txStatuses = filterStatusesByTxIDs(txIDs, txStatuses)

	now := m.now()
	successes = make([]*api.TransactionResponse, 0, len(submittedTxs))

	for idx, tx := range txStatuses {
		txID := tx.TxID
		if txID == "" {
			txID = submittedTxs[idx].TxID()
		}
		successes = append(successes, &api.TransactionResponse{
			Status:       int(api.StatusOK),
			Title:        "OK",
			BlockHash:    &tx.BlockHash,
			BlockHeight:  &tx.BlockHeight,
			TxStatus:     (api.TransactionResponseTxStatus)(tx.Status),
			ExtraInfo:    &tx.ExtraInfo,
			CompetingTxs: &tx.CompetingTxs,
			Timestamp:    now,
			Txid:         txID,
			MerklePath:   &tx.MerklePath,
		})
	}

	return submittedTxs, successes, fails, nil
}

func (m ArcDefaultHandler) validateEFTransaction(ctx context.Context, txValidator validator.DefaultValidator, transaction *sdkTx.Transaction, options *metamorph.TransactionOptions) *api.ErrorFields {
	ctx, span := m.startTracing(ctx, "validateEFTransaction")
	defer m.endTracing(span)

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	if err := txValidator.ValidateTransaction(ctx, transaction, feeOpts, scriptOpts); err != nil {
		statusCode, arcError := m.handleError(ctx, transaction, err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", transaction.TxID()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))
		return arcError
	}

	return nil
}

func (m ArcDefaultHandler) validateBEEFTransaction(ctx context.Context, txValidator validator.BeefValidator, beefTx *beef.BEEF, options *metamorph.TransactionOptions) *api.ErrorFields {
	ctx, span := m.startTracing(ctx, "validateEFTransaction")
	defer m.endTracing(span)

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	if errTx, err := txValidator.ValidateTransaction(ctx, beefTx, feeOpts, scriptOpts); err != nil {
		statusCode, arcError := m.handleError(ctx, errTx, err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", errTx.TxID()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

		return arcError
	}

	return nil
}

func (m ArcDefaultHandler) submitTransactions(ctx context.Context, txs []*sdkTx.Transaction, options *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, *api.ErrorFields) {
	ctx, span := m.startTracing(ctx, "submitTransactions")
	defer m.endTracing(span)

	var submitStatuses []*metamorph.TransactionStatus

	if len(txs) == 1 {
		tx := txs[0]

		// SubmitTransaction() used to avoid performance issue
		status, err := m.TransactionHandler.SubmitTransaction(ctx, tx, options)
		if err != nil {
			statusCode, arcError := m.handleError(ctx, tx, err)
			m.logger.ErrorContext(ctx, "failed to submit transaction", slog.String("id", tx.TxID()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}

		submitStatuses = append(submitStatuses, status)
	} else {
		var err error
		submitStatuses, err = m.TransactionHandler.SubmitTransactions(ctx, txs, options)
		if err != nil {
			statusCode, arcError := m.handleError(ctx, nil, err)
			m.logger.ErrorContext(ctx, "failed to submit transactions", slog.Int("txs", len(txs)), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}
	}

	return submitStatuses, nil
}

func (m ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (*metamorph.TransactionStatus, error) {
	ctx, span := m.startTracing(ctx, "getTransactionStatus")
	defer m.endTracing(span)

	tx, err := m.TransactionHandler.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (ArcDefaultHandler) handleError(_ context.Context, transaction *sdkTx.Transaction, submitErr error) (api.StatusCode, *api.ErrorFields) {
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

func (m ArcDefaultHandler) startTracing(ctx context.Context, spanName string) (context.Context, trace.Span) {
	if m.tracingEnabled {
		var span trace.Span
		ctx, span = otel.Tracer("").Start(ctx, spanName)
		return ctx, span
	}
	return ctx, nil
}

func (m ArcDefaultHandler) endTracing(span trace.Span) {
	if span != nil {
		span.End()
	}
}

func prepareSizingInfo(txs []*sdkTx.Transaction) [][]uint64 {
	sizingInfo := make([][]uint64, 0, len(txs))
	for _, btTx := range txs {
		normalBytes, dataBytes, feeAmount := getSizings(btTx)
		sizingInfo = append(sizingInfo, []uint64{normalBytes, dataBytes, feeAmount})
	}

	return sizingInfo
}

func getSizings(tx *sdkTx.Transaction) (uint64, uint64, uint64) {
	var feeAmount uint64

	for _, in := range tx.Inputs {
		feeAmount += *in.SourceTxSatoshis()
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
	} else if opts.CumulativeFeeValidation {
		fv = validator.CumulativeFeeValidation
	}

	sv := validator.StandardScriptValidation
	if opts.SkipScriptValidation {
		sv = validator.NoneScriptValidation
	}

	return fv, sv
}
