package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/grpc_opts"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-bitcoin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

const (
	maxTimeoutDefault   = 5 * time.Second
	minedDoubleSpendMsg = "previously double spend attempted"
)

var ErrNotFound = errors.New("key could not be found")

type BitcoinNode interface {
	GetTxOut(txHex string, vout int, includeMempool bool) (res *bitcoin.TXOut, err error)
}

type ProcessorI interface {
	ProcessTransaction(req *ProcessorRequest)
	GetProcessorMapSize() int
	GetStatusNotSeen() int64
	GetPeers() []p2p.PeerI
	Shutdown()
	Health() error
}

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger            *slog.Logger
	processor         ProcessorI
	store             store.MetamorphStore
	maxTimeoutDefault time.Duration
	grpcServer        *grpc.Server
	bitcoinNode       BitcoinNode
	forceCheckUtxos   bool
	cleanup           func()
}

func WithLogger(logger *slog.Logger) func(*Server) {
	return func(s *Server) {
		s.logger = logger
	}
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

type ServerOption func(s *Server)

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(s store.MetamorphStore, p ProcessorI, opts ...ServerOption) *Server {
	server := &Server{
		logger:            slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm")),
		processor:         p,
		store:             s,
		maxTimeoutDefault: maxTimeoutDefault,
		forceCheckUtxos:   false,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// StartGRPCServer function
func (s *Server) StartGRPCServer(address string, grpcMessageSize int, prometheusEndpoint string, logger *slog.Logger) error {
	// LEVEL 0 - no security / no encryption

	srvMetrics, opts, cleanup, err := grpc_opts.GetGRPCServerOpts(logger, prometheusEndpoint, grpcMessageSize, "metamorph")
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

	metamorph_api.RegisterMetaMorphAPIServer(s.grpcServer, s)

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

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.processor.Shutdown()
	s.grpcServer.GracefulStop()
	s.grpcServer.Stop()
	s.cleanup()
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*metamorph_api.HealthResponse, error) {
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
	hash := PtrTo(chainhash.DoubleHashH(req.GetRawTx()))
	statusReceived := metamorph_api.Status_RECEIVED

	// Convert gRPC req to store.StoreData struct...
	sReq := &store.StoreData{
		Hash:   hash,
		Status: statusReceived,
		Callbacks: []store.StoreCallback{{
			CallbackURL:   req.GetCallbackUrl(),
			CallbackToken: req.GetCallbackToken(),
		}},
		FullStatusUpdates: req.GetFullStatusUpdates(),
		RawTx:             req.GetRawTx(),
	}

	return s.processTransaction(ctx, req.GetWaitForStatus(), sReq, req.GetMaxTimeout(), hash.String()), nil
}

func (s *Server) PutTransactions(ctx context.Context, req *metamorph_api.TransactionRequests) (*metamorph_api.TransactionStatuses, error) {
	// for each transaction if we have status in the db already set that status in the response
	// if not we store the transaction data and set the transaction status in response array to - STORED
	type processTxInput struct {
		data          *store.StoreData
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

		// Convert gRPC req to store.StoreData struct...
		sReq := &store.StoreData{
			Hash:   hash,
			Status: statusReceived,
			Callbacks: []store.StoreCallback{{
				CallbackURL:   txReq.GetCallbackUrl(),
				CallbackToken: txReq.GetCallbackToken(),
			}},
			FullStatusUpdates: txReq.GetFullStatusUpdates(),
			RawTx:             txReq.GetRawTx(),
		}

		processTxsInputMap[*hash] = processTxInput{
			data:          sReq,
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

func (s *Server) processTransaction(ctx context.Context, waitForStatus metamorph_api.Status, data *store.StoreData, timeoutSeconds int64, TxID string) *metamorph_api.TransactionStatus {
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

	// TODO check the context when API call ends
	s.processor.ProcessTransaction(&ProcessorRequest{Ctx: ctx, Data: data, ResponseChannel: responseChannel})

	if waitForStatus == 0 {
		// wait for seen by default, this is the safest option
		waitForStatus = metamorph_api.Status_SEEN_ON_NETWORK
	}

	returnedStatus := &metamorph_api.TransactionStatus{
		Txid:   TxID,
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
						Txid: TxID,
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
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		s.logger.Error("failed to get transaction", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	txn := &metamorph_api.Transaction{
		Txid:         data.Hash.String(),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
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
	data, err := s.getTransactions(ctx, req)
	if err != nil {
		s.logger.Error("failed to get transactions", slog.String("err", err.Error()))
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
		if !sd.AnnouncedAt.IsZero() {
			txn.AnnouncedAt = timestamppb.New(sd.AnnouncedAt)
		}
		if !sd.MinedAt.IsZero() {
			txn.MinedAt = timestamppb.New(sd.MinedAt)
		}
		if !sd.StoredAt.IsZero() {
			txn.StoredAt = timestamppb.New(sd.StoredAt)
		}

		res = append(res, txn)
	}

	return &metamorph_api.Transactions{Transactions: res}, nil
}

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		s.logger.Error("failed to get transaction status", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	var blockHash string
	if data.BlockHash != nil {
		blockHash = data.BlockHash.String()
	}

	returnStatus := &metamorph_api.TransactionStatus{
		Txid:         data.Hash.String(),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
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

func (s *Server) getTransactionData(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*store.StoreData, *timestamppb.Timestamp, *timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	txBytes, err := hex.DecodeString(req.GetTxid())
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

func (s *Server) getTransactions(ctx context.Context, req *metamorph_api.TransactionsStatusRequest) ([]*store.StoreData, error) {
	keys := make([][]byte, 0, len(req.TxIDs))
	for _, id := range req.TxIDs {

		idBytes, err := hex.DecodeString(id)
		if err != nil {
			return nil, err
		}

		keys = append(keys, bt.ReverseBytes(idBytes))
	}

	return s.store.GetMany(ctx, keys)
}

func (s *Server) SetUnlockedByName(ctx context.Context, req *metamorph_api.SetUnlockedByNameRequest) (*metamorph_api.SetUnlockedByNameResponse, error) {
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
