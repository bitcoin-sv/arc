package blocktx

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	cfg "github.com/bitcoin-sv/arc/internal/helpers"
	"github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	prometheusclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"time"

	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
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
	retryOpts := []retry.CallOption{
		retry.WithMax(3),
		retry.WithPerRetryTimeout(100 * time.Millisecond),
		retry.WithCodes(codes.NotFound, codes.Aborted),
	}

	logger, err := cfg.NewLogger()
	if err != nil {
		return nil, err
	}

	clientMetrics := prometheus.NewClientMetrics(
		prometheus.WithClientHandlingTimeHistogram(
			prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)
	exemplarFromContext := func(ctx context.Context) prometheusclient.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheusclient.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			clientMetrics.UnaryClientInterceptor(prometheus.WithExemplarFromContext(exemplarFromContext)),
			retry.UnaryClientInterceptor(retryOpts...),
			logging.UnaryClientInterceptor(grpc_opts.InterceptorLogger(logger)),
		),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	}

	return grpc.NewClient(address, dialOpts...)
}
