package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	gocore.NewStat("PutTransaction", true)
}

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

const (
	responseTimeout = 5 * time.Second
	blocktxTimeout  = 1 * time.Second
)

var ErrNotFound = errors.New("key could not be found")

type BitcoinNode interface {
	GetTxOut(txHex string, vout int, includeMempool bool) (res *bitcoin.TXOut, err error)
}

type ProcessorI interface {
	LoadUnmined()
	ProcessTransaction(ctx context.Context, req *ProcessorRequest)
	SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error)
	GetStats(debugItems bool) *ProcessorStats
	GetPeers() ([]string, []string)
	Shutdown()
	Health() error
}

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger          *slog.Logger
	processor       ProcessorI
	store           store.MetamorphStore
	timeout         time.Duration
	grpcServer      *grpc.Server
	bitcoinNode     BitcoinNode
	forceCheckUtxos bool
	blocktxTimeout  time.Duration
}

func WithBlocktxTimeout(d time.Duration) func(*Server) {
	return func(s *Server) {
		s.blocktxTimeout = d
	}
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

type ServerOption func(s *Server)

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(s store.MetamorphStore, p ProcessorI, opts ...ServerOption) *Server {
	server := &Server{
		logger:          slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm")),
		processor:       p,
		store:           s,
		timeout:         responseTimeout,
		forceCheckUtxos: false,
		blocktxTimeout:  blocktxTimeout,
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *Server) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// StartGRPCServer function
func (s *Server) StartGRPCServer(address string, grpcMessageSize int) error {
	// LEVEL 0 - no security / no encryption
	var opts []grpc.ServerOption
	prometheusEndpoint := viper.GetString("prometheusEndpoint")
	if prometheusEndpoint != "" {
		opts = append(opts,
			grpc.ChainStreamInterceptor(grpc_prometheus.StreamServerInterceptor),
			grpc.ChainUnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
			grpc.MaxRecvMsgSize(grpcMessageSize),
		)
	}

	s.grpcServer = grpc.NewServer(tracing.AddGRPCServerOptions(opts)...)

	gocore.SetAddress(address)

	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("GRPC server failed to listen [%w]", err)
	}

	metamorph_api.RegisterMetaMorphAPIServer(s.grpcServer, s)

	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	s.logger.Info("GRPC server listening on", slog.String("address", address))

	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("metamorph GRPC server failed [%w]", err)
	}

	return nil
}

func (s *Server) Shutdown() {
	s.logger.Info("Shutting down")
	s.grpcServer.Stop()
	s.processor.Shutdown()
}

func (s *Server) Health(_ context.Context, _ *emptypb.Empty) (*metamorph_api.HealthResponse, error) {
	stats := s.processor.GetStats(false)

	peersConnected, peersDisconnected := s.processor.GetPeers()

	details := fmt.Sprintf(`Peer stats (started: %s)`, stats.StartTime.UTC().Format(time.RFC3339))
	return &metamorph_api.HealthResponse{
		Ok:                true,
		Details:           details,
		Timestamp:         timestamppb.New(time.Now()),
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

func ValidateCallbackURL(callbackURL string) error {
	if callbackURL == "" {
		return nil
	}

	_, err := url.ParseRequestURI(callbackURL)
	if err != nil {
		return fmt.Errorf("invalid URL [%w]", err)
	}

	return nil
}

func (s *Server) PutTransaction(ctx context.Context, req *metamorph_api.TransactionRequest) (*metamorph_api.TransactionStatus, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction")
	defer span.Finish()

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("PutTransaction").AddTime(start)
	}()

	err := ValidateCallbackURL(req.GetCallbackUrl())
	if err != nil {
		s.logger.Error("failed to validate callback URL", slog.String("err", err.Error()))
		return nil, err
	}
	hash := PtrTo(chainhash.DoubleHashH(req.GetRawTx()))
	status := metamorph_api.Status_RECEIVED

	// Convert gRPC req to store.StoreData struct...
	sReq := &store.StoreData{
		Hash:              hash,
		Status:            status,
		CallbackUrl:       req.GetCallbackUrl(),
		CallbackToken:     req.GetCallbackToken(),
		FullStatusUpdates: req.GetFullStatusUpdates(),
		RawTx:             req.GetRawTx(),
	}

	next := gocore.NewStat("PutTransaction").NewStat("2: ProcessTransaction").AddTime(start)
	span2, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:Wait")
	defer span2.Finish()

	defer func() {
		gocore.NewStat("PutTransaction").NewStat("3: Wait for status").AddTime(next)
	}()

	return s.processTransaction(ctx, req.GetWaitForStatus(), sReq, req.GetMaxTimeout(), hash.String()), nil
}

func (s *Server) PutTransactions(ctx context.Context, req *metamorph_api.TransactionRequests) (*metamorph_api.TransactionStatuses, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Server:PutTransactions")
	defer span.Finish()
	start := gocore.CurrentNanos()
	defer gocore.NewStat("PutTransactions").AddTime(start)

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
		err := ValidateCallbackURL(txReq.GetCallbackUrl())
		if err != nil {
			s.logger.Error("failed to validate callback URL", slog.String("err", err.Error()))
			return nil, err
		}

		status := metamorph_api.Status_RECEIVED
		hash := PtrTo(chainhash.DoubleHashH(txReq.GetRawTx()))
		timeout = txReq.GetMaxTimeout()

		// Convert gRPC req to store.StoreData struct...
		sReq := &store.StoreData{
			Hash:              hash,
			Status:            status,
			CallbackUrl:       txReq.GetCallbackUrl(),
			CallbackToken:     txReq.GetCallbackToken(),
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
	responseChannel := make(chan processor_response.StatusAndError, 1)
	defer func() {
		close(responseChannel)
	}()

	// TODO check the context when API call ends
	s.processor.ProcessTransaction(ctx, &ProcessorRequest{Data: data, ResponseChannel: responseChannel})

	if waitForStatus == 0 {
		// wait for seen by default, this is the safest option
		waitForStatus = metamorph_api.Status_SEEN_ON_NETWORK
	}

	// normally a node would respond very quickly, unless it's under heavy load
	timeDuration := s.timeout
	if timeoutSeconds > 0 {
		timeDuration = time.Second * time.Duration(timeoutSeconds)
	}

	t := time.NewTimer(timeDuration)
	returnedStatus := &metamorph_api.TransactionStatus{
		Txid:   TxID,
		Status: metamorph_api.Status_RECEIVED,
	}

	// Return the status if it has greater or equal value
	if statusValueMap[returnedStatus.GetStatus()] >= statusValueMap[waitForStatus] {
		return returnedStatus
	}

	for {
		select {
		case <-t.C:
			returnedStatus.TimedOut = true
			return returnedStatus
		case res := <-responseChannel:
			returnedStatus.Status = res.Status

			if res.Err != nil {
				returnedStatus.RejectReason = res.Err.Error()
			} else {
				returnedStatus.RejectReason = ""
			}

			// Return the status if it has greater or equal value
			if statusValueMap[returnedStatus.GetStatus()] >= statusValueMap[waitForStatus] {
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

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		s.logger.Error("failed to get transaction status", slog.String("hash", req.GetTxid()), slog.String("err", err.Error()))
		return nil, err
	}

	var blockHash string
	if data.BlockHash != nil {
		blockHash = data.BlockHash.String()
	}

	return &metamorph_api.TransactionStatus{
		Txid:         data.Hash.String(),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    blockHash,
		RejectReason: data.RejectReason,
		MerklePath:   data.MerklePath,
	}, nil
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

// TransactionOptions options passed from header when creating transactions.
type TransactionOptions struct {
	ClientID             string               `json:"client_id"`
	CallbackURL          string               `json:"callback_url,omitempty"`
	CallbackToken        string               `json:"callback_token,omitempty"`
	SkipFeeValidation    bool                 `json:"X-SkipFeeValidation,omitempty"`
	SkipScriptValidation bool                 `json:"X-SkipScriptValidation,omitempty"`
	SkipTxValidation     bool                 `json:"X-SkipTxValidation,omitempty"`
	WaitForStatus        metamorph_api.Status `json:"wait_for_status,omitempty"`
	FullStatusUpdates    bool                 `json:"full_status_updates,omitempty"`
	MaxTimeout           int                  `json:"max_timeout,omitempty"`
}
