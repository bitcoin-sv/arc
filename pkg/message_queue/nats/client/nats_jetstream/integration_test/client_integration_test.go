package integration_test

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	natsConnClient   *nats.Conn
	natsConnOpposite *nats.Conn
	mqClient         *nats_jetstream.Client
	logger           *slog.Logger
	err              error
)

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		os.Exit(0)
	}

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

	natsConnOpposite, err = nats_connection.New(natsURL, logger)
	if err != nil {
		log.Printf("failed to create nats connection: %v", err)
		return 1
	}

	defer func() {
		mqClient.Shutdown()

		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	time.Sleep(5 * time.Second)
	return m.Run()
}

func TestPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tm := &test_api.TestMessage{
		Ok: true,
	}

	tt := []struct {
		name     string
		topic    string
		opts     []nats_jetstream.Option
		testFunc func(cl mq.MessageQueueClient, topic string, msg *test_api.TestMessage)
	}{
		{
			name:  "publish marshal - work queue policy",
			topic: "topic-1",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("topic-1", "topic-1-stream", jetstream.WorkQueuePolicy, false),
				nats_jetstream.WithConsumer("topic-1", "topic-1-stream", "topic-1-cons", false, jetstream.AckExplicitPolicy),
			},
			testFunc: func(cl mq.MessageQueueClient, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshal(context.TODO(), topic, msg)
				require.NoError(t, err)
			},
		},
		{
			name:  "publish marshal async - work queue policy",
			topic: "topic-2",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("topic-2", "topic-2-stream", jetstream.WorkQueuePolicy, true),
				nats_jetstream.WithConsumer("topic-2", "topic-2-stream", "topic-2-cons", false, jetstream.AckExplicitPolicy),
			},
			testFunc: func(cl mq.MessageQueueClient, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshalAsync(topic, msg)
				require.NoError(t, err)
			},
		},
		{
			name:  "publish marshal async - interest policy",
			topic: "topic-3",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("topic-3", "topic-3-stream", jetstream.InterestPolicy, false),
				nats_jetstream.WithConsumer("topic-3", "topic-3-stream", "topic-3-cons", false, jetstream.AckExplicitPolicy),
			},
			testFunc: func(cl mq.MessageQueueClient, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshal(context.TODO(), topic, msg)
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mqClient, err = nats_jetstream.New(natsConnClient, logger, tc.opts...)
			require.NoError(t, err)
			messageChan := make(chan *test_api.TestMessage, 100)
			testMessage := &test_api.TestMessage{
				Ok: true,
			}

			// when
			t.Log("subscribe to topic")
			_, err = natsConnClient.QueueSubscribe(tc.topic, "queue", func(msg *nats.Msg) {
				serialized := &test_api.TestMessage{}
				err := proto.Unmarshal(msg.Data, serialized)
				if assert.NoError(t, err) {
					messageChan <- serialized
				}
			})
			require.NoError(t, err)
			t.Log("publish")
			for range 4 {
				tc.testFunc(mqClient, tc.topic, testMessage)
			}

			counter := 0
			t.Log("wait for submitted txs")

			// then
		loop:
			for {
				select {
				case <-time.NewTimer(500 * time.Millisecond).C:
					t.Fatal("timeout waiting for submitted txs")
				case data := <-messageChan:
					require.Equal(t, testMessage.Ok, data.Ok)

					counter++
					if counter >= 4 {
						break loop
					}
				}
			}
		})
	}

	t.Run("subscribe - interest policy", func(t *testing.T) {
		// given
		const topic = "topic-4"
		minedTxsChan1 := make(chan *test_api.TestMessage, 100)
		minedTxsChan2 := make(chan *test_api.TestMessage, 100)

		mqClient = mqClientSubscribe(t, "host1", topic, minedTxsChan1)
		mqClient2 := mqClientSubscribe(t, "host2", topic, minedTxsChan2)
		defer mqClient2.Shutdown()

		// when
		data, err := proto.Marshal(tm)
		require.NoError(t, err)
		err = natsConnOpposite.Publish(topic, data)
		require.NoError(t, err)
		err = natsConnOpposite.Publish(topic, data)
		require.NoError(t, err)
		err = natsConnOpposite.Publish(topic, data)
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
				assert.Equal(t, minedTxBlock.Ok, tm.Ok)
			case minedTxBlock := <-minedTxsChan2:
				counter2++
				assert.Equal(t, minedTxBlock.Ok, tm.Ok)
			}
		}

		assert.Equal(t, 3, counter)
		assert.Equal(t, 3, counter2)
	})
}

func TestSubscribe(t *testing.T) {
	tm := &test_api.TestMessage{
		Ok: true,
	}

	tt := []struct {
		name     string
		topic    string
		opts     []nats_jetstream.Option
		testFunc func(cl mq.MessageQueueClient, topic string, msg []byte, messageChan chan *test_api.TestMessage)
	}{
		{
			name:  "Subscribe - work queue policy",
			topic: "topic-1",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("topic-1", "topic-1-stream", jetstream.WorkQueuePolicy, false),
				nats_jetstream.WithConsumer("topic-1", "topic-1-stream", "topic-1-cons", true, jetstream.AckExplicitPolicy),
			},
			testFunc: func(cl mq.MessageQueueClient, topic string, msg []byte, messageChan chan *test_api.TestMessage) {
				err = cl.Subscribe(topic, func(bytes []byte) error {
					serialized := &test_api.TestMessage{}
					unmarshalErr := proto.Unmarshal(msg, serialized)
					if unmarshalErr != nil {
						return unmarshalErr
					}
					messageChan <- serialized
					return nil
				})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			mqClient, err = nats_jetstream.New(natsConnClient, logger)
			require.NoError(t, err)

			messageChan := make(chan *test_api.TestMessage, 100)

			// subscribe with initialized consumer
			mqClient, err = nats_jetstream.New(natsConnClient, logger, tc.opts...)
			require.NoError(t, err)

			tc.testFunc(mqClient, tc.topic, []byte(tc.topic), messageChan)

			// when
			data, err := proto.Marshal(tm)

			for range 4 {
				require.NoError(t, err)
				err = natsConnOpposite.Publish(tc.topic, data)
			}
			err = natsConnOpposite.Publish(tc.topic, []byte("not valid data"))
			require.NoError(t, err)

			counter := 0

			// then
		loop:
			for {
				select {
				case <-time.NewTimer(500 * time.Millisecond).C:
					t.Fatal("timeout waiting for submitted txs")
				case minedTxBlock := <-messageChan:
					require.Equal(t, minedTxBlock.Ok, tm.Ok)
					counter++
					if counter >= 4 {
						break loop
					}
				}
			}
		})
	}
}

func mqClientSubscribe(t *testing.T, hostname string, topic string, minedTxsChan chan *test_api.TestMessage) *nats_jetstream.Client {
	streamName := fmt.Sprintf("%s-stream", topic)
	consName := fmt.Sprintf("%s-%s-cons", hostname, topic)

	mqOpts := []nats_jetstream.Option{
		nats_jetstream.WithStream(topic, streamName, jetstream.InterestPolicy, false),
		nats_jetstream.WithConsumer(topic, streamName, consName, false, jetstream.AckExplicitPolicy),
	}

	client, err := nats_jetstream.New(natsConnClient, logger, mqOpts...)
	require.NoError(t, err)
	err = client.SubscribeMsg(topic, func(msg jetstream.Msg) error {
		t.Log("got message")
		serialized := &test_api.TestMessage{}
		unmarshalErr := proto.Unmarshal(msg.Data(), serialized)
		if assert.NoError(t, unmarshalErr) {
			minedTxsChan <- serialized
		}
		ackErr := msg.Ack()
		assert.NoError(t, ackErr)
		return nil
	})
	assert.NoError(t, err)

	return client
}
