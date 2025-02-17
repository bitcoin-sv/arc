package metamorph_test

import (
	"context"
	"fmt"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"net/url"
	"os"
	"testing"
	"time"

	zmq "github.com/pebbe/zmq4"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

const ZMQ_FIRST_TOPIC = "hashblock"
const ZMQ_SECOND_TOPIC = "secondtopic"

var zmqURL1, errZMQURL1 = url.Parse("tcp://127.0.0.1:5555")
var zmqURL2, errZMQURL2 = url.Parse("tcp://127.0.0.1:5556")

func CommonSetupAndValidations(t *testing.T) {
	require.NoError(t, errZMQURL1)
	require.NoError(t, errZMQURL2)

}

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
		defer func() {
			err = pubSocket.Close()
		}()

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

func ZMQLibraryCreatePublisherAndPublishTopic(t *testing.T, topic string, url string) func() {

	messages := make(chan string, 10)
	//address := fmt.Sprintf("tcp://%s", url)
	stopPubServer, err := StartPubServer(url, messages)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	go func() {
		time.Sleep(100 * time.Millisecond)

		for i := range 3 {
			messages <- fmt.Sprintf("%s Hello, World! #%d", topic, i)
		}
	}()

	return stopPubServer
}

func ZMQLibrarySusbscribeToTopic(t *testing.T, topic string, url string) (*zmq.Socket, *zmq.Poller) {

	// When
	subscriber, err := zmq.NewSocket(zmq.SUB)
	require.NoError(t, err)

	err = subscriber.Connect(url)
	require.NoError(t, err)

	err = subscriber.SetSubscribe(topic)
	require.NoError(t, err)
	fmt.Println("Subscribed to topic:", topic)

	// Then
	time.Sleep(500 * time.Millisecond)
	poller := zmq.NewPoller()
	poller.Add(subscriber, zmq.POLLIN)

	return subscriber, poller
}

func ZMQLibraryReceiveFromSocket(t *testing.T, subscriber *zmq.Socket, poller *zmq.Poller, num int, expected bool) {
	for i := range num {
		time.Sleep(100 * time.Millisecond)
		polled, err := poller.Poll(2000 * time.Millisecond)
		require.NoError(t, err)
		if len(polled) > 0 {
			msg, err := subscriber.RecvMessage(0)
			require.NoError(t, err)
			fmt.Printf("\nReceived message %d = %s", i, msg)
			assert.Contains(t, msg[0], "Hello, World!")
		} else {
			if !expected {
				fmt.Println("No more messages in the queue")
			} else {
				t.Error("Message not received in time")
			}

		}

	}

}
func TestZMQHandler_prereqs(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	/* Test Case /*
	Given I want to test metamorph handler
	When I am about to test it
	Then I want to make sure that the zmq library is working as a prerequisite
	*/
	defer cancel()
	CommonSetupAndValidations(t)
	stopPubServer1 := ZMQLibraryCreatePublisherAndPublishTopic(t, ZMQ_FIRST_TOPIC, zmqURL1.String())
	subscriber1, poller1 := ZMQLibrarySusbscribeToTopic(t, ZMQ_FIRST_TOPIC, zmqURL1.String())
	defer func() {
		stopPubServer1()
		_ = subscriber1.Close()
	}()

	//Expect the three first messages to be successfully retrieved
	ZMQLibraryReceiveFromSocket(t, subscriber1, poller1, 3, true)
	//Expect the fourth one to be empty
	ZMQLibraryReceiveFromSocket(t, subscriber1, poller1, 1, false)

	stopPubServer2 := ZMQLibraryCreatePublisherAndPublishTopic(t, ZMQ_SECOND_TOPIC, zmqURL2.String())
	subscriber2, poller2 := ZMQLibrarySusbscribeToTopic(t, ZMQ_SECOND_TOPIC, zmqURL2.String())

	defer func() {
		stopPubServer2()
		_ = subscriber2.Close()
	}()
	ZMQLibraryReceiveFromSocket(t, subscriber2, poller2, 1, true)
	assert.NotNil(t, poller2)
}

func TestNewZMQHandlers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	/* Test Case /*
	Given I want to test metamorph handler
	When I am testing new handlers
	Then I want to test that it can support several of them
	*/

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	messages := make(chan []string, 10)

	handler1 := metamorph.NewZMQHandler(ctx, zmqURL1, logger)
	assert.NotNil(t, handler1)

	handler2 := metamorph.NewZMQHandler(ctx, zmqURL2, logger)
	assert.NotNil(t, handler2)

	handler3 := metamorph.NewZMQHandler(ctx, zmqURL2, logger)
	assert.NotNil(t, handler3)
	_ = handler1.Subscribe(ZMQ_FIRST_TOPIC, messages)
	_ = handler2.Subscribe(ZMQ_FIRST_TOPIC, messages)
	_ = handler3.Subscribe(ZMQ_FIRST_TOPIC, messages)
	//subscribing for a second time
	_ = handler1.Subscribe(ZMQ_FIRST_TOPIC, messages)
	_ = handler2.Subscribe(ZMQ_FIRST_TOPIC, messages)
	_ = handler3.Subscribe(ZMQ_FIRST_TOPIC, messages)

	//subscribing to a second topic
	_ = handler1.Subscribe(ZMQ_SECOND_TOPIC, messages)
	_ = handler2.Subscribe(ZMQ_SECOND_TOPIC, messages)
	_ = handler3.Subscribe(ZMQ_SECOND_TOPIC, messages)

	/*TODO How to check errors in handlers e.g. duplicated topics /*

	}

	func TestZMQHandler_Subscribe_Unsubscribe(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		/* Test Case /*
		Given I want to test metamorph handler
		When I am testing subscribing and unsubscribing to topics
		Then I want to make sure all the combinations work
	*/

	// Given
	handler := metamorph.NewZMQHandler(ctx, zmqURL1, logger)

	messages2 := make(chan string, 10)
	stopPubServer, err := StartPubServer(zmqURL1.String(), messages2)
	require.NoError(t, err)
	defer stopPubServer()

	time.Sleep(500 * time.Millisecond)

	//Adding a subscriber
	_ = handler.Subscribe(ZMQ_FIRST_TOPIC, nil)
	go func() {
		time.Sleep(200 * time.Millisecond)
		messages2 <- fmt.Sprintf("%s Hello, World!", ZMQ_FIRST_TOPIC)
	}()

	// When
	err = handler.Unsubscribe(ZMQ_FIRST_TOPIC, nil)
	require.NoError(t, err)
	fmt.Println("Unsubscribed from topic:", ZMQ_FIRST_TOPIC)

}
