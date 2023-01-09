package blocktx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/store"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

// Server type carries the logger within it
type Server struct {
	blocktx_api.UnsafeBlockTxAPIServer
	store         store.Interface
	logger        utils.Logger
	blockNotifier *BlockNotifier
}

// NewServer will return a server instance with the logger stored within it
func NewServer(storeI store.Interface, blockNotifier *BlockNotifier, logger utils.Logger) *Server {

	return &Server{
		store:         storeI,
		logger:        logger,
		blockNotifier: blockNotifier,
	}
}

// StartGRPCServer function
func (s *Server) StartGRPCServer() error {

	address, ok := gocore.Config().Get("blocktx_grpcAddress") //, "localhost:8001")
	if !ok {
		return errors.New("no blocktx_grpcAddress setting found")
	}

	// LEVEL 0 - no security / no encryption
	grpcServer := grpc.NewServer()

	gocore.SetAddress(address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	blocktx_api.RegisterBlockTxAPIServer(grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(grpcServer)

	s.logger.Infof("[BlockTx] GRPC server listening on %s", address)

	if err := grpcServer.Serve(lis); err != nil {
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

func (s *Server) LocateTransaction(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Source, error) {
	source, err := s.store.GetTransactionSource(ctx, transaction.Hash)
	if err != nil {
		return nil, err
	}

	return &blocktx_api.Source{
		Source: source,
	}, nil
}

func (s *Server) RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) (*emptypb.Empty, error) {
	err := s.store.InsertTransaction(ctx, transaction)
	return &emptypb.Empty{}, err
}

func (s *Server) GetBlockTransactions(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error) {
	return s.store.GetBlockTransactions(ctx, block)
}

func (s *Server) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Blocks, error) {
	return s.store.GetTransactionBlocks(ctx, transaction)
}

func (s *Server) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
	return s.store.GetTransactionBlock(ctx, transaction)
}

func (s *Server) GetBlockForHeight(ctx context.Context, height *blocktx_api.Height) (*blocktx_api.Block, error) {
	return s.store.GetBlockForHeight(ctx, height.Height)
}

func (s *Server) GetBlockNotificationStream(height *blocktx_api.Height, srv blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer) error {
	s.blockNotifier.NewSubscription(height, srv)
	return nil
}

func (s *Server) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	return s.store.GetMinedTransactionsForBlock(ctx, blockAndSource)
}
