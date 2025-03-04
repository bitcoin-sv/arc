package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/api/handler/internal/merkle_verifier"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/pkg/tracing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/validator"
	beefValidator "github.com/bitcoin-sv/arc/internal/validator/beef"
	defaultValidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/api"
)

const (
	timeoutSecondsDefault = 5
	mapExpiryTimeDefault  = 24 * time.Hour
)

var (
	ErrInvalidCallbackURL       = errors.New("invalid callback URL")
	ErrCallbackURLNotAcceptable = errors.New("callback URL not acceptable")
	ErrStatusNotSupported       = errors.New("status not supported")
	ErrDecodingBeef             = errors.New("error while decoding BEEF")
	ErrMaxTimeoutExceeded       = fmt.Errorf("max timeout can not be higher than %d", metamorph.MaxTimeout)
)

type ArcDefaultHandler struct {
	TransactionHandler metamorph.TransactionHandler
	NodePolicy         *bitcoin.Settings

	logger                        *slog.Logger
	now                           func() time.Time
	rejectedCallbackURLSubstrings []string
	txFinder                      validator.TxFinderI
	mapExpiryTime                 time.Duration
	defaultTimeout                time.Duration
	mrVerifier                    validator.MerkleVerifierI
	tracingEnabled                bool
	tracingAttributes             []attribute.KeyValue
}

type PostResponse struct {
	StatusCode int
	response   interface{}
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

func WithServerMaxTimeoutDefault(timeout time.Duration) func(*ArcDefaultHandler) {
	return func(s *ArcDefaultHandler) {
		s.defaultTimeout = timeout
	}
}

func WithCacheExpiryTime(d time.Duration) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.mapExpiryTime = d
	}
}

func WithTracer(attr ...attribute.KeyValue) func(s *ArcDefaultHandler) {
	return func(a *ArcDefaultHandler) {
		a.tracingEnabled = true
		if len(attr) > 0 {
			a.tracingAttributes = append(a.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			a.tracingAttributes = append(a.tracingAttributes, attribute.String("file", file))
		}
	}
}

type Option func(f *ArcDefaultHandler)

func NewDefault(
	logger *slog.Logger,
	transactionHandler metamorph.TransactionHandler,
	merkleRootsVerifier blocktx.MerkleRootsVerifier,
	policy *bitcoin.Settings,
	cachedFinder validator.TxFinderI,
	opts ...Option,
) (*ArcDefaultHandler, error) {
	mr := merkle_verifier.New(merkleRootsVerifier)

	handler := &ArcDefaultHandler{
		TransactionHandler: transactionHandler,
		NodePolicy:         policy,
		logger:             logger,
		now:                time.Now,
		mrVerifier:         mr,
		txFinder:           cachedFinder,
		mapExpiryTime:      mapExpiryTimeDefault,
		defaultTimeout:     timeoutSecondsDefault * time.Second,
	}

	// apply options
	for _, opt := range opts {
		opt(handler)
	}

	return handler, nil
}

func (m ArcDefaultHandler) GETPolicy(ctx echo.Context) (err error) {
	_, span := tracing.StartTracing(ctx.Request().Context(), "GETPolicy", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

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

func (m ArcDefaultHandler) GETHealth(ctx echo.Context) (err error) {
	reqCtx := ctx.Request().Context()
	reqCtx, span := tracing.StartTracing(reqCtx, "GETHealth", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = m.TransactionHandler.Health(reqCtx)
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

func (m ArcDefaultHandler) postTransaction(ctx echo.Context, params api.POSTTransactionParams) PostResponse {
	var err error

	reqCtx, span := tracing.StartTracing(ctx.Request().Context(), "POSTTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	transactionOptions, err := getTransactionOptions(params, m.rejectedCallbackURLSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		return PostResponse{e.Status, e}
	}

	txHex, err := parseTransactionFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return PostResponse{e.Status, e}
	}

	// Now we check if we have the transaction present in db, if so we skip validation (as we must have already validated it)
	// if LastSubmitted is not too old and callbacks are the same then we just stop processing transaction as there is nothing new
	txIDs, e := m.getTxIDs(txHex)
	if e != nil {
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		// if an error is returned, the processing failed
		return PostResponse{e.Status, e}
	}

	if !transactionOptions.ForceValidation {
		// check if we already have the transaction in db (so no need to validate)
		tx, err := m.getTransactionStatus(reqCtx, txIDs[0])
		if err != nil {
			// if we have error which is NOT ErrTransactionNotFound, return err
			if !errors.Is(err, metamorph.ErrTransactionNotFound) {
				m.logger.Error("Failed to get transaction status", slog.String("hash", txIDs[0]), slog.String("err", err.Error()))
			}
		} else {
			// if transaction was found skip the validation
			transactionOptions.SkipTxValidation = true

			// check if all callbacks already exist
			callbackAlreadyExists := true

			if transactionOptions.CallbackURL != "" || transactionOptions.CallbackToken != "" {
				for _, cb := range tx.Callbacks {
					callbackAlreadyExists = false
					if cb.CallbackUrl == transactionOptions.CallbackURL && cb.CallbackToken == transactionOptions.CallbackToken {
						callbackAlreadyExists = true
						break
					}
				}
			}

			// if tx has been last submitted less than expiry time ago and callbacks already exist, return current status
			if time.Since(tx.LastSubmitted.AsTime()) < m.mapExpiryTime && callbackAlreadyExists {
				return PostResponse{int(api.StatusOK), &api.TransactionResponse{
					Status:       int(api.StatusOK),
					Title:        "OK",
					BlockHash:    &tx.BlockHash,
					BlockHeight:  &tx.BlockHeight,
					TxStatus:     (api.TransactionResponseTxStatus)(tx.Status),
					ExtraInfo:    &tx.ExtraInfo,
					CompetingTxs: &tx.CompetingTxs,
					Timestamp:    m.now(),
					Txid:         txIDs[0],
					MerklePath:   &tx.MerklePath,
				}}
			}
		}
	}

	successes, fails, e := m.processTransactions(reqCtx, txHex, transactionOptions)
	if e != nil {
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		// if an error is returned, the processing failed
		return PostResponse{e.Status, e}
	}

	if len(fails) > 0 {
		// if a fail result is returned, the processing/validation failed
		e = fails[0]
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return PostResponse{e.Status, e}
	}

	res := successes[0]

	if span != nil {
		span.SetAttributes(attribute.String("status", string(res.TxStatus)))
	}

	return PostResponse{res.Status, res}
}

// POSTTransaction ...
func (m ArcDefaultHandler) POSTTransaction(ctx echo.Context, params api.POSTTransactionParams) (err error) {
	timeout := m.defaultTimeout
	if params.XMaxTimeout != nil {
		if *params.XMaxTimeout > metamorph.MaxTimeout {
			e := api.NewErrorFields(api.ErrStatusBadRequest, ErrMaxTimeoutExceeded.Error())
			return ctx.JSON(e.Status, e)
		}
		timeout = time.Second * time.Duration(*params.XMaxTimeout)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Request().Context(), timeout)
	ctx.SetRequest(ctx.Request().WithContext(timeoutCtx))
	defer cancel()

	postResponse := m.postTransaction(ctx, params)
	return ctx.JSON(postResponse.StatusCode, postResponse.response)
}

// GETTransactionStatus ...
func (m ArcDefaultHandler) GETTransactionStatus(ctx echo.Context, id string) (err error) {
	reqCtx := ctx.Request().Context()

	reqCtx, span := tracing.StartTracing(reqCtx, "GETTransactionStatus", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	tx, err := m.getTransactionStatus(reqCtx, id)
	if err != nil {
		if errors.Is(err, metamorph.ErrTransactionNotFound) {
			e := api.NewErrorFields(api.ErrStatusNotFound, err.Error())
			return ctx.JSON(e.Status, e)
		}

		e := api.NewErrorFields(api.ErrStatusGeneric, err.Error())
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return ctx.JSON(e.Status, e)
	}

	if tx == nil {
		e := api.NewErrorFields(api.ErrStatusNotFound, "failed to find transaction")
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
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

func (m ArcDefaultHandler) postTransactions(ctx echo.Context, params api.POSTTransactionsParams) PostResponse {
	var err error
	reqCtx, span := tracing.StartTracing(ctx.Request().Context(), "POSTTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// set the globals for all transactions in this request
	transactionOptions, err := getTransactionsOptions(params, m.rejectedCallbackURLSubstrings)
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return PostResponse{e.Status, e}
	}

	txsHex, err := parseTransactionsFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transaction from request: %s", err.Error()))
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return PostResponse{e.Status, e}
	}

	// Now we check if we have the transactions present in db, if so we skip validation (as we must have already validated them)
	// if LastSubmitted is not too old and callbacks are the same then we just stop processing transactions as there is nothing new
	txIDs, e := m.getTxIDs(txsHex)
	if e != nil {
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		// if an error is returned, the processing failed

		return PostResponse{e.Status, e}
	}

	if !transactionOptions.ForceValidation {
		// check if we already have the transactions in db (so no need to validate)
		txStatuses, err := m.getTransactionStatuses(reqCtx, txIDs)
		allTransactionsProcessed := false
		if err != nil {
			// if we have error which is NOT ErrTransactionNotFound, return err
			if !errors.Is(err, metamorph.ErrTransactionNotFound) {
				e := api.NewErrorFields(api.ErrStatusGeneric, err.Error())
				return PostResponse{e.Status, e}
			}
		} else if len(txStatuses) == len(txIDs) {
			// if we have found all the transactions, skip the validation
			transactionOptions.SkipTxValidation = true

			// now check if we need to skip the processing of the transaction
			allProcessed := true
			for _, tx := range txStatuses {
				exists := false
				for _, cb := range tx.Callbacks {
					if cb.CallbackUrl == transactionOptions.CallbackURL {
						exists = true
						break
					}
				}
				if time.Since(tx.LastSubmitted.AsTime()) > m.mapExpiryTime || !exists {
					allProcessed = false
					break
				}
			}
			allTransactionsProcessed = allProcessed
		}

		// if nothing to update return
		var successes []*api.TransactionResponse
		if allTransactionsProcessed {
			for _, tx := range txStatuses {
				successes = append(successes, &api.TransactionResponse{
					Status:       int(api.StatusOK),
					Title:        "OK",
					BlockHash:    &tx.BlockHash,
					BlockHeight:  &tx.BlockHeight,
					TxStatus:     (api.TransactionResponseTxStatus)(tx.Status),
					ExtraInfo:    &tx.ExtraInfo,
					CompetingTxs: &tx.CompetingTxs,
					Timestamp:    m.now(),
					Txid:         tx.TxID,
					MerklePath:   &tx.MerklePath,
				})
			}
			// merge success and fail results
			responses := make([]any, 0, len(successes))
			for _, o := range successes {
				responses = append(responses, o)
			}
			return PostResponse{int(api.StatusOK), responses}
		}
	}

	successes, fails, e := m.processTransactions(reqCtx, txsHex, transactionOptions)
	if e != nil {
		if span != nil {
			attr := e.GetSpanAttributes()
			span.SetAttributes(attr...)
		}
		return PostResponse{e.Status, e}
	}

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

	return PostResponse{int(api.StatusOK), responses}
}

// POSTTransactions ...
func (m ArcDefaultHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) (err error) {
	timeout := m.defaultTimeout
	if params.XMaxTimeout != nil {
		if *params.XMaxTimeout > metamorph.MaxTimeout {
			e := api.NewErrorFields(api.ErrStatusBadRequest, ErrMaxTimeoutExceeded.Error())
			return ctx.JSON(e.Status, e)
		}
		timeout = time.Second * time.Duration(*params.XMaxTimeout)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx.Request().Context(), timeout)
	ctx.SetRequest(ctx.Request().WithContext(timeoutCtx))
	defer cancel()

	postResponse := m.postTransactions(ctx, params)
	return ctx.JSON(postResponse.StatusCode, postResponse.response)
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
	transactionOptions := &metamorph.TransactionOptions{}
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

	if params.XFullStatusUpdates != nil {
		transactionOptions.FullStatusUpdates = *params.XFullStatusUpdates
	}

	if params.XForceValidation != nil && *params.XForceValidation {
		transactionOptions.SkipTxValidation = false
		transactionOptions.ForceValidation = true
	}

	return transactionOptions, nil
}

func (m ArcDefaultHandler) getTxIDs(txsHex []byte) ([]string, *api.ErrorFields) {
	var txIDs []string
	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)
		if hexFormat == validator.BeefHex {
			beefTx, remainingBytes, err := beef.DecodeBEEF(txsHex)
			if err != nil {
				errStr := errors.Join(ErrDecodingBeef, err).Error()
				return nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}
			txsHex = remainingBytes
			txIDs = append(txIDs, beefTx.GetLatestTx().TxID().String())
		} else {
			transaction, bytesUsed, err := sdkTx.NewTransactionFromStream(txsHex)
			if err != nil {
				return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			}
			txsHex = txsHex[bytesUsed:]
			txIDs = append(txIDs, transaction.TxID().String())
		}
	}

	return txIDs, nil
}

// processTransactions validates all the transactions in the array and submits to metamorph for processing.
func (m ArcDefaultHandler) processTransactions(ctx context.Context, txsHex []byte, options *metamorph.TransactionOptions) (successes []*api.TransactionResponse, fails []*api.ErrorFields, processingErr *api.ErrorFields) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "processTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var submittedTxs []*sdkTx.Transaction

	// decode and validate txs
	var txIDs []string
	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)

		if hexFormat == validator.BeefHex {
			beefTx, remainingBytes, err := beef.DecodeBEEF(txsHex)
			if err != nil {
				errStr := errors.Join(ErrDecodingBeef, err).Error()
				return nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
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

			txIDs = append(txIDs, beefTx.GetLatestTx().TxID().String())
		} else {
			transaction, bytesUsed, err := sdkTx.NewTransactionFromStream(txsHex)
			if err != nil {
				return nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
			}

			txsHex = txsHex[bytesUsed:]

			v := defaultValidator.New(m.NodePolicy, m.txFinder)
			if arcError := m.validateEFTransaction(ctx, v, transaction, options); arcError != nil {
				fails = append(fails, arcError)
				continue
			}

			submittedTxs = append(submittedTxs, transaction)
			txIDs = append(txIDs, transaction.TxID().String())
		}
	}

	if len(submittedTxs) == 0 {
		return nil, fails, nil
	}

	// submit valid transactions to metamorph
	txStatuses, e := m.submitTransactions(ctx, submittedTxs, options)
	if e != nil {
		return nil, nil, e
	}

	// prepare success results
	txStatuses = filterStatusesByTxIDs(txIDs, txStatuses)

	now := m.now()
	successes = make([]*api.TransactionResponse, 0, len(submittedTxs))

	for idx, tx := range txStatuses {
		txID := tx.TxID
		if txID == "" {
			txID = submittedTxs[idx].TxID().String()
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

	return successes, fails, nil
}

func (m ArcDefaultHandler) validateEFTransaction(ctx context.Context, txValidator validator.DefaultValidator, transaction *sdkTx.Transaction, options *metamorph.TransactionOptions) *api.ErrorFields {
	var err error
	ctx, span := tracing.StartTracing(ctx, "validateEFTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	err = txValidator.ValidateTransaction(ctx, transaction, feeOpts, scriptOpts, m.tracingEnabled, m.tracingAttributes...)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, transaction, err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", transaction.TxID().String()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))
		return arcError
	}

	return nil
}

func (m ArcDefaultHandler) validateBEEFTransaction(ctx context.Context, txValidator validator.BeefValidator, beefTx *beef.BEEF, options *metamorph.TransactionOptions) *api.ErrorFields {
	var err error
	ctx, span := tracing.StartTracing(ctx, "validateBEEFTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	errTx, err := txValidator.ValidateTransaction(ctx, beefTx, feeOpts, scriptOpts)
	if err != nil {
		statusCode, arcError := m.handleError(ctx, errTx, err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", errTx.TxID().String()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

		return arcError
	}

	return nil
}

func (m ArcDefaultHandler) submitTransactions(ctx context.Context, txs []*sdkTx.Transaction, options *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, *api.ErrorFields) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "submitTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var submitStatuses []*metamorph.TransactionStatus

	// to avoid false negatives first check if ctx is expired
	select {
	case <-ctx.Done():
		_, arcError := m.handleError(ctx, nil, context.DeadlineExceeded)
		return nil, arcError
	default:
	}

	if len(txs) == 1 {
		tx := txs[0]

		// SubmitTransaction() used to avoid performance issue
		var status *metamorph.TransactionStatus
		status, err = m.TransactionHandler.SubmitTransaction(ctx, tx, options)
		if err != nil {
			statusCode, arcError := m.handleError(ctx, tx, err)
			m.logger.ErrorContext(ctx, "failed to submit transaction", slog.String("id", tx.TxID().String()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}

		submitStatuses = append(submitStatuses, status)
	} else {
		submitStatuses, err = m.TransactionHandler.SubmitTransactions(ctx, txs, options)
		if err != nil {
			statusCode, arcError := m.handleError(ctx, nil, err)
			m.logger.ErrorContext(ctx, "failed to submit transactions", slog.Int("txs", len(txs)), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

			return nil, arcError
		}
	}

	return submitStatuses, nil
}

func (m ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (tx *metamorph.TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "getTransactionStatus", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	tx, err = m.TransactionHandler.GetTransactionStatus(ctx, id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (m ArcDefaultHandler) getTransactionStatuses(ctx context.Context, txIDs []string) (tx []*metamorph.TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "getTransactionStatus", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	tx, err = m.TransactionHandler.GetTransactionStatuses(ctx, txIDs)
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
		arcError.Txid = PtrTo(transaction.TxID().String())
	}

	return status, arcError
}

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
