package services

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
	"time"

	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
)

func StartCallbacker(logger *slog.Logger, cbCfg *config.CallbackerConfig, commonCfg *config.CommonConfig) (func(), error) {
	logger = logger.With(slog.String("service", "callbacker"))
	logger.Info("Starting")
	var (
		callbackerStore *postgresql.PostgreSQL
		sender          *callbacker.CallbackSender
		server          *callbacker.Server
		healthServer    *grpc_utils.GrpcServer
		mqClient        mq.MessageQueueClient
		processor       *callbacker.Processor
		err             error
	)

	stopFn := func() {
		logger.Info("Shutting down callbacker")
		disposeCallbacker(logger, server, sender, callbackerStore, healthServer, processor, mqClient)
		logger.Info("Shutdown callbacker complete")
	}

	callbackerStore, err = newStore(cbCfg.Db)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker store: %v", err)
	}

	sender, err = callbacker.NewSender(logger, callbacker.WithTimeout(5*time.Second))
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create callback sender: %v", err)
	}

	mqOpts := getCbkMqOpts()
	mqClient, err = mq.NewMqClient(logger, commonCfg.MessageQueue, mqOpts...)
	if err != nil {
		return nil, err
	}

	sendRequestChan := make(chan *callbacker_api.SendRequest, 500)

	mqProvider := callbacker.NewMessageQueueProvider(mqClient, logger)
	err = mqProvider.Start(sendRequestChan)
	if err != nil {
		stopFn()
		return nil, err
	}

	processor, err = callbacker.NewProcessor(
		sender,
		callbackerStore,
		sendRequestChan,
		logger,
		callbacker.WithExpiration(cbCfg.Expiration),
		callbacker.WithSingleSendInterval(cbCfg.Pause),
		callbacker.WithBatchSendInterval(cbCfg.BatchSendInterval),
		callbacker.WithClearInterval(cbCfg.PruneInterval),
		callbacker.WithClearRetentionPeriod(cbCfg.PruneOlderThan),
		callbacker.WithMaxRetries(cbCfg.MaxRetries),
	)
	if err != nil {
		stopFn()
		return nil, err
	}

	err = processor.Start()
	if err != nil {
		stopFn()
		return nil, err
	}

	serverCfg := grpc_utils.ServerConfig{
		PrometheusEndpoint: commonCfg.Prometheus.Endpoint,
		MaxMsgSize:         commonCfg.GrpcMessageSize,
		TracingConfig:      commonCfg.Tracing,
		Name:               "blocktx",
	}

	server, err = callbacker.NewServer(logger, callbackerStore, mqClient, serverCfg)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}

	err = server.ListenAndServe(cbCfg.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPC server failed: %v", err)
	}

	logger.Info("Ready to work")
	return stopFn, nil
}

func getCbkMqOpts() []nats_jetstream.Option {
	callbackStreamName := fmt.Sprintf("%s-stream", mq.CallbackTopic)
	callbackConsName := fmt.Sprintf("%s-cons", mq.CallbackTopic)

	mqOpts := []nats_jetstream.Option{
		nats_jetstream.WithStream(mq.CallbackTopic, callbackStreamName, jetstream.WorkQueuePolicy, false),
		nats_jetstream.WithConsumer(mq.CallbackTopic, callbackStreamName, callbackConsName, true, jetstream.AckExplicitPolicy),
	}
	return mqOpts
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
	sender *callbacker.CallbackSender,
	store *postgresql.PostgreSQL, healthServer *grpc_utils.GrpcServer, processor *callbacker.Processor, mqClient mq.MessageQueueClient) {
	// dispose the dependencies in the correct order:
	// 1. server - ensure no new callbacks will be received
	// 2. processor - remove all URL mappings
	// 3. sender - stop the sender as there are no callbacks left to send
	// 4. mqClient - finally, stop the mq client as there are no callbacks left to send
	// 5. store

	if server != nil {
		server.GracefulStop()
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
