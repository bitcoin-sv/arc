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

	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	checkStatusIntervalDefault = 5 * time.Second
	minedDoubleSpendMsg        = "previously double spend attempted"
	MaxTimeout                 = 30
)

var (
	ErrNotFound = errors.New("key could not be found")
)

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
	grpc_opts.GrpcServer

	logger              *slog.Logger
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
func NewServer(logger *slog.Logger, store store.MetamorphStore, processor ProcessorI, cfg grpc_opts.ServerConfig, opts ...ServerOption) (*Server, error) {
	logger = logger.With(slog.String("module", "server"))

	s := &Server{
		logger:              logger,
		processor:           processor,
		store:               store,
		checkStatusInterval: checkStatusIntervalDefault,
	}

	for _, opt := range opts {
		opt(s)
	}

	grpcServer, err := grpc_opts.NewGrpcServer(logger, cfg)
	if err != nil {
		return nil, err
	}

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

	return &metamorph_api.HealthResponse{
		Timestamp:         timestamppb.New(time.Now()),
		MapSize:           int32(processorMapSize),
		PeersConnected:    strings.Join(peersConnected, ","),
		PeersDisconnected: strings.Join(peersDisconnected, ","),
	}, nil
}

func (s *Server) PutTransaction(ctx context.Context, req *metamorph_api.TransactionRequest) (txStatus *metamorph_api.TransactionStatus, err error) {
	deadline, ok := ctx.Deadline()

	// decrease time to get initial deadline
	newDeadline := deadline
	if time.Now().Add(MaxTimeout * time.Second).Before(deadline) {
		newDeadline = deadline.Add(-(time.Second * MaxTimeout))
	}

	// Create a new context with the updated deadline
	if ok {
		var newCancel context.CancelFunc
		ctx, newCancel = context.WithDeadline(context.Background(), newDeadline)
		defer newCancel()
	}

	ctx, span := tracing.StartTracing(ctx, "PutTransaction", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	hash := PtrTo(chainhash.DoubleHashH(req.GetRawTx()))
	statusReceived := metamorph_api.Status_RECEIVED

	// Convert gRPC req to store.Data struct...
	sReq := toStoreData(hash, statusReceived, req)
	return s.processTransaction(ctx, req.GetWaitForStatus(), sReq, hash.String()), nil
}

func (s *Server) PutTransactions(ctx context.Context, req *metamorph_api.TransactionRequests) (txsStatuses *metamorph_api.TransactionStatuses, err error) {
	deadline, ok := ctx.Deadline()

	// decrease time to get initial deadline
	newDeadline := deadline
	if time.Now().Add(MaxTimeout * time.Second).Before(deadline) {
		newDeadline = deadline.Add(-(time.Second * MaxTimeout))
	}

	// Create a new context with the updated deadline
	if ok {
		var newCancel context.CancelFunc
		ctx, newCancel = context.WithDeadline(context.Background(), newDeadline)
		defer newCancel()
	}

	ctx, span := tracing.StartTracing(ctx, "PutTransactions", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	// for each transaction if we have status in the db already set that status in the response
	// if not we store the transaction data and set the transaction status in response array to - STORED
	type processTxInput struct {
		data          *store.Data
		waitForStatus metamorph_api.Status
		responseIndex int
	}

	// prepare response object before filling with tx statuses
	resp := &metamorph_api.TransactionStatuses{}
	resp.Statuses = make([]*metamorph_api.TransactionStatus, len(req.GetTransactions()))

	processTxsInputMap := make(map[chainhash.Hash]processTxInput)

	for ind, txReq := range req.GetTransactions() {
		statusReceived := metamorph_api.Status_RECEIVED
		hash := PtrTo(chainhash.DoubleHashH(txReq.GetRawTx()))

		processTxsInputMap[*hash] = processTxInput{
			data:          toStoreData(hash, statusReceived, txReq),
			waitForStatus: txReq.GetWaitForStatus(),
			responseIndex: ind,
		}
	}

	// Concurrently process each transaction and wait for the transaction status to return
	wg := &sync.WaitGroup{}
	for hash, input := range processTxsInputMap {
		wg.Add(1)
		go func(ctx context.Context, processTxInput processTxInput, txID string, wg *sync.WaitGroup, resp *metamorph_api.TransactionStatuses) {
			defer wg.Done()

			statusNew := s.processTransaction(ctx, processTxInput.waitForStatus, processTxInput.data, txID)

			resp.Statuses[processTxInput.responseIndex] = statusNew
		}(ctx, input, hash.String(), wg, resp)
	}

	wg.Wait()

	return resp, nil
}

func toStoreData(hash *chainhash.Hash, statusReceived metamorph_api.Status, req *metamorph_api.TransactionRequest) *store.Data {
	callbacks := make([]store.Callback, 0)
	if req.GetCallbackUrl() != "" || req.GetCallbackToken() != "" {
		callbacks = []store.Callback{
			{
				CallbackURL:   req.GetCallbackUrl(),
				CallbackToken: req.GetCallbackToken(),
				AllowBatch:    req.GetCallbackBatch(),
			},
		}
	}

	return &store.Data{
		Hash:              hash,
		Status:            statusReceived,
		Callbacks:         callbacks,
		FullStatusUpdates: req.GetFullStatusUpdates(),
		RawTx:             req.GetRawTx(),
	}
}
func (s *Server) processTransaction(ctx context.Context, waitForStatus metamorph_api.Status, data *store.Data, txID string) *metamorph_api.TransactionStatus {
	var err error
	ctx, span := tracing.StartTracing(ctx, "processTransaction", s.tracingEnabled, s.tracingAttributes...)
	returnedStatus := &metamorph_api.TransactionStatus{
		Txid:   txID,
		Status: metamorph_api.Status_RECEIVED,
	}

	for _, cb := range data.Callbacks {
		if cb.CallbackURL != "" {
			returnedStatus.Callbacks = append(returnedStatus.Callbacks, &metamorph_api.Callback{
				CallbackUrl:   cb.CallbackURL,
				CallbackToken: cb.CallbackToken,
			})
		}
	}

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

	checkStatusTicker := time.NewTicker(s.checkStatusInterval)
	defer checkStatusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Ensure that function returns at latest when context times out
			returnedStatus.TimedOut = true
			return returnedStatus
		case <-checkStatusTicker.C:
			// it's possible the transaction status was received and updated in db by another metamorph
			// check if that's the case and we have a new tx status to return
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
				span.AddEvent("status change", trace.WithAttributes(attribute.String("status", returnedStatus.Status.String())))
			}

			if len(res.CompetingTxs) > 0 {
				returnedStatus.CompetingTxs = res.CompetingTxs
			}

			if res.Err != nil {
				returnedStatus.RejectReason = res.Err.Error()
				// Note: return here so that user doesn't have to wait for timeout in case of an error
				return returnedStatus
			} else {
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
			}

			// Return the status if it has greater or equal value
			if returnedStatus.GetStatus() >= waitForStatus {
				return returnedStatus
			}
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

func (s *Server) getTransactionData(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*store.Data, *timestamppb.Timestamp, error) {
	txBytes, err := hex.DecodeString(req.GetTxid())
	if err != nil {
		return nil, nil, err
	}

	hash := util.ReverseBytes(txBytes)

	var data *store.Data
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

func (s *Server) getTransactions(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) ([]*store.Data, error) {
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

func (s *Server) SetUnlockedByName(ctx context.Context, req *metamorph_api.SetUnlockedByNameRequest) (result *metamorph_api.SetUnlockedByNameResponse, err error) {
	ctx, span := tracing.StartTracing(ctx, "SetUnlockedByName", s.tracingEnabled, s.tracingAttributes...)
	defer func() {
		tracing.EndTracing(span, err)
	}()

	recordsAffected, err := s.store.SetUnlockedByName(ctx, req.GetName())
	if err != nil {
		s.logger.Error("failed to set unlocked by name", slog.String("name", req.GetName()), slog.String("err", err.Error()))
		return nil, err
	}

	result = &metamorph_api.SetUnlockedByNameResponse{
		RecordsAffected: recordsAffected,
	}

	return result, nil
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

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}
