package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/pkg/message_queue"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/mcast"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

const (
	DbModePostgres = "postgres"
	chanBufferSize = 4000
)

func StartMetamorph(logger *slog.Logger, mtmCfg *config.MetamorphConfig, commonCfg *config.CommonConfig) (func(), error) {
	logger = logger.With(slog.String("service", "mtm"))
	logger.Info("Starting")

	cacheStore, err := metamorph.NewCacheStore(commonCfg.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache store: %v", err)
	}

	shutdownFns, optsServer, processorOpts, bcMediatorOpts := enableTracing(commonCfg, logger)

	var stoppable global.Stoppables
	var stoppableWithError global.StoppablesWithError

	stopFn := func() {
		logger.Info("Shutting down metamorph")
		for _, shutdownFn := range shutdownFns {
			shutdownFn()
		}
		stoppable.Shutdown()
		stoppableWithError.Shutdown(logger)
		logger.Info("Shutdown metamorph complete")
	}

	metamorphStore, err := newMetamorphStore(mtmCfg.Db, commonCfg.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to create metamorph store: %v", err)
	}
	stoppableWithError = append(stoppableWithError, metamorphStore)

	bcMediator, messenger, pm, multicaster, statusMessageCh, err := setupMtmBcNetworkCommunication(logger, metamorphStore, mtmCfg, mtmCfg.Health.MinimumHealthyConnections, bcMediatorOpts)
	if err != nil {
		stopFn()
		return nil, err
	}

	stoppable = append(stoppable, messenger)
	stoppable = append(stoppable, pm)
	stoppable = append(stoppable, multicaster)

	mqOpts := getMtmMqOpts(logger)
	mqClient, err := mq.NewMqClient(logger, commonCfg.MessageQueue, mqOpts...)
	if err != nil {
		stopFn()
		return nil, err
	}
	stoppable = append(stoppable, mqClient)

	minedTxsChan := make(chan *blocktx_api.TransactionBlocks, chanBufferSize)
	submittedTxsChan := make(chan *metamorph_api.PostTransactionRequest, chanBufferSize)

	subscribeAdapter := metamorph.NewMessageSubscribeAdapter(mqClient, logger)
	stoppable = append(stoppable, subscribeAdapter)
	err = subscribeAdapter.Start(minedTxsChan, submittedTxsChan)
	if err != nil {
		stopFn()
		return nil, err
	}
	callbackerChan := make(chan *callbacker_api.SendRequest, chanBufferSize)
	publishCallbackAdapter := message_queue.NewPublishAdapter(mqClient, logger)
	stoppable = append(stoppable, publishCallbackAdapter)
	publishCallbackAdapter.StartPublishMarshal(mq.CallbackTopic)
	go func() {
		for msg := range callbackerChan {
			publishCallbackAdapter.Publish(msg)
		}
	}()

	registerTxsChan := make(chan *blocktx_api.Transactions, chanBufferSize)
	publishRegisterTxsAdapter := message_queue.NewPublishAdapter(mqClient, logger)
	stoppable = append(stoppable, publishRegisterTxsAdapter)
	publishRegisterTxsAdapter.StartPublishMarshal(mq.RegisterTxsTopic)
	go func() {
		for msg := range registerTxsChan {
			publishRegisterTxsAdapter.Publish(msg)
		}
	}()

	registerTxChan := make(chan []byte, chanBufferSize)
	publishRegisterTxAdapter := message_queue.NewPublishCoreAdapter(mqClient, logger)
	stoppable = append(stoppable, publishRegisterTxAdapter)
	publishRegisterTxAdapter.StartPublishCore(mq.RegisterTxTopic)
	go func() {
		for msg := range registerTxChan {
			publishRegisterTxAdapter.PublishCore(msg)
		}
	}()

	procLogger := logger.With(slog.String("module", "mtm-proc"))

	btcConn, err := grpc_utils.DialGRPC(mtmCfg.BlocktxDialAddr, commonCfg.Prometheus.Endpoint, commonCfg.GrpcMessageSize, commonCfg.Tracing)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to blocktx server: %v", err)
	}
	blockTxClient := blocktx.NewClient(blocktx_api.NewBlockTxAPIClient(btcConn))

	processorOpts = append(processorOpts,
		metamorph.WithReBroadcastExpiration(commonCfg.ReBroadcastExpiration),
		metamorph.WithReAnnounceUnseenInterval(mtmCfg.ReAnnounceUnseenInterval),
		metamorph.WithReAnnounceSeenLastConfirmedAgo(mtmCfg.ReAnnounceSeen.LastConfirmedAgo),
		metamorph.WithReAnnounceSeenPendingSince(mtmCfg.ReAnnounceSeen.PendingSince),
		metamorph.WithReRegisterSeen(mtmCfg.ReRegisterSeen),
		metamorph.WithRejectPendingSeenEnabled(mtmCfg.RejectPendingSeen.Enabled),
		metamorph.WithRejectPendingSeenLastRequestedAgo(mtmCfg.RejectPendingSeen.LastRequestedAgo),
		metamorph.WithRejectPendingBlocksSince(mtmCfg.RejectPendingSeen.BlocksSince),
		metamorph.WithProcessorLogger(procLogger),
		metamorph.WithCallbackChan(callbackerChan),
		metamorph.WithRegisterTxChan(registerTxChan),
		metamorph.WithRegisterTxsChan(registerTxsChan),
		metamorph.WithMinedTxsChan(minedTxsChan),
		metamorph.WithSubmittedTxsChan(submittedTxsChan),
		metamorph.WithStatusUpdatesInterval(mtmCfg.StatusUpdateInterval),
		metamorph.WithStatTimeLimits(mtmCfg.Stats.NotSeenTimeLimit, mtmCfg.Stats.NotFinalTimeLimit),
		metamorph.WithMaxRetries(mtmCfg.MaxRetries),
		metamorph.WithMinimumHealthyConnections(mtmCfg.Health.MinimumHealthyConnections),
		metamorph.WithBlocktxClient(blockTxClient),
		metamorph.WithDoubleSpendCheckInterval(mtmCfg.DoubleSpendCheckInterval),
		metamorph.WithDoubleSpendTxStatusOlderThanInterval(mtmCfg.DoubleSpendTxStatusOlderThanInterval),
	)

	processor, err := metamorph.NewProcessor(
		metamorphStore,
		cacheStore,
		bcMediator,
		statusMessageCh,
		processorOpts...,
	)
	if err != nil {
		stopFn()
		return nil, err
	}
	stoppable = append(stoppable, processor)

	err = processor.Start(commonCfg.Prometheus.IsEnabled())
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start metamorph processor: %v", err)
	}

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: commonCfg.Prometheus.Endpoint,
		MaxMsgSize:         commonCfg.GrpcMessageSize,
		TracingConfig:      commonCfg.Tracing,
		Name:               "metamorph",
	}

	server, err := metamorph.NewServer(logger, metamorphStore, processor, mqClient, serverCfg, optsServer...)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}
	stoppable = append(stoppable, server)

	err = server.ListenAndServe(mtmCfg.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPC server failed: %v", err)
	}
	err = startZMQs(logger, mtmCfg.BlockchainNetwork.Peers, stopFn, statusMessageCh, &shutdownFns)
	if err != nil {
		return nil, err
	}
	return stopFn, nil
}

func enableTracing(commonCfg *config.CommonConfig, logger *slog.Logger) (shutdownFns []func(), optsServer []metamorph.ServerOption, processorOpts []metamorph.Option, bcMediatorOpts []bcnet.Option) {
	if !commonCfg.IsTracingEnabled() {
		return
	}

	shutdownFns = make([]func(), 0)
	optsServer = make([]metamorph.ServerOption, 0)
	processorOpts = make([]metamorph.Option, 0)
	bcMediatorOpts = make([]bcnet.Option, 0)

	cleanup, err := tracing.Enable(logger, "metamorph", commonCfg.Tracing.DialAddr, commonCfg.Tracing.Sample)
	if err != nil {
		logger.Error("failed to enable tracing", slog.String("err", err.Error()))
	} else {
		shutdownFns = append(shutdownFns, cleanup)
	}

	attributes := commonCfg.Tracing.KeyValueAttributes
	hostname, err := os.Hostname()
	if err == nil {
		hostnameAttr := attribute.String("hostname", hostname)
		attributes = append(attributes, hostnameAttr)
	}

	optsServer = append(optsServer, metamorph.WithServerTracer(attributes...))
	processorOpts = append(processorOpts, metamorph.WithTracerProcessor(attributes...))
	bcMediatorOpts = append(bcMediatorOpts, bcnet.WithTracer(attributes...))

	return shutdownFns, optsServer, processorOpts, bcMediatorOpts
}

func startZMQs(logger *slog.Logger, peers []*config.PeerConfig, stopFn func(), statusMessageCh chan *metamorph_p2p.TxStatusMessage, shutdownFns *[]func()) error {
	for i, peerSetting := range peers {
		zmqURL, err := peerSetting.GetZMQUrl()
		if err != nil {
			logger.Warn("failed to get zmq URL for peer", slog.Int("index", i), slog.String("err", err.Error()))
			continue
		}

		if zmqURL == nil {
			continue
		}

		zmqHandler := metamorph.NewZMQHandler(context.Background(), zmqURL, logger)
		zmq, err := metamorph.NewZMQ(zmqURL, statusMessageCh, zmqHandler, logger)
		if err != nil {
			stopFn()
			return fmt.Errorf("failed to create ZMQ: %v", err)
		}
		logger.Info("Listening to ZMQ", slog.String("host", zmqURL.Hostname()), slog.String("port", zmqURL.Port()))

		cleanup, err := zmq.Start()
		*shutdownFns = append(*shutdownFns, cleanup)
		if err != nil {
			stopFn()
			return fmt.Errorf("failed to start ZMQ: %v", err)
		}
	}
	return nil
}

func getMtmMqOpts(logger *slog.Logger) []nats_jetstream.Option {
	submitStreamName := fmt.Sprintf("%s-stream", mq.SubmitTxTopic)
	submitConsName := fmt.Sprintf("%s-cons", mq.SubmitTxTopic)

	var streamWrapper nats_jetstream.Option = func(cl *nats_jetstream.Client) error {
		withStreamFunc := nats_jetstream.WithStream(mq.SubmitTxTopic, submitStreamName, jetstream.WorkQueuePolicy, false)

		// do not return the error so that metamorph runs even if stream creation fails
		err := withStreamFunc(cl)
		if err != nil {
			logger.Warn("failed to create stream", slog.String("stream", submitStreamName), slog.String("err", err.Error()))
		}

		return nil
	}

	var consumerWrapper nats_jetstream.Option = func(cl *nats_jetstream.Client) error {
		withConsumerFunc := nats_jetstream.WithConsumer(mq.SubmitTxTopic, submitStreamName, submitConsName, true, jetstream.AckExplicitPolicy)

		// do not return the error so that metamorph runs even if consumer creation fails
		err := withConsumerFunc(cl)
		if err != nil {
			logger.Warn("failed to create consumer", slog.String("stream", submitStreamName), slog.String("consumer", submitConsName), slog.String("err", err.Error()))
		}

		return nil
	}

	mqOpts := []nats_jetstream.Option{
		streamWrapper,
		consumerWrapper,
	}
	return mqOpts
}

func newMetamorphStore(dbConfig *config.DbConfig, tracingConfig *config.TracingConfig) (s *postgresql.PostgreSQL, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	switch dbConfig.Mode {
	case DbModePostgres:
		postgres := dbConfig.Postgres

		dbInfo := fmt.Sprintf(
			"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
			postgres.User, postgres.Password, postgres.Name, postgres.Host, postgres.Port, postgres.SslMode,
		)

		opts := make([]func(postgreSQL *postgresql.PostgreSQL), 0)
		if tracingConfig != nil && tracingConfig.IsEnabled() {
			opts = append(opts, postgresql.WithTracing(tracingConfig.KeyValueAttributes))
		}

		s, err = postgresql.New(dbInfo, hostname, postgres.MaxIdleConns, postgres.MaxOpenConns, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

// setupMtmBcNetworkCommunication initializes the Metamorph blockchain network communication, configuring it
// to operate in either classic (P2P-only) or hybrid (P2P and multicast) mode.
//
// Parameters:
// - `l *slog.Logger`: Logger instance for event logging.
// - `s store.MetamorphStore`: Storage interface for Metamorph operations.
// - `metamorphCfg  *config.MetamorphConfig`: Configuration object for blockchain network settings.
// - `mediatorOpts []bcnet.Option`: Additional options for the mediator.
//
// Returns:
// - `mediator *bcnet.Mediator`: Coordinates communication between P2P and multicast layers.
// - `messenger *p2p.NetworkMessenger`: Handles P2P message delivery - used by mediator only.
// - `manager *p2p.PeerManager`: Manages the lifecycle of P2P peers.
// - `multicaster *mcast.Multicaster`: Handles multicast message broadcasting and listening - used by mediator only.
// - `messageCh chan *metamorph_p2p.TxStatusMessage`: Channel for handling transaction status messages.
// - `err error`: Error if any part of the setup fails.
//
// Key Details:
// - **Mode Handling**:
//   - `"classic"`: Uses `metamorph_p2p.NewMsgHandler` for exclusive P2P communication.
//   - `"hybrid"`: Uses `metamorph_p2p.NewHybridMsgHandler` for integration of P2P and multicast group communication.
//
// - **Error Cleanup**: Cleans up resources such as the messenger, manager, and multicaster on failure.
// - **Peer Management**: Establishes connections to P2P peers and configures them with write handlers and buffer sizes.
// - **Multicast Communication**: In hybrid mode, joins multicast groups for transaction and rejection messages.
//
// Message Handlers:
// - `metamorph_p2p.NewMsgHandler`: Used in classic mode, handling all communication via P2P.
// - `metamorph_p2p.NewHybridMsgHandler`: Used in hybrid mode, integrating P2P communication with multicast group updates.
func setupMtmBcNetworkCommunication(l *slog.Logger, s store.MetamorphStore, metamorphCfg *config.MetamorphConfig, minConnections int, mediatorOpts []bcnet.Option) (
	mediator *bcnet.Mediator, messenger *p2p.NetworkMessenger, manager *p2p.PeerManager, multicaster *mcast.Multicaster,
	messageCh chan *metamorph_p2p.TxStatusMessage, err error,
) {
	defer func() {
		// cleanup on error
		if err == nil {
			return
		}

		if messenger != nil {
			messenger.Shutdown()
			messenger = nil
		}

		if manager != nil {
			manager.Shutdown()
			manager = nil
		}

		if multicaster != nil {
			multicaster.Shutdown()
			multicaster = nil
		}
	}()

	cfg := metamorphCfg.BlockchainNetwork
	network, err := config.GetNetwork(cfg.Network)
	if err != nil {
		return
	}

	l.Info("Assuming bitcoin network", "network", network)

	messageCh = make(chan *metamorph_p2p.TxStatusMessage, 10000)
	var msgHandler p2p.MessageHandlerI

	switch cfg.Mode {
	case "classic":
		msgHandler = metamorph_p2p.NewMsgHandler(l, s, messageCh)
	case "hybrid":
		l.Info("!!! Metamorph will communicate with blockchain in HYBRID mode (via p2p and multicast groups) !!!")
		msgHandler = metamorph_p2p.NewHybridMsgHandler(l, messageCh)
	default:
		err = fmt.Errorf("unsupported communication type: %s", cfg.Mode)
		return
	}

	// connect to peers
	var managerOpts []p2p.PeerManagerOptions
	if metamorphCfg.MonitorPeers {
		managerOpts = append(managerOpts, p2p.WithRestartUnhealthyPeers())
	}

	manager = p2p.NewPeerManager(l.With(slog.String("module", "peer-mng")), network, managerOpts...)
	connectionsReady := make(chan struct{})
	go connectToPeers(l, manager, connectionsReady, minConnections, network, msgHandler, cfg.Peers,
		p2p.WithNrOfWriteHandlers(8),
		p2p.WithWriteChannelSize(4096))
	if err != nil {
		return
	}

	// wait until min peer connections are ready and then continue startup while remaining peers connect
	<-connectionsReady
	l.Info("current open peer connections", slog.Uint64("count", uint64(manager.CountConnectedPeers())))

	// connect to mcast
	if cfg.Mode == "hybrid" {
		if cfg.Mcast == nil {
			err = errors.New("mcast config is required")
			return
		}

		// TODO: add interfaces
		groups := mcast.GroupsAddresses{
			McastTx:     cfg.Mcast.McastTx.Address,
			McastReject: cfg.Mcast.McastReject.Address,
		}
		multicaster = mcast.NewMulticaster(l, groups, network, messageCh)
		ok := multicaster.Connect()
		if !ok {
			err = fmt.Errorf("error connecting to mcast: %w", err)
			return
		}
	}

	messenger = p2p.NewNetworkMessenger(l, manager)
	mediator = bcnet.NewMediator(l, cfg.Mode == "classic", messenger, multicaster, mediatorOpts...)
	return
}
