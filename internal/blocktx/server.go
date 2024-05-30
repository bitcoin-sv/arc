package blocktx

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/libsv/go-p2p"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server type carries the logger within it.
type Server struct {
	blocktx_api.UnsafeBlockTxAPIServer
	store                         store.BlocktxStore
	logger                        *slog.Logger
	grpcServer                    *grpc.Server
	peers                         []p2p.PeerI
	cleanup                       func()
	maxAllowedBlockHeightMismatch int
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(storeI store.BlocktxStore, logger *slog.Logger, peers []p2p.PeerI, maxAllowedBlockHeightMismatch int) *Server {
	return &Server{
		store:                         storeI,
		logger:                        logger,
		peers:                         peers,
		maxAllowedBlockHeightMismatch: maxAllowedBlockHeightMismatch,
	}
}

// StartGRPCServer function.
func (s *Server) StartGRPCServer(address string, grpcMessageSize int, prometheusEndpoint string, logger *slog.Logger) error {
	// LEVEL 0 - no security / no encryption
	srvMetrics, opts, cleanup, err := grpc_opts.GetGRPCServerOpts(logger, prometheusEndpoint, grpcMessageSize, "blocktx")
	if err != nil {
		return err
	}

	s.cleanup = cleanup

	grpcSrv := grpc.NewServer(opts...)
	srvMetrics.InitializeMetrics(grpcSrv)

	s.grpcServer = grpcSrv

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	blocktx_api.RegisterBlockTxAPIServer(s.grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	go func() {
		s.logger.Info("GRPC server listening", slog.String("address", address))
		err = s.grpcServer.Serve(lis)
		if err != nil {
			s.logger.Error("GRPC server failed to serve", slog.String("err", err.Error()))
		}
	}()

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

func (s *Server) VerifyMerkleRoots(ctx context.Context, req *blocktx_api.MerkleRootsVerificationRequest) (*blocktx_api.MerkleRootVerificationResponse, error) {
	return s.store.VerifyMerkleRoots(ctx, req.GetMerkleRoots(), s.maxAllowedBlockHeightMismatch)
}

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.grpcServer.Stop()

	s.cleanup()
}
