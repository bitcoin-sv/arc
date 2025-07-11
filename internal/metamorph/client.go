package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

type TransactionHandler interface {
	Health(ctx context.Context) error
	GetTransactions(ctx context.Context, txIDs []string) ([]*Transaction, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	GetTransactionStatuses(ctx context.Context, txIDs []string) ([]*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx sdkTx.Transactions, options *TransactionOptions) ([]*TransactionStatus, error)
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID          string
	MerklePath    string
	BlockHash     string
	BlockHeight   uint64
	Status        string
	ExtraInfo     string
	Callbacks     []*metamorph_api.Callback
	CompetingTxs  []string
	LastSubmitted timestamppb.Timestamp
	Timestamp     int64
}

// Metamorph is the connector to a metamorph server.
type Metamorph struct {
	client            metamorph_api.MetaMorphAPIClient
	mqClient          mq.MessageQueueClient
	logger            *slog.Logger
	now               func() time.Time
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func WithMqClient(mqClient mq.MessageQueueClient) func(*Metamorph) {
	return func(m *Metamorph) {
		m.mqClient = mqClient
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
	if err != nil && !strings.Contains(err.Error(), ErrNotFound.Error()) {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	txStatus = &TransactionStatus{
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
func (m *Metamorph) GetTransactionStatuses(ctx context.Context, txIDs []string) (txStatus []*TransactionStatus, err error) {
	tracingAttr := m.tracingAttributes
	if len(txIDs) == 1 {
		tracingAttr = append(m.tracingAttributes, attribute.String("txID", txIDs[0]))
	}

	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatus", m.tracingEnabled, tracingAttr...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	var txStatuses *metamorph_api.TransactionStatuses
	var txs []*TransactionStatus
	var transactionStatusRequests metamorph_api.TransactionsStatusRequest
	transactionStatusRequests.TxIDs = append(transactionStatusRequests.TxIDs, txIDs...)
	txStatuses, err = m.client.GetTransactionStatuses(ctx, &transactionStatusRequests)
	if err != nil && !strings.Contains(err.Error(), ErrNotFound.Error()) {
		return nil, err
	}

	for _, tx := range txStatuses.Statuses {
		txs = append(txs, &TransactionStatus{
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
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs sdkTx.Transactions, options *TransactionOptions) (txStatuses []*TransactionStatus, err error) {
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

	if options.WaitForStatus == metamorph_api.Status_QUEUED && m.mqClient != nil {
		for _, tx := range in.Transactions {
			err = m.mqClient.PublishMarshal(ctx, mq.SubmitTxTopic, tx)
			if err != nil {
				return nil, err
			}
		}

		// parse response and return to user
		ret := make([]*TransactionStatus, 0)
		for _, tx := range txs {
			ret = append(ret, &TransactionStatus{
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
	return txStatuses, nil
}

func transactionRequest(rawTx []byte, options *TransactionOptions) *metamorph_api.PostTransactionRequest {
	return &metamorph_api.PostTransactionRequest{
		RawTx:             rawTx,
		CallbackUrl:       options.CallbackURL,
		CallbackToken:     options.CallbackToken,
		CallbackBatch:     options.CallbackBatch,
		WaitForStatus:     options.WaitForStatus,
		FullStatusUpdates: options.FullStatusUpdates,
	}
}

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	CallbackURL             string               `json:"callback_url,omitempty"`
	CallbackToken           string               `json:"callback_token,omitempty"`
	CallbackBatch           bool                 `json:"callback_batch,omitempty"`
	SkipFeeValidation       bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation    bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation        bool                 `json:"X-SkipTxValidation,omitempty"`
	ForceValidation         bool                 `json:"X-ForceValidation,omitempty"`
	CumulativeFeeValidation bool                 `json:"X-CumulativeFeeValidation,omitempty"`
	WaitForStatus           metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates       bool                 `json:"full_status_updates,omitempty"`
}

type Transaction struct {
	TxID        string
	Bytes       []byte
	BlockHeight uint64
}
