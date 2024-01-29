package blocktx

import (
	"context"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ClientI is the interface for the block-tx transaction_handler.
type ClientI interface {
	GetTransactionMerklePath(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error)
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error
	Health(ctx context.Context) error
}

type Client struct {
	client blocktx_api.BlockTxAPIClient
}

type Option func(f *Client)

func NewClient(client blocktx_api.BlockTxAPIClient, opts ...Option) ClientI {
	btc := &Client{
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

func (btc *Client) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error {
	_, err := btc.client.RegisterTransaction(ctx, transaction)
	if err != nil {
		return err
	}

	return nil
}

func (btc *Client) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	return btc.client.GetTransactionBlocks(ctx, transaction)
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
