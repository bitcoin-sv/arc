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
  - periodically checks the storage for callbacks in quarantine (temporary ban) and re-attempts dispatch after the quarantine period
- gRPC server with endpoints:
  - Health: provides a health check endpoint for the service
  - SendCallback: receives and processes new callback requests

Startup routine: on service start, checks the storage for pending callbacks and dispatches them if needed
Graceful Shutdown: on service termination, all components are stopped gracefully, ensuring that any unprocessed callbacks are persisted in the database for future processing.

*/

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/bitcoin-sv/arc/internal/callbacker/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/grpc_opts"
)

func StartCallbacker(logger *slog.Logger, appConfig *config.ArcConfig) (func(), error) {
	logger = logger.With(slog.String("service", "callbacker"))
	logger.Info("Starting")

	config := appConfig.Callbacker

	var (
		store        store.CallbackerStore
		sender       *callbacker.CallbackSender
		dispatcher   *callbacker.CallbackDispatcher
		workers      *callbacker.BackgroundWorkers
		server       *callbacker.Server
		healthServer *grpc_opts.GrpcServer

		err error
	)

	stopFn := func() {
		logger.Info("Shutting down callbacker")
		dispose(logger, server, workers, dispatcher, sender, store, healthServer)
		logger.Info("Shutdown complete")
	}

	store, err = newStore(config.Db)
	if err != nil {
		return nil, fmt.Errorf("failed to create callbacker store: %v", err)
	}

	sender, err = callbacker.NewSender(&http.Client{Timeout: 5 * time.Second}, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to create callback sender: %v", err)
	}

	dispatcher = callbacker.NewCallbackDispatcher(sender, store, logger, config.Pause, config.BatchSendInterval, config.QuarantinePolicy.BaseDuration, config.QuarantinePolicy.PermQuarantineAfter)
	err = dispatchPersistedCallbacks(store, dispatcher, logger)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to dispatch previously persisted callbacks: %v", err)
	}

	workers = callbacker.NewBackgroundWorkers(store, dispatcher, logger)
	workers.StartCallbackStoreCleanup(config.PruneInterval, config.PruneOlderThan)
	workers.StartQuarantineCallbacksDispatch(config.QuarantineCheckInterval)

	server, err = callbacker.NewServer(appConfig.PrometheusEndpoint, appConfig.GrpcMessageSize, logger, dispatcher, false)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("create GRPCServer failed: %v", err)
	}

	err = server.ListenAndServe(config.ListenAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("serve GRPC server failed: %v", err)
	}

	healthServer, err = grpc_opts.ServeNewHealthServer(logger, server, config.Health.SeverDialAddr)
	if err != nil {
		stopFn()
		return nil, fmt.Errorf("failed to start health server: %v", err)
	}

	logger.Info("Ready to work")
	return stopFn, nil
}

func newStore(dbConfig *config.DbConfig) (s store.CallbackerStore, err error) {
	switch dbConfig.Mode {
	case DbModePostgres:
		config := dbConfig.Postgres

		dbInfo := fmt.Sprintf(
			"user=%s password=%s dbname=%s host=%s port=%d sslmode=%s",
			config.User, config.Password, config.Name, config.Host, config.Port, config.SslMode,
		)
		s, err = postgresql.New(dbInfo, config.MaxIdleConns, config.MaxOpenConns)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres DB: %v", err)
		}
	default:
		return nil, fmt.Errorf("db mode %s is invalid", dbConfig.Mode)
	}

	return s, err
}

func dispatchPersistedCallbacks(s store.CallbackerStore, d *callbacker.CallbackDispatcher, l *slog.Logger) error {
	l.Info("Dispatch persited callbacks")

	const batchSize = 100
	ctx := context.Background()

	for {
		callbacks, err := s.PopMany(ctx, batchSize)
		if err != nil || len(callbacks) == 0 {
			return err
		}

		for _, c := range callbacks {
			d.Dispatch(c.Url, toCallbackEntry(c), c.AllowBatch)
		}
	}
}

func dispose(l *slog.Logger, server *callbacker.Server, workers *callbacker.BackgroundWorkers,
	dispatcher *callbacker.CallbackDispatcher, sender *callbacker.CallbackSender,
	store store.CallbackerStore, healthServer *grpc_opts.GrpcServer) {

	// dispose the dependencies in the correct order:
	// 1. server - ensure no new callbacks will be received
	// 2. background workers - ensure no callbacks from background will be accepted
	// 3. dispatcher - ensure all already accepted callbacks are proccessed
	// 4. sender - finally, stop the sender as there are no callbacks left to send
	// 5. store

	if server != nil {
		server.GracefulStop()
	}
	if workers != nil {
		workers.GracefulStop()
	}
	if dispatcher != nil {
		dispatcher.GracefulStop()
	}
	if sender != nil {
		sender.GracefulStop()
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

func toCallbackEntry(dto *store.CallbackData) *callbacker.CallbackEntry {
	d := &callbacker.Callback{
		Timestamp: dto.Timestamp,

		CompetingTxs: dto.CompetingTxs,
		TxID:         dto.TxID,
		TxStatus:     dto.TxStatus,
		ExtraInfo:    dto.ExtraInfo,
		MerklePath:   dto.MerklePath,

		BlockHash:   dto.BlockHash,
		BlockHeight: dto.BlockHeight,
	}

	return &callbacker.CallbackEntry{
		Token: dto.Token,
		Data:  d,
	}
}
