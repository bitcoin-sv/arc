package transaction_handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	arc "github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metamorph is the connector to a metamorph server.
type Metamorph struct {
	Client metamorph_api.MetaMorphAPIClient
}

// NewMetamorph creates a connection to a list of metamorph servers via gRPC.
func NewMetamorph(address string, grpcMessageSize int) (*Metamorph, error) {
	conn, err := DialGRPC(address, grpcMessageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection to address %s: %v", address, err)
	}

	return &Metamorph{
		Client: metamorph_api.NewMetaMorphAPIClient(conn),
	}, nil
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
	tx, err := m.Client.GetTransaction(ctx, &metamorph_api.TransactionStatusRequest{
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
	tx, err = m.Client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
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

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format.
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx []byte, txOptions *arc.TransactionOptions) (*TransactionStatus, error) {
	response, err := m.Client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx:             tx,
		CallbackUrl:       txOptions.CallbackURL,
		CallbackToken:     txOptions.CallbackToken,
		MerkleProof:       txOptions.MerkleProof,
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
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs [][]byte, txOptions *arc.TransactionOptions) ([]*TransactionStatus, error) {
	// prepare transaction inputs
	in := new(metamorph_api.TransactionRequests)
	in.Transactions = make([]*metamorph_api.TransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.GetTransactions(), &metamorph_api.TransactionRequest{
			RawTx:             tx,
			CallbackUrl:       txOptions.CallbackURL,
			CallbackToken:     txOptions.CallbackToken,
			MerkleProof:       txOptions.MerkleProof,
			WaitForStatus:     txOptions.WaitForStatus,
			FullStatusUpdates: txOptions.FullStatusUpdates,
			MaxTimeout:        int64(txOptions.MaxTimeout),
		})
	}

	// put all transactions together
	responses, err := m.Client.PutTransactions(ctx, in)
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
