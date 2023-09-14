package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

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
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	gocore.NewStat("PutTransaction", true)
}

const (
	responseTimeout = 5 * time.Second
)

// Server type carries the zmqLogger within it
type Server struct {
	metamorph_api.UnimplementedMetaMorphAPIServer
	logger     utils.Logger
	processor  ProcessorI
	store      store.MetamorphStore
	timeout    time.Duration
	grpcServer *grpc.Server
	btc        blocktx.ClientI
	source     string
}

// NewServer will return a server instance with the zmqLogger stored within it
func NewServer(logger utils.Logger, s store.MetamorphStore, p ProcessorI, btc blocktx.ClientI, source string) *Server {
	return &Server{
		logger:    logger,
		processor: p,
		store:     s,
		timeout:   responseTimeout,
		btc:       btc,
		source:    source,
	}
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

	s.logger.Infof("[Metamorph] GRPC server listening on %s", address)

	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("metamorph GRPC server failed [%w]", err)
	}

	return nil
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

func validateCallbackURL(callbackURL string) error {
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

	err := validateCallbackURL(req.CallbackUrl)
	if err != nil {
		return nil, err
	}

	next, status, hash, transactionStatus, err := s.putTransactionInit(ctx, req, start)
	if err != nil {
		// if we have an error, we will return immediately
		return nil, err
	} else if transactionStatus != nil {
		// if we have a transactionStatus, we can also return immediately
		return transactionStatus, nil
	}
	// Convert gRPC req to store.StoreData struct...
	sReq := &store.StoreData{
		Hash:          hash,
		Status:        status,
		CallbackUrl:   req.CallbackUrl,
		CallbackToken: req.CallbackToken,
		MerkleProof:   req.MerkleProof,
		RawTx:         req.RawTx,
	}

	responseChannel := make(chan processor_response.StatusAndError)
	defer func() {
		close(responseChannel)
	}()

	// TODO check the context when API call ends
	s.processor.ProcessTransaction(NewProcessorRequest(ctx, sReq, responseChannel))

	next = gocore.NewStat("PutTransaction").NewStat("2: ProcessTransaction").AddTime(next)
	span2, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:Wait")
	defer span2.Finish()

	waitForStatus := req.WaitForStatus
	if waitForStatus < metamorph_api.Status_RECEIVED || waitForStatus > metamorph_api.Status_SEEN_ON_NETWORK {
		// wait for seen by default, this is the safest option
		waitForStatus = metamorph_api.Status_SEEN_ON_NETWORK
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
				Txid:     hash.String(),
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
					Txid:         hash.String(),
					RejectReason: resErr.Error(),
				}, nil
			}

			if status >= waitForStatus {
				return &metamorph_api.TransactionStatus{
					Status: status,
					Txid:   hash.String(),
				}, nil
			}
		}
	}
}

func (s *Server) PutTransactions(ctx context.Context, req *metamorph_api.TransactionRequests) (*metamorph_api.TransactionStatuses, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Server:PutTransactions")
	defer span.Finish()
	start := gocore.CurrentNanos()
	defer gocore.NewStat("PutTransactions").AddTime(start)

	// prepare response object before filling with tx statuses
	ret := &metamorph_api.TransactionStatuses{}
	ret.Statuses = make([]*metamorph_api.TransactionStatus, len(req.Transactions))

	// for each transaction if we have status in the db already set that status in the response
	// if not we store the transaction data and set the transaction status in response array to - STORED
	requests := make([]*store.StoreData, 0, len(req.Transactions))
	for ind, req := range req.Transactions {
		err := validateCallbackURL(req.CallbackUrl)
		if err != nil {
			return nil, err
		}
		_, status, hash, transactionStatus, err := s.putTransactionInit(ctx, req, start)
		if err != nil {
			// if we have an error, we will return immediately
			return nil, err
		} else if transactionStatus != nil {
			// if we have a transactionStatus, we can also return immediately
			ret.Statuses[ind] = transactionStatus
			continue
		}

		// Convert gRPC req to store.StoreData struct...
		sReq := &store.StoreData{
			Hash:          hash,
			Status:        status,
			CallbackUrl:   req.CallbackUrl,
			CallbackToken: req.CallbackToken,
			MerkleProof:   req.MerkleProof,
			RawTx:         req.RawTx,
		}

		if err := s.processor.Set(NewProcessorRequest(ctx, sReq, nil)); err != nil {
			return nil, err
		}

		// set status stored
		ret.Statuses[ind] = &metamorph_api.TransactionStatus{
			Status: metamorph_api.Status_STORED,
			Txid:   hash.String(),
		}

		// add new request to process
		requests = append(requests, sReq)
	}
	// As long as we have all the transactions in the db at this point, it's safe to continue processing them asynchronously
	// we are not going to wait for their completion, we will be returning statuses for transactions - STORED
	for _, request := range requests {
		// TODO check the context when API call ends
		go s.processor.ProcessTransaction(NewProcessorRequest(ctx, request, nil))
	}
	return ret, nil
}

func (s *Server) putTransactionInit(ctx context.Context, req *metamorph_api.TransactionRequest, start int64) (int64, metamorph_api.Status, *chainhash.Hash, *metamorph_api.TransactionStatus, error) {
	initSpan, initCtx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:init")
	defer initSpan.Finish()

	// init next variable to allow conditional functions to run, all accepting next as an argument
	next := start

	status := metamorph_api.Status_UNKNOWN
	hash := chainhash.DoubleHashH(req.RawTx)

	initSpan.SetTag("txid", hash.String())

	// Register the transaction in blocktx store
	rtr, err := s.btc.RegisterTransaction(initCtx, &blocktx_api.TransactionAndSource{
		Hash:   hash[:],
		Source: s.source,
	})

	if err != nil {
		return 0, 0, nil, nil, err
	}

	if rtr.Source != s.source {
		if isForwarded(ctx) {
			// This is a forwarded request, so we should not forward it again
			s.logger.Warnf("Endless forwarding loop detected for %v (source in blocktx = %q, my address = %q)", hash, rtr.Source, s.source)
			return 0, 0, nil, nil, fmt.Errorf("endless forwarding loop detected")
		}

		// This transaction was already registered by another metamorph, and we
		// should forward the request to that metamorph
		var ownerConn *grpc.ClientConn
		if ownerConn, err = dialMetamorph(initCtx, rtr.Source); err != nil {
			return 0, 0, nil, nil, err
		}

		defer ownerConn.Close()

		ownerMM := metamorph_api.NewMetaMorphAPIClient(ownerConn)

		var transactionStatus *metamorph_api.TransactionStatus
		transactionStatus.MerklePath = rtr.MerklePath
		if transactionStatus, err = ownerMM.PutTransaction(createForwardedContext(initCtx), req); err != nil {
			return 0, 0, nil, nil, err
		}

		return 0, 0, nil, transactionStatus, nil
	}

	if rtr.BlockHash != nil {
		// If the transaction was mined, we should mark it as such
		status = metamorph_api.Status_MINED
		blockHash, _ := chainhash.NewHash(rtr.BlockHash)
		if err = s.store.UpdateMined(initCtx, &hash, blockHash, rtr.BlockHeight); err != nil {
			return 0, 0, nil, nil, err
		}
	}

	// Check if the transaction is already in the store
	var transactionStatus *metamorph_api.TransactionStatus
	next, transactionStatus = s.checkStore(initCtx, &hash, next)
	if transactionStatus != nil {
		// just return the status if we found it in the store
		transactionStatus.MerklePath = rtr.MerklePath
		return 0, 0, nil, transactionStatus, nil
	}

	checkUtxos := viper.GetBool("metamorph.checkUtxos")
	if checkUtxos {
		next, err = s.utxoCheck(initCtx, next, req.RawTx)
		if err != nil {
			return 0, 0, nil, &metamorph_api.TransactionStatus{
				Status:       metamorph_api.Status_REJECTED,
				Txid:         hash.String(),
				MerklePath:   rtr.MerklePath,
				RejectReason: err.Error(),
			}, nil
		}
	}

	return next, status, &hash, nil, nil
}

func (s *Server) checkStore(ctx context.Context, hash *chainhash.Hash, next int64) (int64, *metamorph_api.TransactionStatus) {
	storeData, err := s.store.Get(ctx, hash[:])
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Errorf("Error getting transaction from store: %v", err)
	}
	if storeData != nil {
		// we found the transaction in the store, so we can just return it
		return 0, &metamorph_api.TransactionStatus{
			TimedOut:     false,
			StoredAt:     timestamppb.New(storeData.StoredAt),
			AnnouncedAt:  timestamppb.New(storeData.AnnouncedAt),
			MinedAt:      timestamppb.New(storeData.MinedAt),
			Txid:         fmt.Sprintf("%v", storeData.Hash),
			MerklePath:   storeData.MerklePath,
			Status:       storeData.Status,
			RejectReason: storeData.RejectReason,
			BlockHeight:  storeData.BlockHeight,
			BlockHash:    fmt.Sprintf("%v", storeData.BlockHash),
		}
	}

	return gocore.NewStat("PutTransaction").NewStat("1: Check store").AddTime(next), nil
}

func (s *Server) utxoCheck(ctx context.Context, next int64, rawTx []byte) (int64, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:UtxoCheck")
	defer span.Finish()

	peerRpcPassword := viper.GetString("peerRpc.password")
	if peerRpcPassword == "" {
		return 0, errors.Errorf("setting peerRpc.password not found")
	}

	peerRpcUser := viper.GetString("peerRpc.user")
	if peerRpcUser == "" {
		return 0, errors.Errorf("setting peerRpc.user not found")
	}

	peerRpcHost := viper.GetString("peerRpc.host")
	if peerRpcHost == "" {
		return 0, errors.Errorf("setting peerRpc.host not found")
	}

	peerRpcPort := viper.GetInt("peerRpc.port")
	if peerRpcPort == 0 {
		return 0, errors.Errorf("setting peerRpc.port not found")
	}

	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
	if err != nil {
		return 0, errors.Errorf("failed to parse rpc URL: %v", err)
	}

	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return 0, err
	}

	var tx *bt.Tx
	tx, err = bt.NewTxFromBytes(rawTx)
	if err != nil {
		s.logger.Errorf("Error creating bitcoin tx: %v", err)
		return 0, err
	}

	for _, input := range tx.Inputs {
		var utxos *bitcoin.TXOut
		utxos, err = node.GetTxOut(input.PreviousTxIDStr(), int(input.PreviousTxOutIndex), true)
		if err != nil {
			s.logger.Errorf("failed to get utxo: %v", err)
			return 0, fmt.Errorf("failed to get utxo: %v", err)
		}

		if utxos == nil {
			s.logger.Errorf("utxo %s:%d not found", input.PreviousTxIDStr(), input.PreviousTxOutIndex)
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

	return &metamorph_api.Transaction{
		Txid:         data.Hash.String(),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    data.BlockHash.String(),
		RejectReason: data.RejectReason,
		RawTx:        data.RawTx,
	}, nil
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

	return &metamorph_api.TransactionStatus{
		Txid:         data.Hash.String(),
		AnnouncedAt:  announcedAt,
		StoredAt:     storedAt,
		MinedAt:      minedAt,
		Status:       data.Status,
		BlockHeight:  data.BlockHeight,
		BlockHash:    blockHash,
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

func dialMetamorph(ctx context.Context, address string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
		grpc.WithChainStreamInterceptor(grpc_prometheus.StreamClientInterceptor),
	}

	return grpc.DialContext(ctx, address, tracing.AddGRPCDialOptions(opts)...)
}

func createForwardedContext(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(
		ctx,
		metadata.Pairs("forwarded", "true"),
	)
}

func isForwarded(ctx context.Context) bool {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		f := md.Get("forwarded")
		if len(f) > 0 && f[0] == "true" {
			return true
		}
	}

	return false
}
