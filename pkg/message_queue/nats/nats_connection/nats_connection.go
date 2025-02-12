package nats_connection

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

var ErrNatsConnectionFailed = fmt.Errorf("failed to connect to NATS server")

func WithMaxReconnects(maxReconnects int) func(config *natsConfig) {
	return func(config *natsConfig) {
		config.maxReconnects = maxReconnects
	}
}

func WithClientClosedChannel(clientClosedCh chan struct{}) func(config *natsConfig) {
	return func(config *natsConfig) {
		config.clientClosedCh = clientClosedCh
	}
}

func WithReconnectWait(reconnectWait time.Duration) func(config *natsConfig) {
	return func(config *natsConfig) {
		config.reconnectWait = reconnectWait
	}
}

type natsConfig struct {
	maxReconnects        int
	pingInterval         time.Duration
	reconnectBufSize     int
	reconnectWait        time.Duration
	maxPingsOutstanding  int
	retryOnFailedConnect bool
	clientClosedCh       chan struct{}
}

func New(natsURL string, logger *slog.Logger, opts ...func(config *natsConfig)) (*nats.Conn, error) {
	var nc *nats.Conn
	var err error

	logger.With(slog.String("module", "nats"))

	cfg := &natsConfig{
		maxReconnects:        10,
		pingInterval:         15 * time.Second,
		reconnectBufSize:     8 * 1024 * 1024,
		reconnectWait:        2 * time.Second,
		maxPingsOutstanding:  2,
		retryOnFailedConnect: true,
		clientClosedCh:       nil,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	natsOpts := []nats.Option{
		nats.Name(hostname),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			if err != nil {
				logger.Error("connection error", slog.String("err", err.Error()))
			}
		}),
		nats.DiscoveredServersHandler(func(nc *nats.Conn) {
			logger.Info(fmt.Sprintf("Known servers: %v", nc.Servers()))
			logger.Info(fmt.Sprintf("Discovered servers: %v", nc.DiscoveredServers()))
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			var args []any
			if err != nil {
				args = append(args, slog.String("err", err.Error()))
			}
			buffered, bufferedErr := nc.Buffered()
			if bufferedErr == nil {
				args = append(args, slog.Int("buffered", buffered))
			}

			logger.Error("client disconnected", args...)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("client reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Warn("client closed")
			if cfg.clientClosedCh != nil {
				select {
				case cfg.clientClosedCh <- struct{}{}:
				default:
				}
			}
		}),
		nats.RetryOnFailedConnect(cfg.retryOnFailedConnect),
		nats.PingInterval(cfg.pingInterval),
		nats.MaxPingsOutstanding(cfg.maxPingsOutstanding),
		nats.ReconnectBufSize(cfg.reconnectBufSize),
		nats.MaxReconnects(cfg.maxReconnects),
		nats.ReconnectWait(cfg.reconnectWait),
	}

	nc, err = nats.Connect(natsURL, natsOpts...)
	if err != nil {
		return nil, errors.Join(ErrNatsConnectionFailed, err)
	}

	return nc, nil
}
