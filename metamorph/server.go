package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/blocktx"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
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
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	gocore.NewStat("PutTransaction", true)
}

const (
	responseTimeout = 5 * time.Second
	blocktxTimeout  = 1 * time.Second
)

var (
	ErrRegisterTxTimeout = errors.New("failed to register transaction due to timeout")
)

type BitcoinNode interface {
	GetTxOut(txHex string, vout int, includeMempool bool) (res *bitcoin.TXOut, err error)
}

type ProcessorI interface {
	LoadUnmined()
	Set(ctx context.Context, req *ProcessorRequest) error
	ProcessTransaction(ctx context.Context, req *ProcessorRequest)
	SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error)
	SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error)
	GetStats(debugItems bool) *ProcessorStats
	GetPeers() ([]string, []string)
	Shutdown()
}

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger          *slog.Logger
	processor       ProcessorI
	store           store.MetamorphStore
	timeout         time.Duration
	grpcServer      *grpc.Server
	btc             blocktx.ClientI
	source          string
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
		s.logger = logger.With(slog.String("service", "mtm"))
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
func NewServer(s store.MetamorphStore, p ProcessorI, btc blocktx.ClientI, source string, opts ...ServerOption) *Server {
	server := &Server{
		logger:          slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevelDefault})).With(slog.String("service", "mtm")),
		processor:       p,
		store:           s,
		timeout:         responseTimeout,
		btc:             btc,
		source:          source,
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
		return nil, err
	}
	hash := handler.PtrTo(chainhash.DoubleHashH(req.GetRawTx()))

	next, status, transactionStatus, err := s.putTransactionInit(ctx, req, start)
	if err != nil {
		s.logger.Error("failed to initialize transaction", slog.String("err", err.Error()))
	}

	if transactionStatus != nil {
		// if we have a transactionStatus, no need to process it
		return transactionStatus, nil
	}

	// Convert gRPC req to store.StoreData struct...
	sReq := &store.StoreData{
		Hash:          hash,
		Status:        status,
		CallbackUrl:   req.GetCallbackUrl(),
		CallbackToken: req.GetCallbackToken(),
		MerkleProof:   req.GetMerkleProof(),
		RawTx:         req.GetRawTx(),
	}

	next = gocore.NewStat("PutTransaction").NewStat("2: ProcessTransaction").AddTime(next)
	span2, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:Wait")
	defer span2.Finish()

	defer func() {
		gocore.NewStat("PutTransaction").NewStat("3: Wait for status").AddTime(next)
	}()

	return s.processTransaction(ctx, req.GetWaitForStatus(), sReq, hash.String()), nil
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
	skipTxInit := false

	for ind, txReq := range req.GetTransactions() {
		err := ValidateCallbackURL(txReq.GetCallbackUrl())
		if err != nil {
			return nil, err
		}

		status := metamorph_api.Status_UNKNOWN
		hash := handler.PtrTo(chainhash.DoubleHashH(txReq.GetRawTx()))

		var transactionStatus *metamorph_api.TransactionStatus

		if !skipTxInit {
			_, status, transactionStatus, err = s.putTransactionInit(ctx, txReq, start)
			if err != nil {
				// If the initialization step times out once, then don't repeat for subsequent transactions in the batch as it may consume a lot of time
				if errors.Is(err, ErrRegisterTxTimeout) {
					skipTxInit = true
				}

				s.logger.Error("failed to register transaction", slog.String("err", err.Error()))
			}
			if transactionStatus != nil {
				// if we have a transactionStatus, no need to process it another time
				resp.Statuses[ind] = transactionStatus
				continue
			}
		}

		// Convert gRPC req to store.StoreData struct...
		sReq := &store.StoreData{
			Hash:          hash,
			Status:        status,
			CallbackUrl:   txReq.GetCallbackUrl(),
			CallbackToken: txReq.GetCallbackToken(),
			MerkleProof:   txReq.GetMerkleProof(),
			RawTx:         txReq.GetRawTx(),
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

			statusNew := s.processTransaction(ctx, processTxInput.waitForStatus, processTxInput.data, txID)

			resp.Statuses[processTxInput.responseIndex] = statusNew
		}(ctx, input, hash.String(), wg, resp)
	}

	wg.Wait()

	return resp, nil
}

func (s *Server) processTransaction(ctx context.Context, waitForStatus metamorph_api.Status, data *store.StoreData, TxID string) *metamorph_api.TransactionStatus {
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
	timeout := time.NewTimer(s.timeout)
	returnedStatus := &metamorph_api.TransactionStatus{Txid: TxID}

	for {
		select {
		case <-timeout.C:
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

func (s *Server) registerTransaction(ctx context.Context, hash chainhash.Hash) (*blocktx_api.RegisterTransactionResponse, error) {
	responseCh := make(chan *blocktx_api.RegisterTransactionResponse, 1)
	errCh := make(chan error, 1)
	blocktxCtx, cancel := context.WithTimeout(ctx, s.blocktxTimeout)
	defer cancel()

	go func() {
		rtr, err := s.btc.RegisterTransaction(blocktxCtx, &blocktx_api.TransactionAndSource{
			Hash:   hash[:],
			Source: s.source,
		})
		if err != nil {
			errCh <- err
			return
		}

		responseCh <- rtr
	}()

	select {
	case <-blocktxCtx.Done():
		return nil, ErrRegisterTxTimeout
	case err := <-errCh:
		return nil, fmt.Errorf("failed to register transaction: %v", err)
	case response := <-responseCh:
		return response, nil
	}
}

func (s *Server) putTransactionInit(ctx context.Context, req *metamorph_api.TransactionRequest, start int64) (int64, metamorph_api.Status, *metamorph_api.TransactionStatus, error) {
	initSpan, initCtx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:init")
	defer initSpan.Finish()

	// init next variable to allow conditional functions to run, all accepting next as an argument
	next := start

	status := metamorph_api.Status_UNKNOWN
	hash := chainhash.DoubleHashH(req.GetRawTx())

	initSpan.SetTag("txid", hash.String())

	// Register the transaction in blocktx store
	rtr, err := s.registerTransaction(ctx, hash)
	if err != nil {
		return 0, 0, nil, err
	}

	if rtr.BlockHash != nil {
		// In case a transaction is submitted to network outside of ARC and mined and now
		// submitting the same transaction through arc endpoint we have problem.
		// The transaction doesn't exist in metamorph so the call after this line
		// (updating tx status to MINED) will fail as there is no transaction to update.
		// In that case we take the transaction and store it first in metamorph database.
		if err := s.store.Set(ctx, hash[:], &store.StoreData{
			Hash:          &hash,
			Status:        status,
			CallbackUrl:   req.GetCallbackUrl(),
			CallbackToken: req.GetCallbackToken(),
			MerkleProof:   req.GetMerkleProof(),
			RawTx:         req.GetRawTx(),
		}); err != nil {
			s.logger.Error("Failed to store transaction", slog.String("hash", hash.String()), slog.String("err", err.Error()))
		}

		s.logger.Debug("Found block for transaction", slog.String("hash", hash.String()))

		// If the transaction was mined, we should mark it as such
		status = metamorph_api.Status_MINED
		blockHash, _ := chainhash.NewHash(rtr.GetBlockHash())
		if err = s.store.UpdateMined(initCtx, &hash, blockHash, rtr.GetBlockHeight()); err != nil {
			return 0, 0, nil, err
		}
	}

	// Check if the transaction is already in the store
	var transactionStatus *metamorph_api.TransactionStatus
	next, transactionStatus = s.checkStore(initCtx, &hash, next)
	if transactionStatus != nil {
		// just return the status if we found it in the store
		transactionStatus.MerklePath = rtr.GetMerklePath()
		return 0, 0, transactionStatus, nil
	}

	if s.forceCheckUtxos {
		next, err = s.CheckUtxos(initCtx, next, req.GetRawTx())
		s.logger.Error("Error checking utxos", slog.String("err", err.Error()))
		if err != nil {
			return 0, 0, &metamorph_api.TransactionStatus{
				Status:       metamorph_api.Status_REJECTED,
				Txid:         hash.String(),
				RejectReason: err.Error(),
			}, nil
		}
	}

	return next, status, nil, nil
}

func (s *Server) checkStore(ctx context.Context, hash *chainhash.Hash, next int64) (int64, *metamorph_api.TransactionStatus) {
	storeData, err := s.store.Get(ctx, hash[:])
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("Error getting transaction from store", slog.String("err", err.Error()))
	}
	if storeData != nil {
		// we found the transaction in the store, so we can just return it
		return 0, &metamorph_api.TransactionStatus{
			TimedOut:     false,
			StoredAt:     timestamppb.New(storeData.StoredAt),
			AnnouncedAt:  timestamppb.New(storeData.AnnouncedAt),
			MinedAt:      timestamppb.New(storeData.MinedAt),
			Txid:         fmt.Sprintf("%v", storeData.Hash),
			Status:       storeData.Status,
			RejectReason: storeData.RejectReason,
			BlockHeight:  storeData.BlockHeight,
			BlockHash:    fmt.Sprintf("%v", storeData.BlockHash),
		}
	}

	return gocore.NewStat("PutTransaction").NewStat("1: Check store").AddTime(next), nil
}

func (s *Server) CheckUtxos(ctx context.Context, next int64, rawTx []byte) (int64, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:UtxoCheck")
	defer span.Finish()

	var tx *bt.Tx
	tx, err := bt.NewTxFromBytes(rawTx)
	if err != nil {
		return 0, fmt.Errorf("failed to create bitcoin tx: %v", err)
	}

	for _, input := range tx.Inputs {
		var utxos *bitcoin.TXOut
		utxos, err = s.bitcoinNode.GetTxOut(input.PreviousTxIDStr(), int(input.PreviousTxOutIndex), true)
		if err != nil {
			return 0, fmt.Errorf("failed to get utxo: %v", err)
		}

		if utxos == nil {
			return 0, fmt.Errorf("utxo %s:%d not found", input.PreviousTxIDStr(), input.PreviousTxOutIndex)
		}
	}

	return gocore.NewStat("PutTransaction").NewStat("0: Check utxos").AddTime(next), nil
}

func (s *Server) GetTransaction(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.Transaction, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
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

func (s *Server) getMerklePath(ctx context.Context, hash *chainhash.Hash, dataStatus metamorph_api.Status) (string, error) {
	merklePathCh := make(chan string, 1)
	errCh := make(chan error, 1)
	blocktxCtx, cancel := context.WithTimeout(ctx, s.blocktxTimeout)
	defer cancel()

	go func() {
		mp, err := s.btc.GetTransactionMerklePath(blocktxCtx, &blocktx_api.Transaction{Hash: hash[:]})
		if err != nil {
			if errors.Is(err, blocktx.ErrTransactionNotFoundForMerklePath) {
				if dataStatus == metamorph_api.Status_MINED {
					errCh <- fmt.Errorf("merkle path not found for mined transaction %s: %v", hash.String(), err)
					return
				}
			} else {
				errCh <- fmt.Errorf("failed to get Merkle path for transaction %s: %v", hash.String(), err)
				return
			}
		}

		merklePathCh <- mp
	}()

	select {
	case <-blocktxCtx.Done():
		return "", errors.New("failed to get Merkle path due to timeout")
	case err := <-errCh:
		return "", fmt.Errorf("failed to get Merkle path: %v", err)
	case merklePath := <-merklePathCh:
		return merklePath, nil
	}
}

func (s *Server) GetTransactionStatus(ctx context.Context, req *metamorph_api.TransactionStatusRequest) (*metamorph_api.TransactionStatus, error) {
	data, announcedAt, minedAt, storedAt, err := s.getTransactionData(ctx, req)
	if err != nil {
		return nil, err
	}

	var blockHash string
	if data.BlockHash != nil {
		blockHash = data.BlockHash.String()
	}

	hash, err := chainhash.NewHashFromStr(req.GetTxid())
	if err != nil {
		return nil, err
	}
	merklePath, err := s.getMerklePath(ctx, hash, data.Status)
	if err != nil {
		s.logger.Error("failed to get merkle path")
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
		MerklePath:   merklePath,
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
		return nil, err
	}

	result := &metamorph_api.SetUnlockedByNameResponse{
		RecordsAffected: int32(recordsAffected),
	}

	return result, err
}
