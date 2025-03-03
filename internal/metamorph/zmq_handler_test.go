package metamorph_test

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/go-zeromq/zmq4"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewZMQHandler(t *testing.T) {
	// Test Case
	// Given I want to test metamorph handler
	// When I have a ZMQ publisher up
	// Then I want to make sure the handler can
	// subscribe and unsubscribe to all topics
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	var handler *metamorph.ZMQHandler
	var zmq *metamorph.ZMQ
	/*TODO without starting a ZMQ the handler will fail to connect with:
	error="zmq4: could not dial to \"tcp://127.0.0.1:5555\" (retry=250ms): dial tcp 127.0.0.1:5555: connect: connection refused
	Adding a publisher and a subscriber like below allows the connection to be stablished but will fail with:
	zmq4: could not open a ZMTP connection with "tcp://127.0.0.1:5555": zmq4: could not initialize ZMTP connection: zmq4: peer="SUB" not compatible with "SUB"
	*/

	pub, sub := ZMQ4StartServer(t)
	defer pub.Close()
	defer sub.Close()

	handler = metamorph.NewZMQHandler(context.Background(), zmqEndpointURL, logger)
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
			time.Sleep(1 * time.Second)
		})
	}
}

func ZMQ4StartServer(t *testing.T) (zmq4.Socket, zmq4.Socket) {
	// Create a new subscriber socket.
	sub := zmq4.NewSub(context.Background())

	// Create a new publisher socket.
	pub := zmq4.NewPub(context.Background())

	//Set sub to subscribe mode
	if err := sub.SetOption(zmq4.OptionSubscribe, ZmqValidTopic); err != nil {
		t.Fatalf("Subscription failed: %v", err)
	}

	// Bind the publisher to a port.
	if err := sub.Listen(zmqEndpoint); err != nil {
		t.Fatalf("Sub Dial failed: %v", err)
	}
	if err := pub.Dial(zmqEndpoint); err != nil {
		t.Fatalf("Pub Dial failed: %v", err)
	}
	return pub, sub
}
