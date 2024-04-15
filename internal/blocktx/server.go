package blocktx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-p2p"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server type carries the logger within it.
type Server struct {
	blocktx_api.UnsafeBlockTxAPIServer
	store      store.BlocktxStore
	logger     *slog.Logger
	grpcServer *grpc.Server
	peers      []p2p.PeerI
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(storeI store.BlocktxStore, logger *slog.Logger, peers []p2p.PeerI) *Server {
	return &Server{
		store:  storeI,
		logger: logger,
		peers:  peers,
	}
}

// StartGRPCServer function.
func (s *Server) StartGRPCServer(address string) error {

	// LEVEL 0 - no security / no encryption
	var opts []grpc.ServerOption
	prometheusEndpoint := viper.Get("prometheusEndpoint")
	if prometheusEndpoint != "" {
		opts = append(opts,
			grpc.ChainStreamInterceptor(grpc_prometheus.StreamServerInterceptor),
			grpc.ChainUnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		)
	}

	s.grpcServer = grpc.NewServer(opts...)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	blocktx_api.RegisterBlockTxAPIServer(s.grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	s.logger.Info("GRPC server listening", slog.String("address", address))

	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("GRPC server failed [%w]", err)
	}

	return nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*blocktx_api.HealthResponse, error) {
	return &blocktx_api.HealthResponse{
		Ok:        true,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) ClearTransactions(ctx context.Context, clearData *blocktx_api.ClearData) (*blocktx_api.ClearDataResponse, error) {
	return s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "transactions")
}

func (s *Server) ClearBlocks(ctx context.Context, clearData *blocktx_api.ClearData) (*blocktx_api.ClearDataResponse, error) {
	return s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "blocks")
}

func (s *Server) ClearBlockTransactionsMap(ctx context.Context, clearData *blocktx_api.ClearData) (*blocktx_api.ClearDataResponse, error) {
	return s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "block_transactions_map")
}

func (s *Server) DelUnfinishedBlockProcessing(ctx context.Context, req *blocktx_api.DelUnfinishedBlockProcessingRequest) (*emptypb.Empty, error) {
	bhs, err := s.store.GetBlockHashesProcessingInProgress(ctx, req.GetProcessedBy())
	if err != nil {
		return &emptypb.Empty{}, err
	}

	for _, bh := range bhs {
		err = s.store.DelBlockProcessing(ctx, bh, req.GetProcessedBy())
		if err != nil {
			return &emptypb.Empty{}, err
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.grpcServer.Stop()
}
