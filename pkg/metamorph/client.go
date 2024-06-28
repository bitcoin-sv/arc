package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	ErrTransactionNotFound       = errors.New("transaction not found")
	ErrParentTransactionNotFound = errors.New("parent transaction not found")
)

type TransactionHandler interface {
	Health(ctx context.Context) error
	GetTransaction(ctx context.Context, txID string) ([]byte, error)
	GetTransactionStatus(ctx context.Context, txID string) (*TransactionStatus, error)
	SubmitTransaction(ctx context.Context, tx *bt.Tx, options *TransactionOptions) (*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx []*bt.Tx, options *TransactionOptions) ([]*TransactionStatus, error)
}

type TransactionMaintainer interface {
	ClearData(ctx context.Context, retentionDays int32) (int64, error)
	SetUnlockedByName(ctx context.Context, name string) (int64, error)
}

// TransactionStatus defines model for TransactionStatus.
type TransactionStatus struct {
	TxID        string
	MerklePath  string
	BlockHash   string
	BlockHeight uint64
	Status      string
	ExtraInfo   string
	Timestamp   int64
}

// Metamorph is the connector to a metamorph server.
type Metamorph struct {
	client   metamorph_api.MetaMorphAPIClient
	mqClient MessageQueueClient
	logger   *slog.Logger
	now      func() time.Time
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

func DialGRPC(address string, prometheusEndpoint string, grpcMessageSize int) (*grpc.ClientConn, error) {
	dialOpts, err := grpc_opts.GetGRPCClientOpts(prometheusEndpoint, grpcMessageSize)
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
func (m *Metamorph) GetTransaction(ctx context.Context, txID string) ([]byte, error) {
	var tx *metamorph_api.Transaction
	tx, err := m.client.GetTransaction(ctx, &metamorph_api.TransactionStatusRequest{
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

// GetTransactionStatus gets the status of a transaction.
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (status *TransactionStatus, err error) {
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
		TxID:        txID,
		MerklePath:  tx.GetMerklePath(),
		Status:      tx.GetStatus().String(),
		BlockHash:   tx.GetBlockHash(),
		BlockHeight: tx.GetBlockHeight(),
		Timestamp:   m.now().Unix(),
	}, nil
}

func (m *Metamorph) Health(ctx context.Context) error {
	_, err := m.client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	return nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx *bt.Tx, options *TransactionOptions) (*TransactionStatus, error) {
	request := &metamorph_api.TransactionRequest{
		RawTx:             tx.Bytes(),
		CallbackUrl:       options.CallbackURL,
		CallbackToken:     options.CallbackToken,
		WaitForStatus:     options.WaitForStatus,
		FullStatusUpdates: options.FullStatusUpdates,
		MaxTimeout:        int64(options.MaxTimeout),
	}

	if options.WaitForStatus == metamorph_api.Status_QUEUED && m.mqClient != nil {
		err := m.mqClient.PublishSubmitTx(request)
		if err != nil {
			return nil, err
		}

		return &TransactionStatus{
			TxID:      tx.TxID(),
			Status:    metamorph_api.Status_QUEUED.String(),
			Timestamp: m.now().Unix(),
		}, nil
	}

	response, err := m.client.PutTransaction(ctx, request)
	if err != nil {
		m.logger.Warn("Failed to submit transaction", slog.String("hash", tx.TxID()), slog.String("key", err.Error()))
		if m.mqClient != nil {
			err = m.mqClient.PublishSubmitTx(request)
			if err != nil {
				return nil, err
			}

			return &TransactionStatus{
				TxID:      tx.TxID(),
				Status:    metamorph_api.Status_QUEUED.String(),
				Timestamp: m.now().Unix(),
			}, nil
		}

		return nil, err
	}

	return &TransactionStatus{
		TxID:        response.GetTxid(),
		Status:      response.GetStatus().String(),
		ExtraInfo:   response.GetRejectReason(),
		BlockHash:   response.GetBlockHash(),
		BlockHeight: response.GetBlockHeight(),
		MerklePath:  response.GetMerklePath(),
		Timestamp:   m.now().Unix(),
	}, nil
}

// SubmitTransactions submits transactions to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs []*bt.Tx, options *TransactionOptions) ([]*TransactionStatus, error) {
	// prepare transaction inputs
	in := new(metamorph_api.TransactionRequests)
	in.Transactions = make([]*metamorph_api.TransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.GetTransactions(), &metamorph_api.TransactionRequest{
			RawTx:             tx.Bytes(),
			CallbackUrl:       options.CallbackURL,
			CallbackToken:     options.CallbackToken,
			WaitForStatus:     options.WaitForStatus,
			FullStatusUpdates: options.FullStatusUpdates,
			MaxTimeout:        int64(options.MaxTimeout),
		})
	}

	if options.WaitForStatus == metamorph_api.Status_QUEUED && m.mqClient != nil {
		err := m.mqClient.PublishSubmitTxs(in)
		if err != nil {
			return nil, err
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

	// put all transactions together
	responses, err := m.client.PutTransactions(ctx, in)
	if err != nil {
		m.logger.Warn("Failed to submit transactions", slog.String("key", err.Error()))
		if m.mqClient != nil {
			err = m.mqClient.PublishSubmitTxs(in)
			if err != nil {
				return nil, err
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

		return nil, err
	}

	// parse response and return to user
	ret := make([]*TransactionStatus, 0)
	for _, response := range responses.GetStatuses() {
		ret = append(ret, &TransactionStatus{
			TxID:        response.GetTxid(),
			MerklePath:  response.GetMerklePath(),
			Status:      response.GetStatus().String(),
			ExtraInfo:   response.GetRejectReason(),
			BlockHash:   response.GetBlockHash(),
			BlockHeight: response.GetBlockHeight(),
			Timestamp:   m.now().Unix(),
		})
	}

	return ret, nil
}

func (m *Metamorph) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	resp, err := m.client.ClearData(ctx, &metamorph_api.ClearDataRequest{RetentionDays: retentionDays})
	if err != nil {
		return 0, err
	}

	return resp.RecordsAffected, nil
}

func (m *Metamorph) SetUnlockedByName(ctx context.Context, name string) (int64, error) {
	resp, err := m.client.SetUnlockedByName(ctx, &metamorph_api.SetUnlockedByNameRequest{Name: name})
	if err != nil {
		return 0, err
	}

	return resp.RecordsAffected, nil
}

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	ClientID             string               `json:"client_id"`
	CallbackURL          string               `json:"callback_url,omitempty"`
	CallbackToken        string               `json:"callback_token,omitempty"`
	SkipFeeValidation    bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation     bool                 `json:"X-SkipTxValidation,omitempty"`
	WaitForStatus        metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates    bool                 `json:"full_status_updates,omitempty"`
	MaxTimeout           int                  `json:"max_timeout,omitempty"`
}
