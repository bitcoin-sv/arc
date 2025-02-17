package metamorph_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/go-zeromq/zmq4"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewZMQHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	var handler *metamorph.ZMQHandler
	var zmq *metamorph.ZMQ

	srv, cli := ZMQ4StartServer(t)
	defer srv.Close()
	defer cli.Close()

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
			time.Sleep(1 * time.Second)
		})
	}

	// Test Case
	// Given I want to test metamorph handler
	// When the publisher is down
	// Then I want to make sure the handler safely fails
	go func() {
		srv.Close()
		cli.Close()
		err = handler.Subscribe("hashblock", zmqMessages)
		require.NoError(t, err)
	}()
	time.Sleep(1 * time.Second)
	go func() {
		err = handler.Unsubscribe("hashtx2", zmqMessages)
		require.NoError(t, err)
	}()
}

func ZMQ4StartServer(t *testing.T) (zmq4.Socket, zmq4.Socket) {
	bkg := context.Background()
	ep := must(EndPoint("tcp"))
	cleanUp(ep)

	_, timeout := context.WithTimeout(bkg, 20*time.Second)
	defer timeout()

	srv := zmq4.NewXPub(bkg, zmq4.WithLogger(Devnull))
	cli := zmq4.NewXSub(bkg, zmq4.WithLogger(Devnull))

	err := srv.Listen(ep)
	if err != nil {
		t.Fatalf("could not listen on %q: %+v", ep, err)
	}

	err = cli.Dial(ep)
	if err != nil {
		t.Fatalf("could not dial %q: %+v", ep, err)
	} else {
		t.Logf("dialed %q", ep)
	}

	pub := zmq4.NewPub(bkg)
	msg := zmq4.NewMsgString("hashblock")
	_ = pub.Send(msg)
	return srv, cli
}

var (
	Devnull = log.New(io.Discard, "zmq4: ", 0)
)

func must(str string, err error) string {
	if err != nil {
		panic(err)
	}
	return str
}

func EndPoint(transport string) (string, error) {
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
	case "ipc":
		return "ipc://tmp-" + newUUID(), nil
	case "inproc":
		return "inproc://tmp-" + newUUID(), nil
	default:
		panic("invalid transport: [" + transport + "]")
	}
}

func newUUID() string {
	var uuid [16]byte
	if _, err := io.ReadFull(rand.Reader, uuid[:]); err != nil {
		_ = fmt.Errorf("cannot generate random data for UUID: %v", err)
	}
	uuid[8] = uuid[8]&^0xc0 | 0x80
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func cleanUp(ep string) {
	switch {
	case strings.HasPrefix(ep, "ipc://"):
		os.Remove(ep[len("ipc://"):])
	case strings.HasPrefix(ep, "inproc://"):
		os.Remove(ep[len("inproc://"):])
	}
}
