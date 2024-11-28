package metamorph_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	zmq "github.com/pebbe/zmq4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"log/slog"
)

func StartPubServer(address string, messages chan string) (func(), error) {
	pubSocket, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("failed to create PUB socket: %w", err)
	}

	err = pubSocket.Bind(address)
	if err != nil {
		return nil, fmt.Errorf("failed to bind PUB socket: %w", err)
	}

	stop := make(chan struct{})
	go func() {
		defer pubSocket.Close()
		for {
			select {
			case <-stop:
				return
			case msg := <-messages:
				fmt.Printf("Sending message: %v\n", msg)
				_, err := pubSocket.Send(msg, 0)
				if err != nil {
					fmt.Printf("Failed to send message: %v\n", err)
				}
			}
		}
	}()

	return func() { close(stop) }, nil
}

func TestNewZMQHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zmqURL, err := url.Parse("tcp://127.0.0.1:5555")
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	// Given
	handler := metamorph.NewZMQHandler(ctx, zmqURL, logger)

	// Then
	assert.NotNil(t, handler)
}

func TestZMQHandler_Subscribe_Unsubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zmqURL, err := url.Parse("tcp://127.0.0.1:5555")
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	// Given
	handler := metamorph.NewZMQHandler(ctx, zmqURL, logger)
	topic := "hashblock"

	messages := make(chan string, 10)
	stopPubServer, err := StartPubServer(zmqURL.String(), messages)
	require.NoError(t, err)
	defer stopPubServer()

	time.Sleep(500 * time.Millisecond)

	// When
	subscriber, err := zmq.NewSocket(zmq.SUB)
	require.NoError(t, err)
	defer subscriber.Close()

	err = subscriber.Connect(zmqURL.String())
	require.NoError(t, err)
	err = subscriber.SetSubscribe(topic)
	require.NoError(t, err)
	fmt.Println("Subscribed to topic:", topic)

	go func() {
		time.Sleep(200 * time.Millisecond)
		messages <- fmt.Sprintf("%s Hello, World!", topic)
	}()

	// Then
	poller := zmq.NewPoller()
	poller.Add(subscriber, zmq.POLLIN)
	polled, err := poller.Poll(2000 * time.Millisecond)
	require.NoError(t, err)

	if len(polled) > 0 {
		msg, err := subscriber.RecvMessage(0)
		require.NoError(t, err)
		fmt.Println("Received message:", msg)
		assert.Contains(t, msg[0], "Hello, World!")
	} else {
		t.Error("Message not received in time")
	}

	// When
	err = handler.Unsubscribe(topic, nil)
	require.NoError(t, err)
	fmt.Println("Unsubscribed from topic:", topic)

	// Then
	time.Sleep(200 * time.Millisecond)
	polled, err = poller.Poll(2000 * time.Millisecond)
	require.NoError(t, err)

	if len(polled) > 0 {
		t.Error("Should not receive message after unsubscribe")
	} else {
		fmt.Println("No message received after unsubscribe")
	}
}

func TestZMQHandler_start(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	zmqURL, err := url.Parse("tcp://127.0.0.1:5555")
	require.NoError(t, err)

	messages := make(chan string, 10)
	address := fmt.Sprintf("tcp://%s", zmqURL.Host)
	stopPubServer, err := StartPubServer(address, messages)
	require.NoError(t, err)
	defer stopPubServer()

	// Given
	time.Sleep(500 * time.Millisecond)

	// When
	subscriber, err := zmq.NewSocket(zmq.SUB)
	require.NoError(t, err)
	defer subscriber.Close()

	err = subscriber.Connect(zmqURL.String())
	require.NoError(t, err)
	topic := "hashblock"
	err = subscriber.SetSubscribe(topic)
	require.NoError(t, err)
	fmt.Println("Subscribed to topic:", topic)

	go func() {
		time.Sleep(200 * time.Millisecond)
		messages <- fmt.Sprintf("%s Hello, World!", topic)
	}()

	// Then
	poller := zmq.NewPoller()
	poller.Add(subscriber, zmq.POLLIN)
	polled, err := poller.Poll(2000 * time.Millisecond)
	require.NoError(t, err)

	if len(polled) > 0 {
		msg, err := subscriber.RecvMessage(0)
		require.NoError(t, err)
		fmt.Println("Received message:", msg)
		assert.Contains(t, msg[0], "Hello, World!")
	} else {
		t.Error("Message not received in time")
	}
}
