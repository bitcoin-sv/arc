package transactionHandler

import (
	"context"
	"errors"
	"time"

	arc "github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metamorph is the connector to a metamorph server
type Metamorph struct {
	Client        metamorph_api.MetaMorphAPIClient
	blockTxClient blocktx.ClientI
	logger        utils.Logger
}

// NewMetamorph creates a connection to a list of metamorph servers via gRPC
func NewMetamorph(targets string, blockTxClient blocktx.ClientI, grpcMessageSize int, logger utils.Logger) (*Metamorph, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(grpcMessageSize)),
	}

	conn, err := grpc.Dial(targets, tracing.AddGRPCDialOptions(opts)...)
	if err != nil {
		return nil, err
	}

	return &Metamorph{
		Client:        metamorph_api.NewMetaMorphAPIClient(conn),
		blockTxClient: blockTxClient,
		logger:        logger,
	}, nil
}

// GetTransaction gets the transaction bytes from metamorph
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

	return tx.RawTx, nil
}

// GetTransactionStatus gets the status of a transaction
func (m *Metamorph) GetTransactionStatus(ctx context.Context, txID string) (status *TransactionStatus, err error) {
	var tx *metamorph_api.TransactionStatus
	tx, err = m.Client.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
		Txid: txID,
	})
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return nil, ErrTransactionNotFound
	}

	hash, err := chainhash.NewHashFromStr(txID)
	if err != nil {
		return nil, err
	}

	merklePath, err := m.blockTxClient.GetTransactionMerklePath(ctx, &blocktx_api.Transaction{Hash: hash[:]})
	if err != nil {
		if errors.Is(err, blocktx.ErrTransactionNotFoundForMerklePath) {
			if tx.Status >= metamorph_api.Status_MINED {
				m.logger.Errorf("Merkle path not found for mined transaction %s: %v", hash.String(), err)
			}
		} else {
			m.logger.Errorf("failed to get Merkle path for transaction %s: %v", hash.String(), err)
		}
	}

	return &TransactionStatus{
		TxID:        txID,
		MerklePath:  merklePath,
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
		MerklePath:  response.MerklePath,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// SubmitTransactions submits transactions to the bitcoin network and returns the transaction in raw format
func (m *Metamorph) SubmitTransactions(ctx context.Context, txs [][]byte, txOptions *arc.TransactionOptions) ([]*TransactionStatus, error) {
	// prepare transaction inputs
	in := new(metamorph_api.TransactionRequests)
	in.Transactions = make([]*metamorph_api.TransactionRequest, 0)
	for _, tx := range txs {
		in.Transactions = append(in.Transactions, &metamorph_api.TransactionRequest{
			RawTx:         tx,
			CallbackUrl:   txOptions.CallbackURL,
			CallbackToken: txOptions.CallbackToken,
			MerkleProof:   txOptions.MerkleProof,
			WaitForStatus: txOptions.WaitForStatus,
		})
	}

	// put all transactions together
	responses, err := m.Client.PutTransactions(ctx, in)
	if err != nil {
		return nil, err
	}

	// parse response and return to user
	ret := make([]*TransactionStatus, 0)
	for _, response := range responses.Statuses {
		ret = append(ret, &TransactionStatus{
			TxID:        response.Txid,
			MerklePath:  response.MerklePath,
			Status:      response.GetStatus().String(),
			ExtraInfo:   response.RejectReason,
			BlockHash:   response.BlockHash,
			BlockHeight: response.BlockHeight,
			Timestamp:   time.Now().Unix(),
		})
	}

	return ret, nil
}
