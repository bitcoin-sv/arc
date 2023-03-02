package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/processor_response"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/tracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/libsv/go-bt/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
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
		timeout:   5 * time.Second,
		btc:       btc,
		source:    source,
	}
}

func (s *Server) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// StartGRPCServer function
func (s *Server) StartGRPCServer(address string) error {
	// LEVEL 0 - no security / no encryption
	var opts []grpc.ServerOption
	opts = append(opts,
		grpc.ChainStreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.ChainUnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)

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
	span, ctx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction")
	defer span.Finish()

	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("PutTransaction").AddTime(start)
	}()

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
		ApiKeyId:      req.ApiKeyId,
		StandardFeeId: req.StandardFeeId,
		DataFeeId:     req.DataFeeId,
		SourceIp:      req.SourceIp,
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

func (s *Server) putTransactionInit(ctx context.Context, req *metamorph_api.TransactionRequest, start int64) (int64, metamorph_api.Status, []byte, *metamorph_api.TransactionStatus, error) {
	initSpan, initCtx := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:init")
	defer initSpan.Finish()

	// init next variable to allow conditional functions to run, all accepting next as an argument
	next := start

	status := metamorph_api.Status_UNKNOWN
	hash := utils.Sha256d(req.RawTx)

	initSpan.SetTag("txid", utils.HexEncodeAndReverseBytes(hash))

	// Register the transaction in blocktx store
	rtr, err := s.btc.RegisterTransaction(initCtx, &blocktx_api.TransactionAndSource{
		Hash:   hash,
		Source: s.source,
	})
	if err != nil {
		return 0, 0, nil, nil, err
	}

	if rtr.Source != s.source {
		if isForwarded(ctx) {
			// This is a forwarded request, so we should not forward it again
			s.logger.Warnf("Endless forwarding loop detected for %s (source in blocktx = %q, my address = %q)", utils.HexEncodeAndReverseBytes(hash), rtr.Source, s.source)
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
		if transactionStatus, err = ownerMM.PutTransaction(createForwardedContext(initCtx), req); err != nil {
			return 0, 0, nil, nil, err
		}

		return 0, 0, nil, transactionStatus, nil
	}

	if rtr.BlockHash != nil {
		// If the transaction was mined, we should mark it as such
		status = metamorph_api.Status_MINED
		if err = s.store.UpdateMined(initCtx, hash, rtr.BlockHash, rtr.BlockHeight); err != nil {
			return 0, 0, nil, nil, err
		}
	}

	// Check if the transaction is already in the store
	var transactionStatus *metamorph_api.TransactionStatus
	next, transactionStatus = s.checkStore(initCtx, hash, next)
	if transactionStatus != nil {
		// just return the status if we found it in the store
		return 0, 0, nil, transactionStatus, nil
	}

	checkUtxos := gocore.Config().GetBool("checkUtxos")
	if checkUtxos {
		next, err = s.utxoCheck(initCtx, next, req.RawTx)
		if err != nil {
			return 0, 0, nil, &metamorph_api.TransactionStatus{
				Status:       metamorph_api.Status_REJECTED,
				Txid:         utils.HexEncodeAndReverseBytes(hash),
				RejectReason: err.Error(),
			}, nil
		}
	}

	return next, status, hash, nil, nil
}

func (s *Server) checkStore(ctx context.Context, hash []byte, next int64) (int64, *metamorph_api.TransactionStatus) {
	storeData, err := s.store.Get(ctx, hash)
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
			Txid:         fmt.Sprintf("%x", bt.ReverseBytes(storeData.Hash)),
			Status:       storeData.Status,
			RejectReason: storeData.RejectReason,
			BlockHeight:  storeData.BlockHeight,
			BlockHash:    hex.EncodeToString(storeData.BlockHash),
		}
	}

	return gocore.NewStat("PutTransaction").NewStat("1: Check store").AddTime(next), nil
}

func (s *Server) utxoCheck(ctx context.Context, next int64, rawTx []byte) (int64, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "Server:PutTransaction:UtxoCheck")
	defer span.Finish()

	rpcURL, err, found := gocore.Config().GetURL("peer_1_rpc")
	if err != nil {
		s.logger.Errorf("Error getting peer_1_rpc: %v", err)
	} else if !found {
		s.logger.Errorf("peer_1_rpc not found")
	} else {
		var node *bitcoin.Bitcoind
		node, err = bitcoin.NewFromURL(rpcURL, false)
		if err != nil {
			s.logger.Errorf("Error creating bitcoin node: %v", err)
		} else {
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
					s.logger.Errorf("Error getting utxo: %v", err)
					return 0, err
				} else {
					if utxos == nil {
						s.logger.Errorf("utxo %s:%d not found", input.PreviousTxIDStr(), input.PreviousTxOutIndex)
						return 0, fmt.Errorf("utxo %s:%d not found", input.PreviousTxIDStr(), input.PreviousTxOutIndex)
					}
				}
			}
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
