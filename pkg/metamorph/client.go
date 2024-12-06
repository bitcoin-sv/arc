package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/tracing"
)

const retryInterval = 300 * time.Millisecond
const maxTimeoutDefault = 5 * time.Second

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

type TransactionHandler interface {
	Health(ctx context.Context) error
	GetTransactions(ctx context.Context, txIDs []string) ([]*Transaction, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	SubmitTransaction(ctx context.Context, tx *sdkTx.Transaction, options *TransactionOptions) (*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx sdkTx.Transactions, options *TransactionOptions) ([]*TransactionStatus, error)
}

type TransactionMaintainer interface {
	ClearData(ctx context.Context, retentionDays int32) (int64, error)
	SetUnlockedByName(ctx context.Context, name string) (int64, error)
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID         string
	MerklePath   string
	BlockHash    string
	BlockHeight  uint64
	Status       string
	ExtraInfo    string
	Callbacks    []*metamorph_api.Callback
	CompetingTxs []string
	Timestamp    int64
}

// Metamorph is the connector to a metamorph server.
type Metamorph struct {
	client            metamorph_api.MetaMorphAPIClient
	mqClient          MessageQueueClient
	logger            *slog.Logger
	now               func() time.Time
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
	maxTimeout        time.Duration
}

func WithMaxTimeoutDefault(d time.Duration) func(*Metamorph) {
	return func(m *Metamorph) {
		m.maxTimeout = d
	}
}

func WithMqClient(mqClient MessageQueueClient) func(*Metamorph) {
	return func(m *Metamorph) {
		m.mqClient = mqClient
	}
}

func WithNow(nowFunc func() time.Time) func(*Metamorph) {
	return func(p *Metamorph) {
		p.now = nowFunc
	}
}

func WithLogger(logger *slog.Logger) func(*Metamorph) {
	return func(m *Metamorph) {
		m.logger = logger
	}
}

func WithTracer(attr ...attribute.KeyValue) func(s *Metamorph) {
	return func(m *Metamorph) {
		m.tracingEnabled = true
		if len(attr) > 0 {
			m.tracingAttributes = append(m.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			m.tracingAttributes = append(m.tracingAttributes, attribute.String("file", file))
		}
	}
}

// NewClient creates a connection to a list of metamorph servers via gRPC.
func NewClient(client metamorph_api.MetaMorphAPIClient, opts ...func(client *Metamorph)) *Metamorph {
	m := &Metamorph{
		client:     client,
		logger:     slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		now:        time.Now,
		maxTimeout: maxTimeoutDefault,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func DialGRPC(address string, prometheusEndpoint string, grpcMessageSize int, tracingConfig *config.TracingConfig) (*grpc.ClientConn, error) {
	dialOpts, err := grpc_opts.GetGRPCClientOpts(prometheusEndpoint, grpcMessageSize, tracingConfig)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(address, dialOpts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// GetTransaction gets the transaction bytes from metamorph.
func (m *Metamorph) GetTransaction(ctx context.Context, txID string) (rawTx []byte, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransaction", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var tx *metamorph_api.Transaction
	tx, err = m.client.GetTransaction(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	return tx.GetRawTx(), nil
}

// GetTransactions gets the transactions data from metamorph.
func (m *Metamorph) GetTransactions(ctx context.Context, txIDs []string) (result []*Transaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	txs, err := m.client.GetTransactions(ctx, &metamorph_api.TransactionsStatusRequest{
		TxIDs: txIDs[:],
	})
	if err != nil {
		return nil, err
	}

	if txs == nil {
		return make([]*Transaction, 0), nil
	}

	result = make([]*Transaction, len(txs.Transactions))
	for i, tx := range txs.Transactions {
		result[i] = &Transaction{
			TxID:        tx.Txid,
			Bytes:       tx.RawTx,
			BlockHeight: tx.BlockHeight,
		}
	}

	return result, nil
}

// GetTransactionStatus gets the status of a transaction.
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (txStatus *TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatus", m.tracingEnabled, append(m.tracingAttributes, attribute.String("txID", txID))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var tx *metamorph_api.TransactionStatus
	tx, err = m.client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil && !strings.Contains(err.Error(), metamorph.ErrNotFound.Error()) {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	return &TransactionStatus{
		TxID:         txID,
		MerklePath:   tx.GetMerklePath(),
		Status:       tx.GetStatus().String(),
		BlockHash:    tx.GetBlockHash(),
		BlockHeight:  tx.GetBlockHeight(),
		ExtraInfo:    tx.GetRejectReason(),
		CompetingTxs: tx.GetCompetingTxs(),
		Callbacks:    tx.GetCallbacks(),
		Timestamp:    m.now().Unix(),
	}, nil
}

func (m *Metamorph) Health(ctx context.Context) (err error) {
	ctx, span := tracing.StartTracing(ctx, "Health", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	_, err = m.client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	return nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx *sdkTx.Transaction, options *TransactionOptions) (txStatus *TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "SubmitTransaction", m.tracingEnabled, append(m.tracingAttributes, attribute.String("txID", tx.TxID()))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	request := transactionRequest(tx.Bytes(), options)

	if options.WaitForStatus == metamorph_api.Status_QUEUED && m.mqClient != nil {
		err = m.mqClient.PublishMarshal(ctx, SubmitTxTopic, request)
		if err != nil {
			return nil, err
		}

		return &TransactionStatus{
			TxID:      tx.TxID(),
			Status:    metamorph_api.Status_QUEUED.String(),
			Timestamp: m.now().Unix(),
		}, nil
	}

	var response *metamorph_api.TransactionStatus
	var getStatusErr error

	// in case of error try PutTransaction until timeout expires

	maxTimeout := max(time.Duration(request.MaxTimeout)*time.Second, m.maxTimeout)
	retryTicker := time.NewTicker(retryInterval)
	defer retryTicker.Stop()

	timeoutTimer := time.NewTimer(maxTimeout)

	for {
		response, err = m.client.PutTransaction(ctx, request)
		if err == nil {
			txStatus = &TransactionStatus{
				TxID:         response.GetTxid(),
				Status:       response.GetStatus().String(),
				ExtraInfo:    response.GetRejectReason(),
				CompetingTxs: response.GetCompetingTxs(),
				BlockHash:    response.GetBlockHash(),
				BlockHeight:  response.GetBlockHeight(),
				MerklePath:   response.GetMerklePath(),
				Callbacks:    response.GetCallbacks(),
				Timestamp:    m.now().Unix(),
			}
			break
		}

		m.logger.ErrorContext(ctx, "Failed to put transaction", slog.String("hash", tx.TxID()), slog.String("err", err.Error()))

		if status.Code(err) == codes.Code(code.Code_DEADLINE_EXCEEDED) {
			// if error is deadline exceeded, check tx status to avoid false negatives

			fallbackCtx, fallbackCtxCancel := cloneCtxWithNewTimeout(ctx, time.Second)
			txStatus, getStatusErr = m.GetTransactionStatus(fallbackCtx, tx.TxID())
			fallbackCtxCancel()

			if getStatusErr == nil {
				break
			}
			m.logger.ErrorContext(ctx, "Failed to get transaction status", slog.String("hash", tx.TxID()), slog.String("err", getStatusErr.Error()))
		}

		select {
		case <-timeoutTimer.C:
			return nil, err

		case <-retryTicker.C:
			continue
		}
	}

	return txStatus, nil
}

// SubmitTransactions submits transactions to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs sdkTx.Transactions, options *TransactionOptions) (txStatuses []*TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "SubmitTransactions", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// prepare transaction inputs
	in := new(metamorph_api.TransactionRequests)
	in.Transactions = make([]*metamorph_api.TransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.Transactions, transactionRequest(tx.Bytes(), options))
	}

	if options.WaitForStatus == metamorph_api.Status_QUEUED && m.mqClient != nil {
		for _, tx := range in.Transactions {
			err = m.mqClient.PublishMarshal(ctx, SubmitTxTopic, tx)
			if err != nil {
				return nil, err
			}
		}

		// parse response and return to user
		ret := make([]*TransactionStatus, 0)
		for _, tx := range txs {
			ret = append(ret, &TransactionStatus{
				TxID:      tx.TxID(),
				Status:    metamorph_api.Status_QUEUED.String(),
				Timestamp: m.now().Unix(),
			})
		}

		return ret, nil
	}
	var responses *metamorph_api.TransactionStatuses

	// in case of error try PutTransaction until timeout expires

	maxTimeout := max(time.Duration(in.Transactions[0].MaxTimeout)*time.Second, m.maxTimeout)
	retryTicker := time.NewTicker(retryInterval)
	defer retryTicker.Stop()

	timeoutTimer := time.NewTimer(maxTimeout)

	for {
		responses, err = m.client.PutTransactions(ctx, in)
		if err == nil {
			for _, response := range responses.GetStatuses() {
				txStatuses = append(txStatuses, &TransactionStatus{
					TxID:         response.GetTxid(),
					MerklePath:   response.GetMerklePath(),
					Status:       response.GetStatus().String(),
					ExtraInfo:    response.GetRejectReason(),
					CompetingTxs: response.GetCompetingTxs(),
					BlockHash:    response.GetBlockHash(),
					BlockHeight:  response.GetBlockHeight(),
					Callbacks:    response.GetCallbacks(),
					Timestamp:    m.now().Unix(),
				})
			}
			break
		}

		m.logger.ErrorContext(ctx, "Failed to put transactions", slog.String("err", err.Error()))

		// if error is deadline exceeded, check tx status to avoid false negatives
		if status.Code(err) == codes.Code(code.Code_DEADLINE_EXCEEDED) {
			completedErrorFree := true
			for _, tx := range txs {
				// Todo: Create and use here client.GetTransactionStatuses rpc function
				fallbackCtx, fallbackCtxCancel := cloneCtxWithNewTimeout(ctx, time.Second)
				txStatus, getStatusErr := m.GetTransactionStatus(fallbackCtx, tx.TxID())
				fallbackCtxCancel()

				if getStatusErr != nil {
					m.logger.ErrorContext(ctx, "Failed to get transaction status", slog.String("hash", tx.TxID()), slog.String("err", getStatusErr.Error()))
					completedErrorFree = false
					break
				}

				txStatuses = append(txStatuses, txStatus)
			}

			if completedErrorFree {
				break
			}
		}

		select {
		case <-timeoutTimer.C:
			return nil, err

		case <-retryTicker.C:
			continue
		}
	}

	return txStatuses, nil
}

func (m *Metamorph) ClearData(ctx context.Context, retentionDays int32) (rowsAffected int64, err error) {
	ctx, span := tracing.StartTracing(ctx, "ClearData", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	resp, err := m.client.ClearData(ctx, &metamorph_api.ClearDataRequest{RetentionDays: retentionDays})
	if err != nil {
		return 0, err
	}

	return resp.RecordsAffected, nil
}

func (m *Metamorph) SetUnlockedByName(ctx context.Context, name string) (rowsAffected int64, err error) {
	ctx, span := tracing.StartTracing(ctx, "SetUnlockedByName", m.tracingEnabled, m.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	resp, err := m.client.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: name})
	if err != nil {
		return 0, err
	}

	return resp.RecordsAffected, nil
}

func transactionRequest(rawTx []byte, options *TransactionOptions) *metamorph_api.TransactionRequest {
	return &metamorph_api.TransactionRequest{
		RawTx:             rawTx,
		CallbackUrl:       options.CallbackURL,
		CallbackToken:     options.CallbackToken,
		CallbackBatch:     options.CallbackBatch,
		WaitForStatus:     options.WaitForStatus,
		FullStatusUpdates: options.FullStatusUpdates,
		MaxTimeout:        int64(options.MaxTimeout),
	}
}

// creates a new context with all parent data, but ignores its timing out or cancellation
func cloneCtxWithNewTimeout(parent context.Context, timeout time.Duration) (context.Context, func()) {
	ctx := context.WithoutCancel(parent)
	return context.WithTimeout(ctx, timeout)
}

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	ClientID                string               `json:"client_id"`
	CallbackURL             string               `json:"callback_url,omitempty"`
	CallbackToken           string               `json:"callback_token,omitempty"`
	CallbackBatch           bool                 `json:"callback_batch,omitempty"`
	SkipFeeValidation       bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation    bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation        bool                 `json:"X-SkipTxValidation,omitempty"`
	CumulativeFeeValidation bool                 `json:"X-CumulativeFeeValidation,omitempty"`
	WaitForStatus           metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates       bool                 `json:"full_status_updates,omitempty"`
	MaxTimeout              int                  `json:"max_timeout,omitempty"`
}

type Transaction struct {
	TxID        string
	Bytes       []byte
	BlockHeight uint64
}
