package blocktx

import (
	"context"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ClientI is the interface for the block-tx transactionHandler.
type ClientI interface {
	GetTransactionMerklePath(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	Health(ctx context.Context) error
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

func (btc *Client) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	transactionBlocks, err := btc.client.GetTransactionBlocks(ctx, transaction)
	if err != nil {
		return nil, err
	}

	return transactionBlocks, nil
}

func (btc *Client) Health(ctx context.Context) error {
	_, err := btc.client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	return nil
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
