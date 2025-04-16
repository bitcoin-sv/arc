package blocktx

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
)

type ProcessorI interface {
	RegisterTransaction(txHash []byte)
}

type PeerManager interface {
	CountConnectedPeers() uint
}

// Server type carries the logger within it.
type Server struct {
	blocktx_api.UnsafeBlockTxAPIServer
	grpc_utils.GrpcServer

	logger                        *slog.Logger
	pm                            PeerManager
	store                         store.BlocktxStore
	maxAllowedBlockHeightMismatch uint64
	processor                     ProcessorI
	mqClient                      mq.MessageQueueClient
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(logger *slog.Logger, store store.BlocktxStore, pm PeerManager, processor ProcessorI, cfg grpc_utils.ServerConfig, maxAllowedBlockHeightMismatch uint64, mqClient mq.MessageQueueClient) (*Server, error) {
	logger = logger.With(slog.String("module", "server"))

	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		GrpcServer:                    grpcServer,
		store:                         store,
		logger:                        logger,
		pm:                            pm,
		processor:                     processor,
		maxAllowedBlockHeightMismatch: maxAllowedBlockHeightMismatch,
		mqClient:                      mqClient,
	}

	// register health server endpoint
	grpc_health_v1.RegisterHealthServer(grpcServer.Srv, s)

	blocktx_api.RegisterBlockTxAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*blocktx_api.HealthResponse, error) {
	status := nats.DISCONNECTED.String()
	if s.mqClient != nil {
		status = s.mqClient.Status()
	}

	return &blocktx_api.HealthResponse{
		Ok:        s.mqClient.IsConnected(),
		Nats:      status,
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) ClearBlocks(ctx context.Context, clearData *blocktx_api.ClearData) (*blocktx_api.RowsAffectedResponse, error) {
	_, err := s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "block_processing")
	if err != nil {
		return nil, err
	}

	return s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "blocks")
}

func (s *Server) ClearRegisteredTransactions(ctx context.Context, clearData *blocktx_api.ClearData) (*blocktx_api.RowsAffectedResponse, error) {
	return s.store.ClearBlocktxTable(ctx, clearData.GetRetentionDays(), "registered_transactions")
}

func (s *Server) VerifyMerkleRoots(ctx context.Context, req *blocktx_api.MerkleRootsVerificationRequest) (*blocktx_api.MerkleRootVerificationResponse, error) {
	return s.store.VerifyMerkleRoots(ctx, req.GetMerkleRoots(), s.maxAllowedBlockHeightMismatch)
}

func (s *Server) RegisterTransaction(_ context.Context, req *blocktx_api.Transaction) (*emptypb.Empty, error) {
	s.processor.RegisterTransaction(req.Hash)
	return &emptypb.Empty{}, nil
}

func (s *Server) RegisterTransactions(_ context.Context, req *blocktx_api.Transactions) (*emptypb.Empty, error) {
	for _, tx := range req.GetTransactions() {
		s.processor.RegisterTransaction(tx.GetHash())
	}

	return &emptypb.Empty{}, nil
}
