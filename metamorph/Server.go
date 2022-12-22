package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/libsv/go-bt"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger    *gocore.Logger
	processor *Processor
	store     store.Store
}

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(logger *gocore.Logger, s store.Store, p *Processor) *Server {

	return &Server{
		logger:    logger,
		processor: p,
		store:     s,
	}
}

// StartGRPCServer function
func (s *Server) StartGRPCServer() error {

	address, ok := gocore.Config().Get("grpcAddress")
	if !ok {
		return errors.New("No grpcAddress setting found.")
	}

	// LEVEL 0 - no security / no encryption
	grpcServer := grpc.NewServer()

	gocore.SetAddress(address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	metamorph_api.RegisterMetaMorphAPIServer(grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(grpcServer)

	s.logger.Infof("Metamorph GRPC server listening on %s", address)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("Metamorph GRPC server failed [%w]", err)
	}

	return nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*metamorph_api.HealthResponse, error) {
	stats := s.processor.GetStats()

	avg := float32(0.0)
	if stats.ProcessedCount > 0 {
		avg = float32(stats.ProcessedMillis) / float32(stats.ProcessedCount)
	}

	details := fmt.Sprintf(`Peer stats (started: %s)`, stats.StartTime.UTC().Format(time.RFC3339))
	return &metamorph_api.HealthResponse{
		Ok:        true,
		Details:   details,
		Timestamp: timestamppb.New(time.Now()),
		Workers:   int32(stats.WorkerCount),
		Uptime:    float32(stats.UptimeMillis) / 1000.0,
		Queued:    stats.QueuedCount,
		Processed: stats.ProcessedCount,
		Waiting:   stats.QueueLength,
		Average:   avg,
		MapSize:   stats.ChannelMapSize,
	}, nil
}

func (s *Server) PutTransaction(_ context.Context, req *metamorph_api.TransactionRequest) (*metamorph_api.TransactionStatus, error) {
	responseChannel := make(chan ProcessorResponse)
	defer func() {
		close(responseChannel)
	}()

	// Convert gRPC req to store.StoreData struct...
	status := metamorph_api.Status_UNKNOWN
	hash := utils.Sha256d(req.RawTx)

	storeData, err := s.store.Get(context.Background(), hash)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Errorf("Error getting transaction from store: %v", err)
	}
	if storeData != nil {
		// we found the transaction in the store, so we can just return it
		return &metamorph_api.TransactionStatus{
			TimedOut:     false,
			StoredAt:     timestamppb.New(storeData.StoredAt),
			AnnouncedAt:  timestamppb.New(storeData.AnnouncedAt),
			MinedAt:      timestamppb.New(storeData.MinedAt),
			Txid:         fmt.Sprintf("%x", bt.ReverseBytes(storeData.Hash)),
			Status:       storeData.Status,
			RejectReason: storeData.RejectReason,
			BlockHeight:  storeData.BlockHeight,
			BlockHash:    hex.EncodeToString(storeData.BlockHash),
		}, nil
	}

	sReq := &store.StoreData{
		Hash:          hash,
		Status:        status,
		ApiKeyId:      req.ApiKeyId,
		StandardFeeId: req.StandardFeeId,
		DataFeeId:     req.DataFeeId,
		SourceIp:      req.SourceIp,
		CallbackUrl:   req.CallbackUrl,
		CallbackToken: req.CallbackToken,
		MerkleProof:   req.MerkleProof,
		RawTx:         req.RawTx,
	}

	s.processor.ProcessTransaction(NewProcessorRequest(sReq, responseChannel))

	// normally a node would respond very quickly, unless it's under heavy load
	timeout := time.NewTimer(5 * time.Second)
	for {
		select {
		case <-timeout.C:
			return &metamorph_api.TransactionStatus{
				TimedOut: true,
				Status:   status,
			}, nil
		case res := <-responseChannel:
			resStatus := res.GetStatus()
			if resStatus != metamorph_api.Status_UNKNOWN {
				status = resStatus
			}

			resErr := res.GetErr()
			if resErr != nil {
				return &metamorph_api.TransactionStatus{
					Status:       status,
					RejectReason: resErr.Error(),
				}, nil
			}

			if status >= metamorph_api.Status_SENT_TO_NETWORK {
				return &metamorph_api.TransactionStatus{
					Status: status,
				}, nil
			}
		}
	}
}

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	txBytes, err := hex.DecodeString(req.Txid)
	if err != nil {
		return nil, err
	}

	hash := bt.ReverseBytes(txBytes)

	var data *store.StoreData
	data, err = s.store.Get(ctx, hash)
	if err != nil {
		return nil, err
	}

	var announcedAt *timestamppb.Timestamp
	if !data.AnnouncedAt.IsZero() {
		announcedAt = timestamppb.New(data.AnnouncedAt)
	}
	var minedAt *timestamppb.Timestamp
	if !data.MinedAt.IsZero() {
		minedAt = timestamppb.New(data.MinedAt)
	}
	var storedAt *timestamppb.Timestamp
	if !data.StoredAt.IsZero() {
		storedAt = timestamppb.New(data.StoredAt)
	}
	return &metamorph_api.TransactionStatus{
		Txid:         fmt.Sprintf("%x", bt.ReverseBytes(data.Hash)),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    fmt.Sprintf("%x", bt.ReverseBytes(data.BlockHash)),
		RejectReason: data.RejectReason,
	}, nil
}
