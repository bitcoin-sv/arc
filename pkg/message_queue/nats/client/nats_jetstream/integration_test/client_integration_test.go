package integration_test

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	logger      *slog.Logger
	err         error
	natsURL     string
	containerID string
)

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		return
	}

	testmain(m)
}

func testmain(m *testing.M) int {
	var pool *dockertest.Pool
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "4337"
	enableJetStreamCmd := "--js"
	name := "nats-jetstream"

	var resource *dockertest.Resource
	resource, natsURL, err = testutils.RunNats(pool, port, name, enableJetStreamCmd)
	if err != nil {
		log.Print(err)
		return 1
	}

	containerID = resource.Container.ID

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	defer func() {
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

	consume := func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage) {
		err = cl.Consume(topic, func(bytes []byte) error {
			serialized := &test_api.TestMessage{}
			err := proto.Unmarshal(bytes, serialized)
			if assert.NoError(t, err) {
				messageChan <- serialized
			}

			return nil
		})
		require.NoError(t, err)
	}

	tt := []struct {
		name          string
		topic         string
		opts          []nats_jetstream.Option
		testFunc      func(cl *nats_jetstream.Client, topic string, msg *test_api.TestMessage)
		subscribeFunc func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage)
	}{
		{
			name:  "publish marshal - work queue policy",
			topic: "pub-topic-1",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("pub-topic-1", "pub-topic-1-stream", jetstream.WorkQueuePolicy, false, true),
				nats_jetstream.WithConsumer("pub-topic-1", "pub-topic-1-stream", "pub-topic-1-cons", false, jetstream.AckExplicitPolicy, true),
			},
			testFunc: func(cl *nats_jetstream.Client, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshal(context.TODO(), topic, msg)
				require.NoError(t, err)
			},
			subscribeFunc: consume,
		},
		{
			name:  "publish marshal async - work queue policy",
			topic: "pub-topic-2",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("pub-topic-2", "pub-topic-2-stream", jetstream.WorkQueuePolicy, true, true),
				nats_jetstream.WithConsumer("pub-topic-2", "pub-topic-2-stream", "pub-topic-2-cons", false, jetstream.AckExplicitPolicy, true),
			},
			testFunc: func(cl *nats_jetstream.Client, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshalAsync(topic, msg)
				require.NoError(t, err)
			},
			subscribeFunc: consume,
		},
		{
			name:  "publish marshal - interest policy",
			topic: "pub-topic-3",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("pub-topic-3", "pub-topic-3-stream", jetstream.InterestPolicy, false, true),
				nats_jetstream.WithConsumer("pub-topic-3", "pub-topic-3-stream", "pub-topic-3-cons", false, jetstream.AckExplicitPolicy, true),
			},
			testFunc: func(cl *nats_jetstream.Client, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshal(context.TODO(), topic, msg)
				require.NoError(t, err)
			},
			subscribeFunc: consume,
		},
		{
			name:  "publish marshal core",
			topic: "pub-topic-4",
			opts:  []nats_jetstream.Option{},
			testFunc: func(cl *nats_jetstream.Client, topic string, msg *test_api.TestMessage) {
				err = cl.PublishMarshalCore(topic, msg)
				require.NoError(t, err)
			},
			subscribeFunc: func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage) {
				err = cl.QueueSubscribe(topic, func(bytes []byte) error {
					serialized := &test_api.TestMessage{}
					err := proto.Unmarshal(bytes, serialized)
					if assert.NoError(t, err) {
						messageChan <- serialized
					}

					return nil
				})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsConnClient, err := nats_connection.New(natsURL, logger)
			require.NoError(t, err)
			mqClient, err := nats_jetstream.New(natsConnClient, logger, tc.opts...)
			require.NoError(t, err)
			defer mqClient.Shutdown()

			// when
			messageChan := make(chan *test_api.TestMessage, 100)
			tm := &test_api.TestMessage{
				Ok: true,
			}
			t.Log("subscribe to topic")
			tc.subscribeFunc(mqClient, tc.topic, messageChan)

			t.Log("run test function 4 times")
			for range 4 {
				tc.testFunc(mqClient, tc.topic, tm)
			}

			counter := 0
			t.Log("wait for submitted txs")

			// then
		loop:
			for {
				select {
				case <-time.NewTimer(1000 * time.Millisecond).C:
					t.Fatal("timeout waiting for submitted txs")
				case data := <-messageChan:
					require.Equal(t, tm.Ok, data.Ok)
					t.Log("received message")

					counter++
					if counter >= 4 {
						break loop
					}
				}
			}
		})
	}
}

func TestSubscribe(t *testing.T) {
	tt := []struct {
		name     string
		topic    string
		opts     []nats_jetstream.Option
		testFunc func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage)
	}{
		{
			name:  "subscribe - work queue policy",
			topic: "sub-topic-1",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("sub-topic-1", "sub-topic-1-stream", jetstream.WorkQueuePolicy, false, true),
				nats_jetstream.WithConsumer("sub-topic-1", "sub-topic-1-stream", "sub-topic-1-cons", false, jetstream.AckExplicitPolicy, true),
			},
			testFunc: func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage) {
				err = cl.Consume(topic, func(bytes []byte) error {
					serialized := &test_api.TestMessage{}
					unmarshalErr := proto.Unmarshal(bytes, serialized)
					if unmarshalErr != nil {
						return unmarshalErr
					}
					messageChan <- serialized
					return nil
				})
				require.NoError(t, err)
			},
		},
		{
			name:  "subscribe msg - interest policy",
			topic: "sub-topic-2",
			opts: []nats_jetstream.Option{
				nats_jetstream.WithStream("sub-topic-2", "sub-topic-2-stream", jetstream.InterestPolicy, false, true),
				nats_jetstream.WithConsumer("sub-topic-2", "sub-topic-2-stream", "sub-topic-2-cons", false, jetstream.AckExplicitPolicy, true),
			},
			testFunc: func(cl *nats_jetstream.Client, topic string, messageChan chan *test_api.TestMessage) {
				err = cl.ConsumeMsg(topic, func(msg jetstream.Msg) error {
					serialized := &test_api.TestMessage{}
					unmarshalErr := proto.Unmarshal(msg.Data(), serialized)
					if assert.NoError(t, unmarshalErr) {
						messageChan <- serialized
					}
					ackErr := msg.Ack()
					assert.NoError(t, ackErr)
					return nil
				})
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			natsConnClient, err := nats_connection.New(natsURL, logger)
			require.NoError(t, err)

			natsConnOpposite, err := nats_connection.New(natsURL, logger)
			require.NoError(t, err)

			mqClient, err := nats_jetstream.New(natsConnClient, logger, tc.opts...)
			require.NoError(t, err)
			defer mqClient.Shutdown()

			messageChan := make(chan *test_api.TestMessage, 10)

			// subscribe with initialized consumer
			mqClientOpposite, err := nats_jetstream.New(natsConnOpposite, logger, tc.opts...)
			require.NoError(t, err)

			tc.testFunc(mqClient, tc.topic, messageChan)
			tm := &test_api.TestMessage{
				Ok: true,
			}
			// when
			for range 4 {
				err = mqClientOpposite.PublishMarshal(context.TODO(), tc.topic, tm)
				require.NoError(t, err)
			}

			counter := 0

			// then
		loop:
			for {
				select {
				case <-time.NewTimer(500 * time.Millisecond).C:
					t.Fatal("timeout waiting for submitted txs")
				case minedTxBlock := <-messageChan:
					t.Log("received message")
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
