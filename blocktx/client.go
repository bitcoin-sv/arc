package blocktx

import (
	"context"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"os"
)

// ClientI is the interface for the block-tx transactionHandler.
type ClientI interface {
	GetTransactionMerklePath(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error)
	GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*blocktx_api.Block, error)
	GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error)
	GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error)
}

const (
	logLevelDefault = slog.LevelInfo
)

type Client struct {
	logger *slog.Logger
	client blocktx_api.BlockTxAPIClient
}

func WithLogger(logger *slog.Logger) func(*Client) {
	return func(p *Client) {
		p.logger = logger.With(slog.String("service", "btx"))
	}
}

type Option func(f *Client)

func NewClient(client blocktx_api.BlockTxAPIClient, opts ...Option) ClientI {
	btc := &Client{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "btx")),
		client: client,
	}

	for _, opt := range opts {
		opt(btc)
	}

	return btc
}

func (btc *Client) GetTransactionMerklePath(ctx context.Context, hash *blocktx_api.Transaction) (string, error) {
	merklePath, err := btc.client.GetTransactionMerklePath(ctx, hash)
	if err != nil {
		return "", err
	}

	return merklePath.GetMerklePath(), nil
}

func (btc *Client) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error) {
	return btc.client.RegisterTransaction(ctx, transaction)
}

func (btc *Client) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.RegisterTransactionResponse, error) {
	block, err := btc.client.GetTransactionBlock(ctx, transaction)
	if err != nil {
		return nil, err
	}

	return &blocktx_api.RegisterTransactionResponse{
		BlockHash:   block.GetHash(),
		BlockHeight: block.GetHeight(),
	}, nil
}

func (btc *Client) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	transactionBlocks, err := btc.client.GetTransactionBlocks(ctx, transaction)
	if err != nil {
		return nil, err
	}

	return transactionBlocks, nil
}

func (btc *Client) GetBlock(ctx context.Context, blockHash *chainhash.Hash) (*blocktx_api.Block, error) {
	block, err := btc.client.GetBlock(ctx, &blocktx_api.Hash{
		Hash: blockHash[:],
	})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (btc *Client) GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error) {
	block, err := btc.client.GetLastProcessedBlock(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (btc *Client) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	mt, err := btc.client.GetMinedTransactionsForBlock(ctx, blockAndSource)
	if err != nil {
		return nil, err
	}

	return mt, nil
}

func DialGRPC(address string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	return grpc.Dial(address, tracing.AddGRPCDialOptions(opts)...)
}
