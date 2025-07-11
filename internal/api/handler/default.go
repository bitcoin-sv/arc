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
	"sync"
	"sync/atomic"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/ccoveille/go-safecast"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/version"
	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	timeoutSecondsDefault        = 5
	rebroadcastExpirationDefault = 24 * time.Hour
	currentBlockUpdateInterval   = 5 * time.Second
	GenesisForkBlockMain         = int32(620539)
	GenesisForkBlockTest         = int32(1344302)
	GenesisForkBlockRegtest      = int32(10000)
)

var (
	ErrInvalidCallbackURL       = errors.New("invalid callback URL")
	ErrCallbackURLNotAcceptable = errors.New("callback URL not acceptable")
	ErrStatusNotSupported       = errors.New("status not supported")
	ErrDecodingBeef             = errors.New("error while decoding BEEF")
	ErrBeefByteSlice            = errors.New("error while getting BEEF byte slice")
	ErrMaxTimeoutExceeded       = fmt.Errorf("max timeout can not be higher than %d", metamorph.MaxTimeout)
)

type ArcDefaultHandler struct {
	TransactionHandler      metamorph.TransactionHandler
	btxClient               blocktx.Client
	NodePolicy              *bitcoin.Settings
	maxTxSizePolicy         uint64
	maxTxSigopsCountsPolicy uint64
	maxscriptsizepolicy     uint64
	currentBlockHeight      int32
	waitGroup               *sync.WaitGroup
	cancelAll               context.CancelFunc
	ctx                     context.Context

	logger                        *slog.Logger
	now                           func() time.Time
	rejectedCallbackURLSubstrings []string
	rebroadcastExpiration         time.Duration
	defaultTimeout                time.Duration
	tracingEnabled                bool
	tracingAttributes             []attribute.KeyValue
	stats                         *Stats
	defaultValidator              DefaultValidator
	beefValidator                 BeefValidator
	standardFormatSupported       bool
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

func WithStats(stats *Stats) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.stats = stats
	}
}

func WithStandardFormatSupported(standardFormatSupported bool) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.standardFormatSupported = standardFormatSupported
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

func WithRebroadcastExpiration(d time.Duration) func(*ArcDefaultHandler) {
	return func(p *ArcDefaultHandler) {
		p.rebroadcastExpiration = d
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

type DefaultValidator interface {
	ValidateTransaction(ctx context.Context, tx *sdkTx.Transaction, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation, blockHeight int32) error
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, beefTx *sdkTx.Beef, feeValidation validator.FeeValidation, scriptValidation validator.ScriptValidation) (failedTx *sdkTx.Transaction, err error)
}

func NewDefault(
	logger *slog.Logger,
	transactionHandler metamorph.TransactionHandler,
	btxClient blocktx.Client,
	policy *bitcoin.Settings,
	defaultValidator DefaultValidator,
	beefValidator BeefValidator,
	opts ...Option,
) (*ArcDefaultHandler, error) {
	var maxscriptsizepolicy, maxTxSigopsCountsPolicy, maxTxSizePolicy uint64
	var err error
	if policy != nil {
		maxscriptsizepolicy, err = safecast.ToUint64(policy.MaxScriptSizePolicy)
		if err != nil {
			return nil, err
		}

		maxTxSigopsCountsPolicy, err = safecast.ToUint64(policy.MaxTxSigopsCountsPolicy)
		if err != nil {
			return nil, err
		}

		maxTxSizePolicy, err = safecast.ToUint64(policy.MaxTxSizePolicy)
		if err != nil {
			return nil, err
		}
	}

	handler := &ArcDefaultHandler{
		TransactionHandler:      transactionHandler,
		NodePolicy:              policy,
		logger:                  logger,
		now:                     time.Now,
		rebroadcastExpiration:   rebroadcastExpirationDefault,
		defaultTimeout:          timeoutSecondsDefault * time.Second,
		maxTxSizePolicy:         maxTxSizePolicy,
		maxTxSigopsCountsPolicy: maxTxSigopsCountsPolicy,
		maxscriptsizepolicy:     maxscriptsizepolicy,
		btxClient:               btxClient,
		waitGroup:               &sync.WaitGroup{},
		defaultValidator:        defaultValidator,
		beefValidator:           beefValidator,
	}

	// apply options
	for _, opt := range opts {
		opt(handler)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	handler.cancelAll = cancelAll
	handler.ctx = ctx

	return handler, nil
}

func (m *ArcDefaultHandler) StartUpdateCurrentBlockHeight() {
	ticker := time.NewTicker(currentBlockUpdateInterval) // Use constant for this
	m.waitGroup.Add(1)

	go func() {
		defer m.waitGroup.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				blockHeight, err := m.btxClient.CurrentBlockHeight(m.ctx)
				if err != nil {
					m.logger.Error("Failed to get current block height", slog.String("err", err.Error()))
					continue
				}

				height, err := safecast.ToInt32(blockHeight.CurrentBlockHeight)
				if err != nil {
					m.logger.Error("cannot cast height to int32", slog.String("err", err.Error()))
					continue
				}
				old := atomic.LoadInt32(&m.currentBlockHeight)
				if old < height {
					atomic.StoreInt32(&m.currentBlockHeight, height)
					m.logger.Info("Current block height updated", slog.Int64("old", int64(old)), slog.Int64("new", int64(height)))
				}
			}
		}
	}()
}

func (m *ArcDefaultHandler) GETPolicy(ctx echo.Context) (err error) {
	_, span := tracing.StartTracing(ctx.Request().Context(), "GETPolicy", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	satoshis, bytes := calcFeesFromBSVPerKB(m.NodePolicy.MinMiningTxFee)

	return ctx.JSON(http.StatusOK, api.PolicyResponse{

		Policy: api.Policy{
			Maxscriptsizepolicy:     m.maxscriptsizepolicy,
			Maxtxsigopscountspolicy: m.maxTxSigopsCountsPolicy,
			Maxtxsizepolicy:         m.maxTxSizePolicy,
			MiningFee: api.FeeAmount{
				Bytes:    bytes,
				Satoshis: satoshis,
			},
			StandardFormatSupported: PtrTo(m.standardFormatSupported),
		},
		Timestamp: m.now().UTC(),
	})
}

func (m *ArcDefaultHandler) GETHealth(ctx echo.Context) (err error) {
	var reason *string
	err = m.TransactionHandler.Health(ctx.Request().Context())
	if err != nil {
		errMsg := err.Error()
		reason = &errMsg
	}
	return ctx.JSON(http.StatusOK, api.Health{
		Healthy: PtrTo(err == nil),
		Version: &version.Version,
		Reason:  reason,
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
func (m *ArcDefaultHandler) POSTTransaction(ctx echo.Context, params api.POSTTransactionParams) (err error) {
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

	txsHex, err := parseTransactionFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transactions from request: %s", err.Error()))
		res := PostResponse{e.Status, e}
		return ctx.JSON(res.StatusCode, res.response)
	}
	txsParams := api.POSTTransactionsParams(params)
	postResponse := m.postTransactions(ctx, txsHex, txsParams)

	switch postResponse.response.(type) {
	case []interface{}:
		response, ok := postResponse.response.([]interface{})
		if ok && len(response) > 0 {
			switch response[0].(type) {
			case *api.TransactionResponse:
				res, ok := postResponse.response.([]interface{})[0].(*api.TransactionResponse)
				if ok {
					return ctx.JSON(res.Status, res)
				}
			case *api.ErrorFields:
				res, ok := postResponse.response.([]interface{})[0].(*api.ErrorFields)
				if ok {
					return ctx.JSON(res.Status, res)
				}
			}
		}

	case *api.ErrorFields:
		res, ok := postResponse.response.(*api.ErrorFields)
		if ok {
			return ctx.JSON(res.Status, res)
		}
	}
	return ctx.JSON(postResponse.StatusCode, postResponse.response)
}

// GETTransactionStatus ...
func (m *ArcDefaultHandler) GETTransactionStatus(ctx echo.Context, id string) (err error) {
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

func (m *ArcDefaultHandler) postTransactions(ctx echo.Context, txsHex []byte, params api.POSTTransactionsParams) PostResponse {
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

	// check if transactions are present in db, if so skip validation (as they must have already been validated them)
	// if LastSubmitted is not too old and callbacks are the same then stop processing transactions as there is nothing new
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
		// check if transactions already exist in db (so no need to validate)
		txStatuses, err := m.getTransactionStatuses(reqCtx, txIDs)
		allTransactionsProcessed := false
		if err != nil {
			// if we have error which is NOT ErrTransactionNotFound, return err
			if !errors.Is(err, metamorph.ErrTransactionNotFound) {
				e := api.NewErrorFields(api.ErrStatusGeneric, err.Error())
				return PostResponse{e.Status, e}
			}
		} else if len(txStatuses) == len(txIDs) {
			// if found all the transactions found, skip the validation
			transactionOptions.SkipTxValidation = true

			// check if processing of the transaction can be skipped
			allTransactionsProcessed = m.checkAllProcessed(txStatuses, transactionOptions)
		}

		// if nothing to update return
		if allTransactionsProcessed {
			return m.postResponseForAllTxsProcessed(txStatuses)
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
	responses := mergeSuccessAndFailResults(successes, fails)

	return PostResponse{int(api.StatusOK), responses}
}

func (m *ArcDefaultHandler) checkAllProcessed(txStatuses []*metamorph.TransactionStatus, transactionOptions *metamorph.TransactionOptions) bool {
	allProcessed := true
	for _, tx := range txStatuses {
		exists := false
		for _, cb := range tx.Callbacks {
			if cb.CallbackUrl == transactionOptions.CallbackURL {
				exists = true
				break
			}
		}
		if time.Since(tx.LastSubmitted.AsTime()) > m.rebroadcastExpiration || !exists {
			allProcessed = false
			break
		}
	}
	return allProcessed
}

func mergeSuccessAndFailResults(successes []*api.TransactionResponse, fails []*api.ErrorFields) []any {
	responses := make([]any, 0, len(successes)+len(fails))
	for _, o := range successes {
		responses = append(responses, o)
	}
	for _, fo := range fails {
		responses = append(responses, fo)
	}
	return responses
}

func (m *ArcDefaultHandler) postResponseForAllTxsProcessed(txStatuses []*metamorph.TransactionStatus) PostResponse {
	var successes []*api.TransactionResponse
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

// POSTTransactions ...
func (m *ArcDefaultHandler) POSTTransactions(ctx echo.Context, params api.POSTTransactionsParams) (err error) {
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

	txsHex, err := parseTransactionsFromRequest(ctx.Request())
	if err != nil {
		e := api.NewErrorFields(api.ErrStatusBadRequest, fmt.Sprintf("error parsing transactions from request: %s", err.Error()))
		res := PostResponse{e.Status, e}
		return ctx.JSON(res.StatusCode, res.response)
	}

	postResponse := m.postTransactions(ctx, txsHex, params)
	return ctx.JSON(postResponse.StatusCode, postResponse.response)
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

func (m *ArcDefaultHandler) getTxIDs(txsHex []byte) ([]string, *api.ErrorFields) {
	var txIDs []string
	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)
		if hexFormat == validator.BeefHex {
			beefTx, _, err := beef.DecodeBEEF(txsHex)
			if err != nil {
				errStr := errors.Join(ErrDecodingBeef, err).Error()
				return nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			beefBytes, err := beefTx.Bytes()
			if err != nil {
				errStr := errors.Join(ErrBeefByteSlice, err).Error()
				return nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			bytesUsed := len(beefBytes)
			txsHex = txsHex[bytesUsed:]

			for _, tx := range beefTx.Transactions {
				// in case there is just 1 transaction append it, otherwise append only unmined
				if tx.DataFormat == sdkTx.RawTx || (tx.DataFormat == sdkTx.RawTxAndBumpIndex && len(beefTx.Transactions) == 1) {
					txIDs = append(txIDs, tx.Transaction.TxID().String())
				}
			}

			continue
		}

		transaction, bytesUsed, err := sdkTx.NewTransactionFromStream(txsHex)
		if err != nil {
			return nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}
		txsHex = txsHex[bytesUsed:]
		txIDs = append(txIDs, transaction.TxID().String())
	}

	return txIDs, nil
}

// processTransactions validates all the transactions in the array and submits to metamorph for processing.
func (m *ArcDefaultHandler) processTransactions(ctx context.Context, txsHex []byte, options *metamorph.TransactionOptions) (successes []*api.TransactionResponse, fails []*api.ErrorFields, processingErr *api.ErrorFields) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "processTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// decode and validate txs
	txIDs, submittedTxs, fails, errFields := m.getTxDataFromHex(ctx, options, txsHex, fails)

	if errFields != nil {
		return nil, nil, errFields
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

func (m *ArcDefaultHandler) getTxDataFromHex(ctx context.Context, options *metamorph.TransactionOptions, txsHex []byte, fails []*api.ErrorFields) ([]string, []*sdkTx.Transaction, []*api.ErrorFields, *api.ErrorFields) {
	var submittedTxs []*sdkTx.Transaction
	var txIDs []string

	for len(txsHex) != 0 {
		hexFormat := validator.GetHexFormat(txsHex)

		if hexFormat == validator.BeefHex {
			beefTx, txID, err := beef.DecodeBEEF(txsHex)
			if err != nil {
				errStr := errors.Join(ErrDecodingBeef, err).Error()
				return nil, nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			beefBytes, err := beefTx.Bytes()
			if err != nil {
				errStr := errors.Join(ErrBeefByteSlice, err).Error()
				return nil, nil, nil, api.NewErrorFields(api.ErrStatusMalformed, errStr)
			}

			bytesUsed := len(beefBytes)
			txsHex = txsHex[bytesUsed:]

			arcError := m.validateBEEFTransaction(ctx, beefTx, options, txID)
			if arcError != nil {
				fails = append(fails, arcError)
				continue
			}

			for _, tx := range beefTx.Transactions {
				// in case there is just 1 transaction append it, otherwise append only unmined
				if tx.DataFormat == sdkTx.RawTx || (tx.DataFormat == sdkTx.RawTxAndBumpIndex && len(beefTx.Transactions) == 1) {
					submittedTxs = append(submittedTxs, tx.Transaction)
					txIDs = append(txIDs, tx.Transaction.TxID().String())
				}
			}

			continue
		}

		transaction, bytesUsed, err := sdkTx.NewTransactionFromStream(txsHex)
		if err != nil {
			return nil, nil, nil, api.NewErrorFields(api.ErrStatusBadRequest, err.Error())
		}

		txsHex = txsHex[bytesUsed:]

		if arcError := m.validateEFTransaction(ctx, transaction, options); arcError != nil {
			fails = append(fails, arcError)
			continue
		}

		submittedTxs = append(submittedTxs, transaction)
		txIDs = append(txIDs, transaction.TxID().String())
	}
	return txIDs, submittedTxs, fails, nil
}

func (m *ArcDefaultHandler) validateEFTransaction(ctx context.Context, tx *sdkTx.Transaction, options *metamorph.TransactionOptions) *api.ErrorFields {
	var err error
	ctx, span := tracing.StartTracing(ctx, "validateEFTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	err = m.defaultValidator.ValidateTransaction(ctx, tx, feeOpts, scriptOpts, atomic.LoadInt32(&m.currentBlockHeight))
	if err != nil {
		statusCode, arcError := m.handleError(ctx, tx.TxID().String(), err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", tx.TxID().String()), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))
		return arcError
	}

	return nil
}

func (m *ArcDefaultHandler) validateBEEFTransaction(ctx context.Context, beefTx *sdkTx.Beef, options *metamorph.TransactionOptions, txID string) *api.ErrorFields {
	var err error
	ctx, span := tracing.StartTracing(ctx, "validateBEEFTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if options.SkipTxValidation {
		return nil
	}

	feeOpts, scriptOpts := toValidationOpts(options)

	failedTx, err := m.beefValidator.ValidateTransaction(ctx, beefTx, feeOpts, scriptOpts)
	if err != nil {
		if failedTx != nil {
			txID = failedTx.TxID().String()
		}
		statusCode, arcError := m.handleError(ctx, txID, err)
		m.logger.ErrorContext(ctx, "failed to validate transaction", slog.String("id", txID), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

		return arcError
	}

	return nil
}

func (m *ArcDefaultHandler) submitTransactions(ctx context.Context, txs []*sdkTx.Transaction, options *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, *api.ErrorFields) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "submitTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var submitStatuses []*metamorph.TransactionStatus

	// to avoid false negatives first check if ctx is expired
	select {
	case <-ctx.Done():
		_, arcError := m.handleError(ctx, "", context.DeadlineExceeded)
		return nil, arcError
	default:
	}

	submitStatuses, err = m.TransactionHandler.SubmitTransactions(ctx, txs, options)
	if err != nil {
		var tx *sdkTx.Transaction
		if len(txs) == 1 {
			tx = txs[0]
		}
		statusCode, arcError := m.handleError(ctx, tx.TxID().String(), err)
		m.logger.ErrorContext(ctx, "failed to submit transactions", slog.Int("txs", len(txs)), slog.Int("status", int(statusCode)), slog.String("err", err.Error()))

		return nil, arcError
	}

	if m.stats != nil {
		m.stats.Add(len(txs))
	}

	return submitStatuses, nil
}

func (m *ArcDefaultHandler) getTransactionStatus(ctx context.Context, id string) (tx *metamorph.TransactionStatus, err error) {
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

func (m *ArcDefaultHandler) getTransactionStatuses(ctx context.Context, txIDs []string) (tx []*metamorph.TransactionStatus, err error) {
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

func (m *ArcDefaultHandler) CurrentBlockHeight() int32 {
	return atomic.LoadInt32(&m.currentBlockHeight)
}

func (m *ArcDefaultHandler) handleError(_ context.Context, txID string, submitErr error) (api.StatusCode, *api.ErrorFields) {
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

	if txID != "" {
		arcError.Txid = PtrTo(txID)
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

func (m *ArcDefaultHandler) Shutdown() {
	if m.stats != nil {
		m.stats.UnregisterStats()
	}

	m.cancelAll()
	m.waitGroup.Wait()
}
