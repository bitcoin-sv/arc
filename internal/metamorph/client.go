package metamorph

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

// Metamorph is the connector to a metamorph server.
type Metamorph struct {
	client            metamorph_api.MetaMorphAPIClient
	logger            *slog.Logger
	now               func() time.Time
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
	queuedTxsCh       chan *metamorph_api.PostTransactionRequest
}

func WithQueuedTxsCh(queuedTxsCh chan *metamorph_api.PostTransactionRequest) func(*Metamorph) {
	return func(m *Metamorph) {
		m.queuedTxsCh = queuedTxsCh
	}
}

func WithClientNow(nowFunc func() time.Time) func(*Metamorph) {
	return func(p *Metamorph) {
		p.now = nowFunc
	}
}

func WithLogger(logger *slog.Logger) func(*Metamorph) {
	return func(m *Metamorph) {
		m.logger = logger
	}
}

func WithClientTracer(attr ...attribute.KeyValue) func(s *Metamorph) {
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
		client: client,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		now:    time.Now,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
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
		return nil, global.ErrTransactionNotFound
	}

	return tx.GetRawTx(), nil
}

// GetTransactions gets the transactions data from metamorph.
func (m *Metamorph) GetTransactions(ctx context.Context, txIDs []string) (result []*global.Transaction, err error) {
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
		return make([]*global.Transaction, 0), nil
	}

	result = make([]*global.Transaction, len(txs.Transactions))
	for i, tx := range txs.Transactions {
		result[i] = &global.Transaction{
			TxID:        tx.Txid,
			Bytes:       tx.RawTx,
			BlockHeight: tx.BlockHeight,
		}
	}

	return result, nil
}

// GetTransactionStatus gets the status of a transaction.
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (txStatus *global.TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatus", m.tracingEnabled, append(m.tracingAttributes, attribute.String("txID", txID))...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var tx *metamorph_api.TransactionStatus
	tx, err = m.client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil && !strings.Contains(err.Error(), ErrNotFound.Error()) {
		return nil, err
	}

	if tx == nil {
		return nil, global.ErrTransactionNotFound
	}

	txStatus = &global.TransactionStatus{
		TxID:         txID,
		MerklePath:   tx.GetMerklePath(),
		Status:       tx.GetStatus().String(),
		BlockHash:    tx.GetBlockHash(),
		BlockHeight:  tx.GetBlockHeight(),
		ExtraInfo:    tx.GetRejectReason(),
		CompetingTxs: tx.GetCompetingTxs(),
		Callbacks:    tx.GetCallbacks(),
		Timestamp:    m.now().Unix(),
	}

	if tx.GetLastSubmitted() != nil {
		txStatus.LastSubmitted = *tx.GetLastSubmitted()
	}
	return txStatus, nil
}

// GetTransactionStatuses gets the status of all transactions.
func (m *Metamorph) GetTransactionStatuses(ctx context.Context, txIDs []string) (txStatus []*global.TransactionStatus, err error) {
	tracingAttr := m.tracingAttributes
	if len(txIDs) == 1 {
		tracingAttr = append(m.tracingAttributes, attribute.String("txID", txIDs[0]))
	}

	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatus", m.tracingEnabled, tracingAttr...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var txStatuses *metamorph_api.TransactionStatuses
	var txs []*global.TransactionStatus
	var transactionStatusRequests metamorph_api.TransactionsStatusRequest
	transactionStatusRequests.TxIDs = append(transactionStatusRequests.TxIDs, txIDs...)
	txStatuses, err = m.client.GetTransactionStatuses(ctx, &transactionStatusRequests)
	if err != nil && !strings.Contains(err.Error(), ErrNotFound.Error()) {
		return nil, err
	}

	for _, tx := range txStatuses.Statuses {
		txs = append(txs, &global.TransactionStatus{
			TxID:          tx.Txid,
			MerklePath:    tx.GetMerklePath(),
			Status:        tx.GetStatus().String(),
			BlockHash:     tx.GetBlockHash(),
			BlockHeight:   tx.GetBlockHeight(),
			ExtraInfo:     tx.GetRejectReason(),
			CompetingTxs:  tx.GetCompetingTxs(),
			Callbacks:     tx.GetCallbacks(),
			LastSubmitted: *tx.GetLastSubmitted(),
			Timestamp:     m.now().Unix(),
		})
	}

	return txs, nil
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

// SubmitTransactions submits transactions to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs sdkTx.Transactions, options *global.TransactionOptions) (txStatuses []*global.TransactionStatus, err error) {
	attributes := m.tracingAttributes
	if len(txs) == 1 {
		attributes = append(m.tracingAttributes, attribute.String("txID", txs[0].TxID().String()))
	}

	ctx, span := tracing.StartTracing(ctx, "SubmitTransactions", m.tracingEnabled, attributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// prepare transaction inputs
	in := new(metamorph_api.PostTransactionsRequest)
	in.Transactions = make([]*metamorph_api.PostTransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.Transactions, transactionRequest(tx.Bytes(), options))
	}

	if options.WaitForStatus == metamorph_api.Status_QUEUED {
		for _, tx := range in.Transactions {
			if m.queuedTxsCh != nil {
				select {
				case m.queuedTxsCh <- tx:
				default:
					m.logger.Warn("Failed to send transaction to queued txs channel")
				}
			} else {
				m.logger.Warn("Queued txs channel not available")
			}
		}

		// parse response and return to user
		ret := make([]*global.TransactionStatus, 0)
		for _, tx := range txs {
			ret = append(ret, &global.TransactionStatus{
				TxID:      tx.TxID().String(),
				Status:    metamorph_api.Status_QUEUED.String(),
				Timestamp: m.now().Unix(),
			})
		}

		return ret, nil
	}
	var responses *metamorph_api.TransactionStatuses

	deadline, ok := ctx.Deadline()
	if ok {
		newDeadline := deadline.Add(deadlineExtension)

		var newCancel context.CancelFunc
		// increase deadline by `deadlineExtension`. This ensures that expiration happens from inside the metamorph function
		ctx, newCancel = context.WithDeadline(context.WithoutCancel(ctx), newDeadline)
		defer newCancel()
	}

	responses, err = m.client.PostTransactions(ctx, in)
	if err != nil {
		return nil, err
	}
	for _, response := range responses.GetStatuses() {
		txStatuses = append(txStatuses, &global.TransactionStatus{
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
	return txStatuses, nil
}

func transactionRequest(rawTx []byte, options *global.TransactionOptions) *metamorph_api.PostTransactionRequest {
	return &metamorph_api.PostTransactionRequest{
		RawTx:             rawTx,
		CallbackUrl:       options.CallbackURL,
		CallbackToken:     options.CallbackToken,
		CallbackBatch:     options.CallbackBatch,
		WaitForStatus:     options.WaitForStatus,
		FullStatusUpdates: options.FullStatusUpdates,
	}
}
