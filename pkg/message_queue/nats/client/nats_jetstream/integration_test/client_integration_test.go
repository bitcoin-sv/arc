package integration_test

import (
	"context"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	natsConnClient *nats.Conn
	natsConn       *nats.Conn
	mqClient       *nats_jetstream.Client
	logger         *slog.Logger
)

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "4337"
	enableJetStreamCmd := "--js"
	name := "nats-jetstream"

	resource, natsURL, err := testutils.RunNats(pool, port, name, enableJetStreamCmd)
	if err != nil {
		log.Print(err)
		return 1
	}

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	natsConnClient, err = nats_connection.New(natsURL, logger)
	if err != nil {
		log.Printf("failed to create nats connection: %v", err)
		return 1
	}

	natsConn, err = nats_connection.New(natsURL, logger)
	if err != nil {
		log.Printf("failed to create nats connection: %v", err)
		return 1
	}

	defer func() {
		err = natsConn.Drain()
		if err != nil {
			log.Fatalf("failed to drain nats connection: %v", err)
		}

		mqClient.Shutdown()

		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	time.Sleep(5 * time.Second)
	return m.Run()
}

func TestNatsClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	txBlock := &test_api.TestMessage{
		Ok: true,
	}
	t.Run("publish - work queue policy", func(t *testing.T) {
		// given
		const topic = "submit-tx"
		mqClient, err := nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithWorkQueuePolicy(topic))

		require.NoError(t, err)
		messageChan := make(chan *test_api.TestMessage, 100)
		testMessage := &test_api.TestMessage{
			Ok: true,
		}

		// when
		t.Log("subscribe to topic")
		_, err = natsConnClient.QueueSubscribe(topic, "queue", func(msg *nats.Msg) {
			serialized := &test_api.TestMessage{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			messageChan <- serialized
		})
		require.NoError(t, err)
		t.Log("publish")
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)

		counter := 0
		t.Log("wait for submitted txs")

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				t.Log("timer finished")
				break loop
			case data := <-messageChan:
				counter++
				require.Equal(t, testMessage.Ok, data.Ok)
			}
		}

		require.Equal(t, 4, counter)
	})

	t.Run("subscribe - work queue policy", func(t *testing.T) {
		// given

		const topic = "mined-txs"

		mqClient, err := nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithWorkQueuePolicy(topic))
		require.NoError(t, err)

		messageChan := make(chan *test_api.TestMessage, 100)

		// subscribe without initialized consumer, expect error
		err = mqClient.Subscribe(topic, func(_ []byte) error {
			return nil
		})
		require.ErrorIs(t, err, nats_jetstream.ErrConsumerNotInitialized)

		// subscribe with initialized consumer
		mqClient, err = nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(topic))
		require.NoError(t, err)

		err = mqClient.Subscribe(topic, func(msg []byte) error {
			serialized := &test_api.TestMessage{}
			unmarshalErr := proto.Unmarshal(msg, serialized)
			if unmarshalErr != nil {
				return unmarshalErr
			}
			messageChan <- serialized
			return nil
		})
		require.NoError(t, err)

		// when
		data, err := proto.Marshal(txBlock)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)
		err = natsConn.Publish(topic, []byte("not valid data"))
		require.NoError(t, err)

		counter := 0

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				break loop
			case minedTxBlock := <-messageChan:
				counter++
				require.Equal(t, minedTxBlock.Ok, txBlock.Ok)
			}
		}
		require.Equal(t, 3, counter)
	})

	t.Run("publish - interest policy", func(t *testing.T) {
		// given
		const topic = "interest-txs"
		mqClient, err := nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithInterestPolicy(topic))
		require.NoError(t, err)
		messageChan := make(chan *test_api.TestMessage, 100)
		testMessage := &test_api.TestMessage{
			Ok: true,
		}

		// when
		t.Log("subscribe to topic")
		_, err = natsConnClient.QueueSubscribe(topic, "queue", func(msg *nats.Msg) {
			serialized := &test_api.TestMessage{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			messageChan <- serialized
		})
		require.NoError(t, err)
		t.Log("publish")
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), topic, testMessage)
		require.NoError(t, err)

		counter := 0
		t.Log("wait for submitted txs")

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				t.Log("timer finished")
				break loop
			case data := <-messageChan:
				counter++
				require.Equal(t, testMessage.Ok, data.Ok)
			}
		}

		require.Equal(t, 4, counter)
	})

	t.Run("subscribe - interest policy", func(t *testing.T) {
		// given
		var err error
		const topic = "interest-blocks"
		minedTxsChan1 := make(chan *test_api.TestMessage, 100)
		minedTxsChan2 := make(chan *test_api.TestMessage, 100)

		mqClient = mqClientSubscribe(t, topic, "host1", minedTxsChan1)
		mqClient2 := mqClientSubscribe(t, topic, "host2", minedTxsChan2)
		defer mqClient2.Shutdown()

		// when
		data, err := proto.Marshal(txBlock)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)
		err = natsConn.Publish(topic, data)
		require.NoError(t, err)

		counter := 0
		counter2 := 0

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				break loop
			case minedTxBlock := <-minedTxsChan1:
				counter++
				require.Equal(t, minedTxBlock.Ok, txBlock.Ok)
			case minedTxBlock := <-minedTxsChan2:
				counter2++
				require.Equal(t, minedTxBlock.Ok, txBlock.Ok)
			}
		}

		require.Equal(t, 3, counter)
		require.Equal(t, 3, counter2)
	})
}

func mqClientSubscribe(t *testing.T, topic string, hostName string, minedTxsChan chan *test_api.TestMessage) *nats_jetstream.Client {
	client, err := nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithSubscribedInterestPolicy(hostName, []string{topic}, true))
	require.NoError(t, err)
	err = client.SubscribeMsg(topic, func(msg jetstream.Msg) error {
		serialized := &test_api.TestMessage{}
		unmarshlErr := proto.Unmarshal(msg.Data(), serialized)
		require.NoError(t, unmarshlErr)

		minedTxsChan <- serialized
		ackErr := msg.Ack()
		require.NoError(t, ackErr)
		return nil
	})
	require.NoError(t, err)

	return client
}
