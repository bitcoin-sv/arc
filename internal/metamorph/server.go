package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	chh "github.com/bsv-blockchain/go-bt/v2/chainhash"
	"github.com/bsv-blockchain/go-sdk/util"
	"github.com/ccoveille/go-safecast"
	"github.com/nats-io/nats.go"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	checkStatusIntervalDefault = 5 * time.Second
	minedDoubleSpendMsg        = "previously double spend attempted"
	deadlineExtension          = 1 * time.Second * global.MaxTimeout
	minusDeadlineExtension     = -1 * deadlineExtension
)

var ErrNotFound = errors.New("key could not be found")

type BitcoinNode interface {
	GetTxOut(txHex string, vout int, includeMempool bool) (res *bitcoin.TXOut, err error)
}

type ProcessorI interface {
	ProcessTransaction(ctx context.Context, req *ProcessorRequest)
	GetProcessorMapSize() int
	GetPeers() []p2p.PeerI
	Health() error
}

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	grpc_utils.GrpcServer

	logger              *slog.Logger
	mq                  mq.MessageQueueClient
	processor           ProcessorI
	store               store.MetamorphStore
	checkStatusInterval time.Duration
	tracingEnabled      bool
	tracingAttributes   []attribute.KeyValue
}

func WithCheckStatusInterval(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.checkStatusInterval = d
	}
}

// WithServerTracer sets the tracer to be used for tracing
func WithServerTracer(attr ...attribute.KeyValue) func(s *Server) {
	return func(s *Server) {
		s.tracingEnabled = true
		if len(attr) > 0 {
			s.tracingAttributes = append(s.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			s.tracingAttributes = append(s.tracingAttributes, attribute.String("file", file))
		}
	}
}

type ServerOption func(s *Server)

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(logger *slog.Logger, store store.MetamorphStore, processor ProcessorI, mq mq.MessageQueueClient, cfg grpc_utils.ServerConfig, opts ...ServerOption) (*Server, error) {
	logger = logger.With(slog.String("module", "server"))

	s := &Server{
		logger:              logger,
		processor:           processor,
		store:               store,
		checkStatusInterval: checkStatusIntervalDefault,
		mq:                  mq,
	}

	for _, opt := range opts {
		opt(s)
	}

	grpcServer, err := grpc_utils.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

	// register health server endpoint
	grpc_health_v1.RegisterHealthServer(grpcServer.Srv, s)

	s.GrpcServer = grpcServer

	metamorph_api.RegisterMetaMorphAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(ctx context.Context, _ *emptypb.Empty) (healthResp *metamorph_api.HealthResponse, err error) {
	_, span := tracing.StartTracing(ctx, "Health", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	processorMapSize := s.processor.GetProcessorMapSize()

	peers := s.processor.GetPeers()

	peersConnected := make([]string, 0, len(peers))
	peersDisconnected := make([]string, 0, len(peers))
	for _, peer := range peers {
		if peer.Connected() {
			peersConnected = append(peersConnected, peer.String())
		} else {
			peersDisconnected = append(peersDisconnected, peer.String())
		}
	}

	status := nats.DISCONNECTED
	if s.mq != nil {
		status = s.mq.Status()
	}

	processorMapSizeInt32, err := safecast.ToInt32(processorMapSize)
	if err != nil {
		s.logger.Error("failed to convert processor map size to int32", slog.String("err", err.Error()))
		return nil, err
	}

	return &metamorph_api.HealthResponse{
		Nats:              status.String(),
		Timestamp:         timestamppb.New(time.Now()),
		MapSize:           processorMapSizeInt32,
		PeersConnected:    strings.Join(peersConnected, ","),
		PeersDisconnected: strings.Join(peersDisconnected, ","),
	}, nil
}

func (s *Server) PostTransactions(ctx context.Context, req *metamorph_api.PostTransactionsRequest) (txsStatuses *metamorph_api.TransactionStatuses, err error) {
	ctx, span := tracing.StartTracing(ctx, "PostTransactions", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	deadline, ok := ctx.Deadline()
	if ok {
		// Create a new deadline
		newDeadline := deadline
		if time.Now().Add(deadlineExtension).Before(deadline) {
			// decrease time to get initial deadline
			newDeadline = deadline.Add(minusDeadlineExtension)
		}

		var newCancel context.CancelFunc
		ctx, newCancel = context.WithDeadline(context.WithoutCancel(ctx), newDeadline)
		defer newCancel()
	}

	// for each transaction if we have status in the db already set that status in the response
	// if not, we store the transaction data and set the transaction status in response array to - STORED
	type processTxInput struct {
		data          *global.TransactionData
		waitForStatus metamorph_api.Status
		responseIndex int
	}

	// prepare the response object before filling with tx statuses
	resp := &metamorph_api.TransactionStatuses{}
	resp.Statuses = make([]*metamorph_api.TransactionStatus, len(req.GetTransactions()))

	processTxsInputMap := make(map[chh.Hash]processTxInput)

	for ind, txReq := range req.GetTransactions() {
		statusReceived := metamorph_api.Status_RECEIVED
		hash := PtrTo(chh.DoubleHashH(txReq.GetRawTx()))

		processTxsInputMap[*hash] = processTxInput{
			data:          requestToStoreData(hash, statusReceived, txReq),
			waitForStatus: txReq.GetWaitForStatus(),
			responseIndex: ind,
		}
	}

	// Concurrently process each transaction and wait for the transaction status to return
	wg := &sync.WaitGroup{}
	for hash, input := range processTxsInputMap {
		wg.Go(func() {
			statusNew := s.processTransaction(ctx, input.waitForStatus, input.data, hash.String())

			resp.Statuses[input.responseIndex] = statusNew
		})
	}

	wg.Wait()

	return resp, nil
}

func requestToStoreData(hash *chh.Hash, statusReceived metamorph_api.Status, req *metamorph_api.PostTransactionRequest) *global.TransactionData {
	callbacks := make([]global.Callback, 0)
	if req.GetCallbackUrl() != "" || req.GetCallbackToken() != "" {
		callbacks = []global.Callback{
			{
				CallbackURL:   req.GetCallbackUrl(),
				CallbackToken: req.GetCallbackToken(),
				AllowBatch:    req.GetCallbackBatch(),
			},
		}
	}

	return &global.TransactionData{
		Hash:              hash,
		Status:            statusReceived,
		Callbacks:         callbacks,
		FullStatusUpdates: req.GetFullStatusUpdates(),
		RawTx:             req.GetRawTx(),
	}
}

func (s *Server) processTransaction(ctx context.Context, waitForStatus metamorph_api.Status, data *global.TransactionData, txID string) *metamorph_api.TransactionStatus {
	var err error
	ctx, span := tracing.StartTracing(ctx, "processTransaction", s.tracingEnabled, s.tracingAttributes...)
	returnedStatus := &metamorph_api.TransactionStatus{
		Txid:   txID,
		Status: metamorph_api.Status_RECEIVED,
	}
	updateReturnedCallbacks(data, returnedStatus)
	defer func() {
		if span != nil {
			span.SetAttributes(attribute.String("finalStatus",
				returnedStatus.Status.String()),
				attribute.String("txID", returnedStatus.Txid),
				attribute.Bool("timeout", returnedStatus.TimedOut),
				attribute.String("waitFor", waitForStatus.String()),
			)
		}
		tracing.EndTracing(span, err)
	}()

	// to avoid false negatives first check if ctx is expired
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	responseChannel := make(chan StatusAndError, 10)
	s.processor.ProcessTransaction(ctx, &ProcessorRequest{Data: data, ResponseChannel: responseChannel})

	if waitForStatus == 0 {
		// wait for seen by default, this is the safest option
		waitForStatus = metamorph_api.Status_SEEN_ON_NETWORK
	}

	// Return the status if it has greater or equal value
	if returnedStatus.GetStatus() >= waitForStatus {
		return returnedStatus
	}
	status := s.waitForTxStatus(ctx, returnedStatus, responseChannel, txID, waitForStatus)
	return status
}

func updateReturnedCallbacks(data *global.TransactionData, returnedStatus *metamorph_api.TransactionStatus) {
	for _, cb := range data.Callbacks {
		if cb.CallbackURL != "" {
			returnedStatus.Callbacks = append(returnedStatus.Callbacks, &metamorph_api.Callback{
				CallbackUrl:   cb.CallbackURL,
				CallbackToken: cb.CallbackToken,
			})
		}
	}
}

func (s *Server) GetTransaction(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (txn *metamorph_api.Transaction, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransaction", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	data, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get transaction", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	txn = &metamorph_api.Transaction{
		Txid:         data.Hash.String(),
		StoredAt:     storedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		RejectReason: data.RejectReason,
		RawTx:        data.RawTx,
	}
	if data.BlockHash != nil {
		txn.BlockHash = data.BlockHash.String()
	}

	return txn, nil
}

func (s *Server) GetTransactions(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) (txs *metamorph_api.Transactions, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactions", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	data, err := s.getTransactions(ctx, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get transactions", slog.String("err", err.Error()))
		return nil, err
	}

	res := make([]*metamorph_api.Transaction, 0, len(data))
	for _, sd := range data {
		txn := &metamorph_api.Transaction{
			Txid:         sd.Hash.String(),
			Status:       sd.Status,
			BlockHeight:  sd.BlockHeight,
			RejectReason: sd.RejectReason,
			RawTx:        sd.RawTx,
		}
		if sd.BlockHash != nil {
			txn.BlockHash = sd.BlockHash.String()
		}
		if !sd.StoredAt.IsZero() {
			txn.StoredAt = timestamppb.New(sd.StoredAt)
		}

		res = append(res, txn)
	}

	return &metamorph_api.Transactions{Transactions: res}, nil
}

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (returnStatus *metamorph_api.TransactionStatus, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatus", s.tracingEnabled, s.tracingAttributes...)

	var status metamorph_api.Status

	defer func() {
		if span != nil {
			span.SetAttributes(attribute.String("status", status.String()))
		}
		tracing.EndTracing(span, err)
	}()

	data, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		s.logger.ErrorContext(ctx, "failed to get transaction status", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	var blockHash string
	if data.BlockHash != nil {
		blockHash = data.BlockHash.String()
	}

	status = data.Status

	returnStatus = &metamorph_api.TransactionStatus{
		Txid:          data.Hash.String(),
		StoredAt:      storedAt,
		Status:        data.Status,
		BlockHeight:   data.BlockHeight,
		BlockHash:     blockHash,
		RejectReason:  data.RejectReason,
		CompetingTxs:  data.CompetingTxs,
		MerklePath:    data.MerklePath,
		LastSubmitted: timestamppb.New(data.LastSubmittedAt),
	}

	for _, cb := range data.Callbacks {
		if cb.CallbackURL != "" {
			returnStatus.Callbacks = append(returnStatus.Callbacks, &metamorph_api.Callback{
				CallbackUrl:   cb.CallbackURL,
				CallbackToken: cb.CallbackToken,
				AllowBatch:    cb.AllowBatch,
			})
		}
	}

	if returnStatus.Status == metamorph_api.Status_MINED && len(returnStatus.CompetingTxs) > 0 {
		returnStatus.CompetingTxs = []string{}
		returnStatus.RejectReason = minedDoubleSpendMsg
	}

	return returnStatus, nil
}

func (s *Server) GetTransactionStatuses(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) (returnStatus *metamorph_api.TransactionStatuses, err error) {
	ctx, span := tracing.StartTracing(ctx, "GetTransactionStatuses", s.tracingEnabled, s.tracingAttributes...)

	var status metamorph_api.Status

	defer func() {
		if span != nil {
			span.SetAttributes(attribute.String("status", status.String()))
		}
		tracing.EndTracing(span, err)
	}()
	statuses, err := s.getTransactions(ctx, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		s.logger.ErrorContext(ctx, "failed to get transaction status", slog.String("err", err.Error()))
		return nil, err
	}
	returnStatus = &metamorph_api.TransactionStatuses{
		Statuses: make([]*metamorph_api.TransactionStatus, 0),
	}
	for _, status := range statuses {
		var storedAt *timestamppb.Timestamp
		if !status.StoredAt.IsZero() {
			storedAt = timestamppb.New(status.StoredAt)
		}
		txStatus := &metamorph_api.TransactionStatus{
			Txid:          status.Hash.String(),
			StoredAt:      storedAt,
			Status:        status.Status,
			BlockHeight:   status.BlockHeight,
			RejectReason:  status.RejectReason,
			CompetingTxs:  status.CompetingTxs,
			MerklePath:    status.MerklePath,
			LastSubmitted: timestamppb.New(status.LastSubmittedAt),
		}
		if status.BlockHash != nil {
			txStatus.BlockHash = status.BlockHash.String()
		}
		for _, cb := range status.Callbacks {
			if cb.CallbackURL != "" {
				if txStatus.Callbacks == nil {
					txStatus.Callbacks = make([]*metamorph_api.Callback, 0)
				}
				txStatus.Callbacks = append(txStatus.Callbacks, &metamorph_api.Callback{
					CallbackUrl:   cb.CallbackURL,
					CallbackToken: cb.CallbackToken,
					AllowBatch:    cb.AllowBatch,
				})
			}
		}
		returnStatus.Statuses = append(returnStatus.Statuses, txStatus)
	}
	return returnStatus, nil
}

func (s *Server) getTransactionData(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*global.TransactionData, *timestamppb.Timestamp, error) {
	txBytes, err := hex.DecodeString(req.GetTxid())
	if err != nil {
		return nil, nil, err
	}

	hash := util.ReverseBytes(txBytes)

	var data *global.TransactionData
	data, err = s.store.Get(ctx, hash)
	if err != nil {
		return nil, nil, err
	}

	var storedAt *timestamppb.Timestamp
	if !data.StoredAt.IsZero() {
		storedAt = timestamppb.New(data.StoredAt)
	}

	return data, storedAt, nil
}

func (s *Server) getTransactions(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) ([]*global.TransactionData, error) {
	keys := make([][]byte, 0, len(req.TxIDs))
	for _, id := range req.TxIDs {
		idBytes, err := hex.DecodeString(id)
		if err != nil {
			return nil, err
		}

		keys = append(keys, util.ReverseBytes(idBytes))
	}

	return s.store.GetMany(ctx, keys)
}

func (s *Server) UpdateInstances(ctx context.Context, request *metamorph_api.UpdateInstancesRequest) (*emptypb.Empty, error) {
	rowsAffected, err := s.store.SetUnlockedByNameExcept(ctx, request.Instances)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	if rowsAffected > 0 {
		s.logger.Info("unlocked items", slog.Int64("items", rowsAffected))
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) ClearData(ctx context.Context, req *metamorph_api.ClearDataRequest) (result *metamorph_api.ClearDataResponse, err error) {
	ctx, span := tracing.StartTracing(ctx, "ClearData", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	recordsAffected, err := s.store.ClearData(ctx, req.RetentionDays)
	if err != nil {
		s.logger.Error("failed to clear data", slog.String("err", err.Error()))
		return nil, err
	}

	result = &metamorph_api.ClearDataResponse{
		RecordsAffected: recordsAffected,
	}

	return result, nil
}

func (s *Server) waitForTxStatus(ctx context.Context, returnedStatus *metamorph_api.TransactionStatus, responseChannel chan StatusAndError, txID string, waitForStatus metamorph_api.Status) *metamorph_api.TransactionStatus {
	ctx, span := tracing.StartTracing(ctx, "waitForTxStatus", s.tracingEnabled, s.tracingAttributes...)
	var err error
	defer func() {
		tracing.EndTracing(span, err)
	}()
	checkStatusTicker := time.NewTicker(s.checkStatusInterval)
	defer checkStatusTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Ensure that function returns at latest when context times out
			returnedStatus.TimedOut = true
			return returnedStatus
		case <-checkStatusTicker.C:
			// It's possible the transaction status was received and updated in db by another metamorph instance
			// If yes, return new tx status
			var tx *metamorph_api.TransactionStatus
			tx, err = s.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
				Txid: txID,
			})
			if err == nil && tx.Status >= waitForStatus {
				return tx
			}
		case res := <-responseChannel:
			returnedStatus.Status = res.Status

			if span != nil {
				span.AddEvent("status change", trace.WithAttributes(attribute.String("status", (*returnedStatus).Status.String())))
			}

			if len(res.CompetingTxs) > 0 {
				returnedStatus.CompetingTxs = res.CompetingTxs
			}

			if res.Err != nil {
				returnedStatus.RejectReason = res.Err.Error()
				// Note: return here so that user doesn't have to wait for timeout in case of an error
				return returnedStatus
			}
			returnedStatus.RejectReason = ""
			if res.Status == metamorph_api.Status_MINED {
				var tx *metamorph_api.TransactionStatus
				tx, err = s.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
					Txid: txID,
				})
				if err != nil {
					s.logger.Error("failed to get mined transaction from storage", slog.String("err", err.Error()))
					returnedStatus.RejectReason = err.Error()
					return returnedStatus
				}

				return tx
			}

			// Return the status if it has greater or equal value
			if returnedStatus.GetStatus() >= waitForStatus {
				return returnedStatus
			}
		}
	}
}

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}
