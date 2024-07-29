package nats_mq

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func WithLogin(user string, password string) nats.Option {
	return nats.UserInfo(user, password)
}

func NewNatsClient(natsURL string, logger *slog.Logger, natsOpts ...nats.Option) (*nats.Conn, error) {
	var nc *nats.Conn
	var err error

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	opts := []nats.Option{
		nats.Name(hostname),
		nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, err error) {
			logger.Error("connection error", slog.String("err", err.Error()))
		}),
		nats.DiscoveredServersHandler(func(nc *nats.Conn) {
			logger.Info(fmt.Sprintf("Known servers: %v\n", nc.Servers()))
			logger.Info(fmt.Sprintf("Discovered servers: %v\n", nc.DiscoveredServers()))
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Error("client disconnected", slog.String("err", err.Error()))
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("client reconnected")
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("client closed")
		}),
		nats.RetryOnFailedConnect(true),
		nats.PingInterval(2 * time.Minute),
		nats.MaxPingsOutstanding(2),
		nats.ReconnectBufSize(8 * 1024 * 1024),
		nats.MaxReconnects(60),
		nats.ReconnectWait(2 * time.Second),
	}

	opts = append(opts, natsOpts...)

	nc, err = nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS server: %v", err)
	}

	return nc, nil
}
