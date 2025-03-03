package cmd

/* Callbacker Service */
/*

This service manages the sending and storage of callbacks, with a persistent storage backend using PostgreSQL.
It starts by checking the storage for any unsent callbacks and passing them to the callback dispatcher.

Key components:
- PostgreSQL DB: used for persistent storage of callbacks
- callback dispatcher: responsible for dispatching callbacks to sender
- callback sender: responsible for sending callbacks
- background tasks:
  - periodically cleans up old, unsent callbacks from storage
  - periodically checks the storage for callbacks in delayed state (temporary ban) and re-attempts dispatch after the delay period
- gRPC server with endpoints:
  - Health: provides a health check endpoint for the service
  - SendCallback: receives and processes new callback requests

Startup routine: on service start, checks the storage for pending callbacks and dispatches them if needed
Graceful Shutdown: on service termination, all components are stopped gracefully, ensuring that any unprocessed callbacks are persisted in the database for future processing.

*/

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
)

func StartCallbacker(logger *slog.Logger, arcConfig *config.ArcConfig, shutdownCh chan string) (func(), error) {
	logger = logger.With(slog.String("service", "callbacker"))
	logger.Info("Starting")

	cfg := arcConfig.Callbacker

	var (
		callbackerStore *postgresql.PostgreSQL
		sender          *callbacker.CallbackSender
		dispatcher      *callbacker.CallbackDispatcher
		server          *callbacker.Server
		healthServer    *grpc_utils.GrpcServer
		mqClient        callbacker.MessageQueueClient
		processor       *callbacker.Processor
		err             error
	)

	stopFn := func() {
		logger.Info("Shutting down callbacker")
		disposeCallbacker(logger, server, dispatcher, sender, callbackerStore, healthServer, processor, mqClient)
		logger.Info("Shutdown callbacker complete")
	}

	callbackerStore, err = newStore(cfg.Db)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker store: %v", err)
	}

	sender, err = callbacker.NewSender(&http.Client{Timeout: 5 * time.Second}, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create callback sender: %v", err)
	}

	runNewManager := func(url string) callbacker.SendManagerI {
		manager := send_manager.New(url, sender, callbackerStore, logger,
			send_manager.WithQueueProcessInterval(cfg.Pause),
			send_manager.WithBatchSendInterval(cfg.BatchSendInterval),
			send_manager.WithExpiration(cfg.Expiration),
		)
		manager.Start()

		return manager
	}

	dispatcher = callbacker.NewCallbackDispatcher(sender, runNewManager)

	clientClosedCh := make(chan struct{}, 1)

	natsConnection, err := nats_connection.New(arcConfig.MessageQueue.URL, logger, nats_connection.WithClientClosedChannel(clientClosedCh))
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to establish connection to message queue at URL %s: %v", arcConfig.MessageQueue.URL, err)
	}

	go func() {
		<-clientClosedCh
		logger.Warn("message queue client closed")
		shutdownCh <- "message queue client closed"
	}()

	hostname, err := os.Hostname()
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}

	opts := []nats_jetstream.Option{
		nats_jetstream.WithSubscribedInterestPolicy(hostname, []string{callbacker.CallbackTopic}, true),
	}
	mqClient, err = nats_jetstream.New(natsConnection, logger, opts...)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create nats client: %v", err)
	}

	processor, err = callbacker.NewProcessor(dispatcher, callbackerStore, mqClient, hostname, logger)
	if err != nil {
		stopFn()
		return nil, err
	}

	processor.StartCallbackStoreCleanup(cfg.PruneInterval, cfg.PruneOlderThan)
	processor.DispatchPersistedCallbacks()

	err = processor.Start()
	if err != nil {
		stopFn()
		return nil, err
	}

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: arcConfig.Prometheus.Endpoint,
		MaxMsgSize:         arcConfig.GrpcMessageSize,
		TracingConfig:      arcConfig.Tracing,
		Name:               "blocktx",
	}

	server, err = callbacker.NewServer(logger, dispatcher, callbackerStore, mqClient, serverCfg)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}

	err = server.ListenAndServe(cfg.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPC server failed: %v", err)
	}

	healthServer, err = grpc_utils.ServeNewHealthServer(logger, server, cfg.Health.SeverDialAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	logger.Info("Ready to work")
	return stopFn, nil
}

func newStore(dbConfig *config.DbConfig) (s *postgresql.PostgreSQL, err error) {
	switch dbConfig.Mode {
	case DbModePostgres:
		cfg := dbConfig.Postgres

		dbInfo := fmt.Sprintf(
			"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
			cfg.User, cfg.Password, cfg.Name, cfg.Host, cfg.Port, cfg.SslMode,
		)
		s, err = postgresql.New(dbInfo, cfg.MaxIdleConns, cfg.MaxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

func disposeCallbacker(l *slog.Logger, server *callbacker.Server,
	dispatcher *callbacker.CallbackDispatcher, sender *callbacker.CallbackSender,
	store *postgresql.PostgreSQL, healthServer *grpc_utils.GrpcServer, processor *callbacker.Processor, mqClient callbacker.MessageQueueClient) {
	// dispose the dependencies in the correct order:
	// 1. server - ensure no new callbacks will be received
	// 2. dispatcher - ensure all already accepted callbacks are processed
	// 3. processor - remove all URL mappings
	// 4. sender - finally, stop the sender as there are no callbacks left to send
	// 5. store

	if server != nil {
		server.GracefulStop()
	}
	if dispatcher != nil {
		dispatcher.GracefulStop()
	}
	if processor != nil {
		processor.GracefulStop()
	}
	if sender != nil {
		sender.GracefulStop()
	}
	if mqClient != nil {
		mqClient.Shutdown()
	}
	if store != nil {
		err := store.Close()
		if err != nil {
			l.Error("Could not close the store", slog.String("err", err.Error()))
		}
	}

	if healthServer != nil {
		healthServer.GracefulStop()
	}
}
