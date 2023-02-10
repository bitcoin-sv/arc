package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/libsv/go-bt/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	gocore.NewStat("PutTransaction", true)
}

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger     utils.Logger
	processor  ProcessorI
	store      store.MetamorphStore
	timeout    time.Duration
	grpcServer *grpc.Server
}

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(logger utils.Logger, s store.MetamorphStore, p ProcessorI) *Server {
	return &Server{
		logger:    logger,
		processor: p,
		store:     s,
		timeout:   5 * time.Second,
	}
}

func (s *Server) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// StartGRPCServer function
func (s *Server) StartGRPCServer(address string) error {
	// LEVEL 0 - no security / no encryption
	var opts []grpc.ServerOption
	s.grpcServer = grpc.NewServer(tracing.AddGRPCServerOptions(opts)...)

	gocore.SetAddress(address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	metamorph_api.RegisterMetaMorphAPIServer(s.grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	s.logger.Infof("[Metamorph] GRPC server listening on %s", address)

	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("metamorph GRPC server failed [%w]", err)
	}

	return nil
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*metamorph_api.HealthResponse, error) {
	stats := s.processor.GetStats()

	peersConnected, peersDisconnected := s.processor.GetPeers()

	details := fmt.Sprintf(`Peer stats (started: %s)`, stats.StartTime.UTC().Format(time.RFC3339))
	return &metamorph_api.HealthResponse{
		Ok:                true,
		Details:           details,
		Timestamp:         timestamppb.New(time.Now()),
		Workers:           int32(stats.WorkerCount),
		Uptime:            float32(time.Since(stats.StartTime).Milliseconds()) / 1000.0,
		Queued:            stats.QueuedCount,
		Processed:         stats.SentToNetwork.GetCount(),
		Waiting:           stats.QueueLength,
		Average:           float32(stats.SentToNetwork.GetAverageDuration().Milliseconds()),
		MapSize:           stats.ChannelMapSize,
		PeersConnected:    strings.Join(peersConnected, ","),
		PeersDisconnected: strings.Join(peersDisconnected, ","),
	}, nil
}

func (s *Server) PutTransaction(ctx context.Context, req *metamorph_api.TransactionRequest) (*metamorph_api.TransactionStatus, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("PutTransaction").AddTime(start)
	}()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction")
	defer span.Finish()

	responseChannel := make(chan StatusAndError)
	defer func() {

		close(responseChannel)
	}()

	// Convert gRPC req to store.StoreData struct...
	status := metamorph_api.Status_UNKNOWN
	hash := utils.Sha256d(req.RawTx)

	storeData, err := s.store.Get(ctx, hash)
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

	next := gocore.NewStat("PutTransaction").NewStat("1: Check store").AddTime(start)

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

	s.processor.ProcessTransaction(NewProcessorRequest(ctx, sReq, responseChannel))

	next = gocore.NewStat("PutTransaction").NewStat("2: ProcessTransaction").AddTime(next)
	span2, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:Wait")
	defer span2.Finish()

	waitForStatus := req.WaitForStatus
	if waitForStatus < metamorph_api.Status_RECEIVED || waitForStatus > metamorph_api.Status_SEEN_ON_NETWORK {
		waitForStatus = metamorph_api.Status_SENT_TO_NETWORK
	}

	defer func() {
		gocore.NewStat("PutTransaction").NewStat("3: Wait for status").AddTime(next)
	}()

	// normally a node would respond very quickly, unless it's under heavy load
	timeout := time.NewTimer(s.timeout)
	for {
		select {
		case <-timeout.C:
			return &metamorph_api.TransactionStatus{
				TimedOut: true,
				Status:   status,
				Txid:     utils.HexEncodeAndReverseBytes(hash),
			}, nil
		case res := <-responseChannel:
			resStatus := res.Status
			if resStatus != metamorph_api.Status_UNKNOWN {
				status = resStatus
			}

			resErr := res.Err
			if resErr != nil {
				return &metamorph_api.TransactionStatus{
					Status:       status,
					Txid:         utils.HexEncodeAndReverseBytes(hash),
					RejectReason: resErr.Error(),
				}, nil
			}

			if status >= waitForStatus {
				return &metamorph_api.TransactionStatus{
					Status: status,
					Txid:   utils.HexEncodeAndReverseBytes(hash),
				}, nil
			}
		}
	}
}

func (s *Server) GetTransaction(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.Transaction, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		return nil, err
	}

	return &metamorph_api.Transaction{
		Txid:         fmt.Sprintf("%x", bt.ReverseBytes(data.Hash)),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    fmt.Sprintf("%x", bt.ReverseBytes(data.BlockHash)),
		RejectReason: data.RejectReason,
		RawTx:        data.RawTx,
	}, nil
}

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		return nil, err
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

func (s *Server) getTransactionData(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*store.StoreData, *timestamppb.Timestamp, *timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	txBytes, err := hex.DecodeString(req.Txid)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	hash := bt.ReverseBytes(txBytes)

	var data *store.StoreData
	data, err = s.store.Get(ctx, hash)
	if err != nil {
		return nil, nil, nil, nil, err
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

	return data, announcedAt, minedAt, storedAt, nil
}
