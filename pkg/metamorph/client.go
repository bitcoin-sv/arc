package metamorph

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	SubmitTransaction(ctx context.Context, tx []byte, options *TransactionOptions) (*TransactionStatus, error)
	SubmitTransactions(ctx context.Context, tx [][]byte, options *TransactionOptions) ([]*TransactionStatus, error)
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
	client metamorph_api.MetaMorphAPIClient
}

// NewClient creates a connection to a list of metamorph servers via gRPC.
func NewClient(client metamorph_api.MetaMorphAPIClient) *Metamorph {
	return &Metamorph{
		client: client,
	}
}

func DialGRPC(address string, grpcMessageSize int) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcMessageSize)),
	}

	conn, err := grpc.Dial(address, tracing.AddGRPCDialOptions(opts)...)
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
		Timestamp:   time.Now().Unix(),
	}, nil
}

func (m *Metamorph) Health(ctx context.Context) error {
	resp, err := m.client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	if !resp.Ok {
		return errors.New(resp.Details)
	}

	return nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx []byte, txOptions *TransactionOptions) (*TransactionStatus, error) {
	response, err := m.client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx:             tx,
		CallbackUrl:       txOptions.CallbackURL,
		CallbackToken:     txOptions.CallbackToken,
		WaitForStatus:     txOptions.WaitForStatus,
		FullStatusUpdates: txOptions.FullStatusUpdates,
		MaxTimeout:        int64(txOptions.MaxTimeout),
	})
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		TxID:        response.GetTxid(),
		Status:      response.GetStatus().String(),
		ExtraInfo:   response.GetRejectReason(),
		BlockHash:   response.GetBlockHash(),
		BlockHeight: response.GetBlockHeight(),
		MerklePath:  response.GetMerklePath(),
		Timestamp:   time.Now().Unix(),
	}, nil
}

// SubmitTransactions submits transactions to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs [][]byte, txOptions *TransactionOptions) ([]*TransactionStatus, error) {
	// prepare transaction inputs
	in := new(metamorph_api.TransactionRequests)
	in.Transactions = make([]*metamorph_api.TransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.GetTransactions(), &metamorph_api.TransactionRequest{
			RawTx:             tx,
			CallbackUrl:       txOptions.CallbackURL,
			CallbackToken:     txOptions.CallbackToken,
			WaitForStatus:     txOptions.WaitForStatus,
			FullStatusUpdates: txOptions.FullStatusUpdates,
			MaxTimeout:        int64(txOptions.MaxTimeout),
		})
	}

	// put all transactions together
	responses, err := m.client.PutTransactions(ctx, in)
	if err != nil {
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
			Timestamp:   time.Now().Unix(),
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
