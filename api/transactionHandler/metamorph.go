package transactionHandler

import (
	"context"
	"sync"
	"time"

	arc "github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/ordishs/go-utils"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metamorph is the connector to a metamorph server
type Metamorph struct {
	mu            sync.RWMutex
	Client        metamorph_api.MetaMorphAPIClient
	ClientCache   map[string]metamorph_api.MetaMorphAPIClient
	blockTxClient blocktx.ClientI
}

// NewMetamorph creates a connection to a list of metamorph servers via gRPC
func NewMetamorph(targets string, blockTxClient blocktx.ClientI, grpcMessageSize int) (*Metamorph, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	dialOptions := tracing.AddGRPCDialOptions(opts)
	dialOptions = append(dialOptions, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcMessageSize)))
	conn, err := grpc.Dial(targets, dialOptions...)
	if err != nil {
		return nil, err
	}

	return &Metamorph{
		Client:        metamorph_api.NewMetaMorphAPIClient(conn),
		ClientCache:   make(map[string]metamorph_api.MetaMorphAPIClient),
		blockTxClient: blockTxClient,
	}, nil
}

// GetTransaction gets the transaction bytes from metamorph
func (m *Metamorph) GetTransaction(ctx context.Context, txID string) ([]byte, error) {
	client, err := m.getMetamorphClientForTx(ctx, txID)
	if err != nil {
		return nil, err
	}

	var tx *metamorph_api.Transaction
	tx, err = client.GetTransaction(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	return tx.RawTx, nil
}

// GetTransactionStatus gets the status of a transaction
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (status *TransactionStatus, err error) {
	var client metamorph_api.MetaMorphAPIClient
	if client, err = m.getMetamorphClientForTx(ctx, txID); err != nil {
		return nil, err
	}

	var tx *metamorph_api.TransactionStatus
	tx, err = client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	return &TransactionStatus{
		TxID:        txID,
		Status:      tx.Status.String(),
		BlockHash:   tx.BlockHash,
		BlockHeight: tx.BlockHeight,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// SubmitTransaction submits a transaction to the bitcoin network and returns the transaction in raw format
func (m *Metamorph) SubmitTransaction(ctx context.Context, tx []byte, txOptions *arc.TransactionOptions) (*TransactionStatus, error) {
	response, err := m.Client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx:         tx,
		CallbackUrl:   txOptions.CallbackURL,
		CallbackToken: txOptions.CallbackToken,
		MerkleProof:   txOptions.MerkleProof,
		WaitForStatus: txOptions.WaitForStatus,
	})
	if err != nil {
		return nil, err
	}

	return &TransactionStatus{
		TxID:        response.Txid,
		Status:      response.GetStatus().String(),
		ExtraInfo:   response.RejectReason,
		BlockHash:   response.BlockHash,
		BlockHeight: response.BlockHeight,
		Timestamp:   time.Now().Unix(),
	}, nil
}

func (m *Metamorph) getMetamorphClientForTx(ctx context.Context, txID string) (metamorph_api.MetaMorphAPIClient, error) {
	hash, err := utils.DecodeAndReverseHexString(txID)
	if err != nil {
		return nil, err
	}

	var target string
	if target, err = m.blockTxClient.LocateTransaction(ctx, &blocktx_api.Transaction{
		Hash: hash,
	}); err != nil {
		if errors.Is(err, blocktx.ErrTransactionNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	if target == "" {
		// TODO what do we do in this case? Reach out to all metamorph servers or reach out to a node?
		return nil, ErrTransactionNotFound
	}

	m.mu.RLock()
	client, found := m.ClientCache[target]
	m.mu.RUnlock()

	if !found {
		var conn *grpc.ClientConn
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
			grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		}
		if conn, err = grpc.Dial(target, tracing.AddGRPCDialOptions(opts)...); err != nil {
			return nil, err
		}
		client = metamorph_api.NewMetaMorphAPIClient(conn)

		m.mu.Lock()
		m.ClientCache[target] = client
		m.mu.Unlock()
	}

	return client, nil
}
