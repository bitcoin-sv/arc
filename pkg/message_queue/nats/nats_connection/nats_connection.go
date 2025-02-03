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

func New(natsURL string, logger *slog.Logger) (*nats.Conn, error) {
	var nc *nats.Conn
	var err error

	logger.With(slog.String("module", "nats"))

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	opts := []nats.Option{
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
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logger.Error("client disconnected", slog.String("err", err.Error()))
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("client reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("client closed")
		}),
		nats.RetryOnFailedConnect(true),
		nats.PingInterval(30 * time.Second),
		nats.MaxPingsOutstanding(2),
		nats.ReconnectBufSize(8 * 1024 * 1024),
		nats.MaxReconnects(60),
		nats.ReconnectWait(2 * time.Second),
	}

	nc, err = nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, errors.Join(ErrNatsConnectionFailed, err)
	}

	return nc, nil
}
