package blocktx

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BlocktxClient interface {
	Health(ctx context.Context) error
	ClearTransactions(ctx context.Context, retentionDays int32) (int64, error)
	ClearBlocks(ctx context.Context, retentionDays int32) (int64, error)
	ClearBlockTransactionsMap(ctx context.Context, retentionDays int32) (int64, error)
	DelUnfinishedBlockProcessing(ctx context.Context, processedBy string) error
}

type Client struct {
	client blocktx_api.BlockTxAPIClient
}

func NewClient(client blocktx_api.BlockTxAPIClient) BlocktxClient {
	btc := &Client{
		client: client,
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

func (btc *Client) DelUnfinishedBlockProcessing(ctx context.Context, processedBy string) error {
	_, err := btc.client.DelUnfinishedBlockProcessing(ctx, &blocktx_api.DelUnfinishedBlockProcessingRequest{ProcessedBy: processedBy})
	if err != nil {
		return err
	}
	return nil
}

func (btc *Client) ClearTransactions(ctx context.Context, retentionDays int32) (int64, error) {
	resp, err := btc.client.ClearTransactions(ctx, &blocktx_api.ClearData{RetentionDays: retentionDays})
	if err != nil {
		return 0, err
	}
	return resp.Rows, nil
}

func (btc *Client) ClearBlocks(ctx context.Context, retentionDays int32) (int64, error) {
	resp, err := btc.client.ClearBlocks(ctx, &blocktx_api.ClearData{RetentionDays: retentionDays})
	if err != nil {
		return 0, err
	}
	return resp.Rows, nil
}

func (btc *Client) ClearBlockTransactionsMap(ctx context.Context, retentionDays int32) (int64, error) {
	resp, err := btc.client.ClearBlockTransactionsMap(ctx, &blocktx_api.ClearData{RetentionDays: retentionDays})
	if err != nil {
		return 0, err
	}
	return resp.Rows, nil
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
