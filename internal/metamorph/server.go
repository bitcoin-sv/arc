package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
)

const (
	maxTimeoutDefault   = 5 * time.Second
	minedDoubleSpendMsg = "previously double spend attempted"
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

	logger            *slog.Logger
	processor         ProcessorI
	store             store.MetamorphStore
	maxTimeoutDefault time.Duration
	bitcoinNode       BitcoinNode
	forceCheckUtxos   bool
	tracingEnabled    bool
}

func WithForceCheckUtxos(bitcoinNode BitcoinNode) func(*Server) {
	return func(s *Server) {
		s.bitcoinNode = bitcoinNode
		s.forceCheckUtxos = true
	}
}

func WithMaxTimeoutDefault(timeout time.Duration) func(*Server) {
	return func(s *Server) {
		s.maxTimeoutDefault = timeout
	}
}

// WithTracer sets the tracer to be used for tracing
func WithTracer() func(s *Server) {
	return func(s *Server) {
		s.tracingEnabled = true
	}
}

type ServerOption func(s *Server)

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(prometheusEndpoint string, maxMsgSize int, logger *slog.Logger,
	store store.MetamorphStore, processor ProcessorI, opts ...ServerOption) (*Server, error) {

	logger = logger.With(slog.String("module", "server"))

	s := &Server{
		logger:            logger,
		processor:         processor,
		store:             store,
		maxTimeoutDefault: maxTimeoutDefault,
		forceCheckUtxos:   false,
	}

	for _, opt := range opts {
		opt(s)
	}

	grpcServer, err := grpc_opts.NewGrpcServer(logger, "metamorph", prometheusEndpoint, maxMsgSize, true)
	if err != nil {
		return nil, err
	}

	s.GrpcServer = grpcServer

	metamorph_api.RegisterMetaMorphAPIServer(s.GrpcServer.Srv, s)
	reflection.Register(s.GrpcServer.Srv)

	return s, nil
}

func (s *Server) Health(ctx context.Context, _ *emptypb.Empty) (*metamorph_api.HealthResponse, error) {
	_, span := s.startTracing(ctx, "Health")
	defer s.endTracing(span)

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

func (s *Server) PutTransaction(ctx context.Context, req *metamorph_api.TransactionRequest) (*metamorph_api.TransactionStatus, error) {
	ctx, span := s.startTracing(ctx, "PutTransaction")
	defer s.endTracing(span)

	hash := PtrTo(chainhash.DoubleHashH(req.GetRawTx()))
	statusReceived := metamorph_api.Status_RECEIVED

	// Convert gRPC req to store.Data struct...
	sReq := toStoreData(hash, statusReceived, req)
	return s.processTransaction(ctx, req.GetWaitForStatus(), sReq, req.GetMaxTimeout(), hash.String()), nil
}

func (s *Server) PutTransactions(ctx context.Context, req *metamorph_api.TransactionRequests) (*metamorph_api.TransactionStatuses, error) {
	ctx, span := s.startTracing(ctx, "PutTransactions")
	defer s.endTracing(span)

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
	var timeout int64

	for ind, txReq := range req.GetTransactions() {
		statusReceived := metamorph_api.Status_RECEIVED
		hash := PtrTo(chainhash.DoubleHashH(txReq.GetRawTx()))
		timeout = txReq.GetMaxTimeout()

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
		// TODO check the Context when API call ends
		go func(ctx context.Context, processTxInput processTxInput, txID string, wg *sync.WaitGroup, resp *metamorph_api.TransactionStatuses) {
			defer wg.Done()

			statusNew := s.processTransaction(ctx, processTxInput.waitForStatus, processTxInput.data, timeout, txID)

			resp.Statuses[processTxInput.responseIndex] = statusNew
		}(ctx, input, hash.String(), wg, resp)
	}

	wg.Wait()

	return resp, nil
}

func toStoreData(hash *chainhash.Hash, statusReceived metamorph_api.Status, req *metamorph_api.TransactionRequest) *store.Data {
	return &store.Data{
		Hash:   hash,
		Status: statusReceived,
		Callbacks: []store.Callback{{
			CallbackURL:   req.GetCallbackUrl(),
			CallbackToken: req.GetCallbackToken(),
			AllowBatch:    req.GetCallbackBatch(),
		}},
		FullStatusUpdates: req.GetFullStatusUpdates(),
		RawTx:             req.GetRawTx(),
	}
}
func (s *Server) processTransaction(ctx context.Context, waitForStatus metamorph_api.Status, data *store.Data, timeoutSeconds int64, txID string) *metamorph_api.TransactionStatus {
	ctx, span := s.startTracing(ctx, "processTransaction")
	defer s.endTracing(span)

	responseChannel := make(chan StatusAndError, 10)

	// normally a node would respond very quickly, unless it's under heavy load
	timeDuration := s.maxTimeoutDefault
	if timeoutSeconds > 0 {
		timeDuration = time.Second * time.Duration(timeoutSeconds)
	}

	ctx, cancel := context.WithTimeout(ctx, timeDuration)
	defer func() {
		cancel()
		close(responseChannel)
	}()

	s.processor.ProcessTransaction(ctx, &ProcessorRequest{Data: data, ResponseChannel: responseChannel})

	if waitForStatus == 0 {
		// wait for seen by default, this is the safest option
		waitForStatus = metamorph_api.Status_SEEN_ON_NETWORK
	}

	returnedStatus := &metamorph_api.TransactionStatus{
		Txid:   txID,
		Status: metamorph_api.Status_RECEIVED,
	}

	// Return the status if it has greater or equal value
	if returnedStatus.GetStatus() >= waitForStatus {
		return returnedStatus
	}

	for {
		select {
		case <-ctx.Done():
			// Ensure that function returns at latest when context times out
			returnedStatus.TimedOut = true
			return returnedStatus
		case res := <-responseChannel:
			returnedStatus.Status = res.Status

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
					tx, err := s.GetTransactionStatus(ctx, &metamorph_api.TransactionStatusRequest{
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

func (s *Server) GetTransaction(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.Transaction, error) {
	ctx, span := s.startTracing(ctx, "GetTransaction")
	defer s.endTracing(span)

	data, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to get transaction", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	txn := &metamorph_api.Transaction{
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

func (s *Server) GetTransactions(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) (*metamorph_api.Transactions, error) {
	ctx, span := s.startTracing(ctx, "GetTransactions")
	defer s.endTracing(span)

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

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	ctx, span := s.startTracing(ctx, "GetTransactionStatus")
	defer s.endTracing(span)

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

	returnStatus := &metamorph_api.TransactionStatus{
		Txid:         data.Hash.String(),
		StoredAt:     storedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    blockHash,
		RejectReason: data.RejectReason,
		CompetingTxs: data.CompetingTxs,
		MerklePath:   data.MerklePath,
	}

	if returnStatus.Status == metamorph_api.Status_MINED && len(returnStatus.CompetingTxs) > 0 {
		returnStatus.CompetingTxs = []string{}
		returnStatus.RejectReason = minedDoubleSpendMsg
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

func (s *Server) SetUnlockedByName(ctx context.Context, req *metamorph_api.SetUnlockedByNameRequest) (*metamorph_api.SetUnlockedByNameResponse, error) {
	ctx, span := s.startTracing(ctx, "SetUnlockedByName")
	defer s.endTracing(span)

	recordsAffected, err := s.store.SetUnlockedByName(ctx, req.GetName())
	if err != nil {
		s.logger.Error("failed to set unlocked by name", slog.String("name", req.GetName()), slog.String("err", err.Error()))
		return nil, err
	}

	result := &metamorph_api.SetUnlockedByNameResponse{
		RecordsAffected: recordsAffected,
	}

	return result, nil
}

func (s *Server) ClearData(ctx context.Context, req *metamorph_api.ClearDataRequest) (*metamorph_api.ClearDataResponse, error) {
	ctx, span := s.startTracing(ctx, "ClearData")
	defer s.endTracing(span)

	recordsAffected, err := s.store.ClearData(ctx, req.RetentionDays)
	if err != nil {
		s.logger.Error("failed to clear data", slog.String("err", err.Error()))
		return nil, err
	}

	result := &metamorph_api.ClearDataResponse{
		RecordsAffected: recordsAffected,
	}

	return result, nil
}

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

func (s *Server) startTracing(ctx context.Context, spanName string) (context.Context, trace.Span) {
	if s.tracingEnabled {
		var span trace.Span
		ctx, span = otel.Tracer("").Start(ctx, spanName)
		return ctx, span
	}
	return ctx, nil
}

func (s *Server) endTracing(span trace.Span) {
	if span != nil {
		span.End()
	}
}
