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

type BlocktxClient interface {
	Health(ctx context.Context) error
	ClearTransactions(ctx context.Context, req *blocktx_api.ClearData) (int64, error)
	ClearBlocks(ctx context.Context, req *blocktx_api.ClearData) (int64, error)
	ClearBlockTransactionsMap(ctx context.Context, req *blocktx_api.ClearData) (int64, error)
}

type Client struct {
	client blocktx_api.BlockTxAPIClient
}

type Option func(f *Client)

func NewClient(client blocktx_api.BlockTxAPIClient, opts ...Option) BlocktxClient {
	btc := &Client{
		client: client,
	}

	for _, opt := range opts {
		opt(btc)
	}

	return btc
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

func (btc *Client) ClearTransactions(ctx context.Context, req *blocktx_api.ClearData) (int64, error) {
	resp, err := btc.client.ClearTransactions(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.Rows, nil
}

func (btc *Client) ClearBlocks(ctx context.Context, req *blocktx_api.ClearData) (int64, error) {
	resp, err := btc.client.ClearBlocks(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.Rows, nil
}

func (btc *Client) ClearBlockTransactionsMap(ctx context.Context, req *blocktx_api.ClearData) (int64, error) {
	resp, err := btc.client.ClearBlockTransactionsMap(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.Rows, nil
}
