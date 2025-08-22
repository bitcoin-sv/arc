package metamorph_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/metamorph"
)

func testNewZMQHandler(t *testing.T, zmqURL *url.URL, waitingTime time.Duration) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	var handler *metamorph.ZMQHandler
	var zmq *metamorph.ZMQ

	srv, cli := zMQ4StartServer(t)
	defer srv.Close()
	defer cli.Close()

	handler = metamorph.NewZMQHandlerWithRefreshRate(context.Background(), zmqURL, logger, 1*time.Second)
	require.NotNil(t, handler)
	zmq, err := metamorph.NewZMQ(zmqEndpointURL, statusMessageCh, handler, logger)
	require.NoError(t, err)
	closeZMQ, err := zmq.Start()
	require.NoError(t, err)
	defer closeZMQ()
	logger.Info("Listening to ZMQ", slog.String("host", zmqEndpointURL.Hostname()), slog.String("port", zmqEndpointURL.Port()))

	tt := []struct {
		name        string
		expectedErr bool
	}{
		{name: "notvalid", expectedErr: true},
		{name: "hashblock"},
		{name: "discardedfrommempool"},
		{name: "hashtx2"},
		{name: "invalidtx"},
	}
	// Test Case
	// Given I want to test metamorph handler
	// When I have a ZMQ publisher up
	// Then I want to make sure the handler can
	// subscribe and unsubscribe to all topics

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			go func() {
				err = handler.Subscribe(tc.name, zmqMessages)
				if tc.expectedErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				err = handler.Unsubscribe(tc.name, zmqMessages)
				if tc.expectedErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}()
			time.Sleep(waitingTime)
		})
	}

	// Test Case
	// Given I want to test metamorph handler
	// When the publisher is down
	// Then I want to make sure the handler safely fails
	srv.Close()
	cli.Close()
	err = handler.Subscribe("hashblock", zmqMessages)
	require.NoError(t, err)
	time.Sleep(waitingTime)
	err = handler.Unsubscribe("hashtx2", zmqMessages)
	require.NoError(t, err)
	time.Sleep(waitingTime)
}

func TestNewZMQHandler(t *testing.T) {
	testNewZMQHandler(t, zmqEndpointURL, 1*time.Second)
}

func TestNewZMQHandlerWrongURL(t *testing.T) {
	testNewZMQHandler(t, zmqNotExistingURL, 1*time.Millisecond)
}

func zMQ4StartServer(t *testing.T) (zmq4.Socket, zmq4.Socket) {
	ctx := context.Background()
	ep, err := endPoint(t, "tcp")
	require.NoError(t, err)

	_, timeout := context.WithTimeout(ctx, 20*time.Second)
	defer timeout()

	logger := log.Default()

	srv := zmq4.NewXPub(ctx, zmq4.WithLogger(logger))
	cli := zmq4.NewXSub(ctx, zmq4.WithLogger(logger))
	err = srv.Listen(ep)
	require.NoError(t, err)

	err = cli.Dial(ep)
	require.NoError(t, err)
	t.Logf("dialed %q", ep)

	pub := zmq4.NewPub(ctx)
	msg := zmq4.NewMsgString("hashblock")
	_ = pub.Send(msg)
	return srv, cli
}

func endPoint(t *testing.T, transport string) (string, error) {
	switch transport {
	case "tcp":
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:5555")
		if err != nil {
			return "", err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return "", err
		}
		defer l.Close()
		return fmt.Sprintf("tcp://%s", l.Addr()), nil
	default:
		t.Fatalf("invalid transport: %s", transport)
		return "", nil
	}
}
