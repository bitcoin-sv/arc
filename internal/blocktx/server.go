package blocktx

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-sdk/util"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

var (
	ErrFailedToGetBlockGaps    = errors.New("failed to get block gaps")
	ErrFailedToGetLatestBlocks = errors.New("failed to get latest blocks")
)

type ProcessorI interface {
	RegisterTransaction(txHash []byte)
	CurrentBlockHeight() (uint64, error)
}

type PeerManager interface {
	CountConnectedPeers() uint
	GetPeers() []p2p.PeerI
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
	retentionDays                 int
}

// NewServer will return a server instance with the logger stored within it.
func NewServer(logger *slog.Logger, store store.BlocktxStore, pm PeerManager, processor ProcessorI, cfg grpc_utils.ServerConfig, maxAllowedBlockHeightMismatch uint64, mqClient mq.MessageQueueClient, retentionDays int) (*Server, error) {
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
		retentionDays:                 retentionDays,
	}

	// register health server endpoint
	grpc_health_v1.RegisterHealthServer(grpcServer.Srv, s)

	blocktx_api.RegisterBlockTxAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*blocktx_api.HealthResponse, error) {
	status := nats.DISCONNECTED
	if s.mqClient != nil {
		status = s.mqClient.Status()
	}

	return &blocktx_api.HealthResponse{
		Ok:        s.mqClient.IsConnected(),
		Nats:      status.String(),
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

func (s *Server) AnyTransactionsMined(ctx context.Context, req *blocktx_api.Transactions) (*blocktx_api.AnyTransactionsMinedResponse, error) {
	minedTxs := make([][]byte, len(req.Transactions))
	minedStatuses := make(map[string]bool, len(req.Transactions))
	for i, v := range req.Transactions {
		minedStatuses[hex.EncodeToString(v.Hash)] = false
		minedTxs[i] = util.ReverseBytes(v.Hash)
	}

	// get mined txs and mark them as mined
	res := blocktx_api.AnyTransactionsMinedResponse{}
	txs, err := s.store.GetMinedTransactions(ctx, minedTxs)
	if err != nil {
		return &res, err
	}
	for _, tx := range txs {
		minedStatuses[hex.EncodeToString(tx.TxHash)] = true
	}

	for k, v := range minedStatuses {
		hash, _ := hex.DecodeString(k)
		res.Transactions = append(res.Transactions, &blocktx_api.IsMined{
			Hash:  hash,
			Mined: v,
		})
	}

	return &res, nil
}

func (s *Server) CurrentBlockHeight(_ context.Context, _ *emptypb.Empty) (*blocktx_api.CurrentBlockHeightResponse, error) {
	height, err := s.processor.CurrentBlockHeight()
	return &blocktx_api.CurrentBlockHeightResponse{CurrentBlockHeight: height}, err
}

func (s *Server) LatestBlocks(ctx context.Context, req *blocktx_api.NumOfLatestBlocks) (*blocktx_api.LatestBlocksResponse, error) {
	blocks, err := s.store.LatestBlocks(ctx, req.Blocks)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetLatestBlocks, err)
	}
	const (
		hoursPerDay   = 24
		blocksPerHour = 6
	)

	heightRange := s.retentionDays * hoursPerDay * blocksPerHour
	blockGaps, err := s.store.GetBlockGaps(ctx, heightRange)
	if err != nil {
		return nil, errors.Join(ErrFailedToGetBlockGaps, err)
	}

	gaps := make([]*blocktx_api.BlockGap, 0, len(blockGaps))
	for _, v := range blockGaps {
		if v.Hash == nil {
			continue
		}
		gaps = append(gaps, &blocktx_api.BlockGap{
			Hash:   v.Hash[:],
			Height: v.Height,
		})
	}

	response := &blocktx_api.LatestBlocksResponse{
		Blocks:    blocks,
		BlockGaps: gaps,
	}

	return response, nil
}
