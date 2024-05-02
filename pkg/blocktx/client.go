package blocktx

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
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

func DialGRPC(logger *slog.Logger, address string, prometheusEndpoint string, grpcMessageSize int) (*grpc.ClientConn, error) {
	dialOpts, err := grpc_opts.GetGRPCClientOpts(logger, prometheusEndpoint, grpcMessageSize)
	if err != nil {
		return nil, err
	}

	return grpc.NewClient(address, dialOpts...)
}
