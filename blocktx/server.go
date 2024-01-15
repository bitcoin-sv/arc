package blocktx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server type carries the logger within it.
type Server struct {
	blocktx_api.UnsafeBlockTxAPIServer
	store         store.Interface
	logger        *slog.Logger
	blockNotifier *BlockNotifier
	grpcServer    *grpc.Server
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(storeI store.Interface, blockNotifier *BlockNotifier, logger *slog.Logger) *Server {
	return &Server{
		store:         storeI,
		logger:        logger,
		blockNotifier: blockNotifier,
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

	s.grpcServer = grpc.NewServer(tracing.AddGRPCServerOptions(opts)...)

	gocore.SetAddress(address)

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

func (s *Server) GetTransactionMerklePath(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.MerklePath, error) {
	hash, err := chainhash.NewHash(transaction.GetHash())
	if err != nil {
		return nil, err
	}

	merklePath, err := s.store.GetTransactionMerklePath(ctx, hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrTransactionNotFoundForMerklePath
		}
		return nil, err
	}

	return &blocktx_api.MerklePath{MerklePath: merklePath}, nil
}

func (s *Server) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*blocktx_api.RegisterTransactionResponse, error) {
	source, merklePath, hash, height, err := s.store.RegisterTransaction(ctx, transaction)
	return &blocktx_api.RegisterTransactionResponse{
		Source:      source,
		MerklePath:  merklePath,
		BlockHash:   hash,
		BlockHeight: height,
	}, err
}

func (s *Server) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transactions) (*blocktx_api.TransactionBlocks, error) {
	return s.store.GetTransactionBlocks(ctx, transaction)
}

func (s *Server) GetBlockForHeight(ctx context.Context, height *blocktx_api.Height) (*blocktx_api.Block, error) {
	return s.store.GetBlockForHeight(ctx, height.GetHeight())
}

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.grpcServer.Stop()
}
