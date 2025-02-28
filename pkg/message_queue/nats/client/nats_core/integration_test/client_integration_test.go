package integration_test

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_core"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

const (
	SubmitTxTopic = "submit-tx"
	MinedTxsTopic = "mined-txs"
)

var (
	natsConnClient *nats.Conn
	natsConn       *nats.Conn
	mqClient       *nats_core.Client
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

	port := "4336"
	name := "nats-core"
	resource, natsURL, err := testutils.RunNats(pool, port, name)
	if err != nil {
		log.Print(err)
		return 1
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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

	testMessage := &test_api.TestMessage{
		Ok: true,
	}

	t.Run("publish", func(t *testing.T) {
		// given
		mqClient = nats_core.New(natsConnClient)
		messageChan := make(chan *test_api.TestMessage, 100)
		message := &test_api.TestMessage{
			Ok: true,
		}

		// when
		t.Log("subscribe to topic")
		_, err := natsConnClient.QueueSubscribe(SubmitTxTopic, "queue", func(msg *nats.Msg) {
			serialized := &test_api.TestMessage{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			messageChan <- serialized
		})
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		t.Log("publish")
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, message)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, message)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, message)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, message)
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
				require.Equal(t, message.Ok, data.Ok)
			}
		}

		require.Equal(t, 4, counter)
	})

	t.Run("subscribe", func(t *testing.T) {
		// given
		mqClient = nats_core.New(natsConnClient)
		messageChan := make(chan *test_api.TestMessage, 100)

		// when
		err := mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
			serialized := &test_api.TestMessage{}
			err := proto.Unmarshal(msg, serialized)
			require.NoError(t, err)
			messageChan <- serialized
			return nil
		})
		require.NoError(t, err)

		data, err := proto.Marshal(testMessage)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)

		counter := 0

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				break loop
			case message := <-messageChan:
				counter++
				require.Equal(t, message.Ok, testMessage.Ok)
			}
		}
		require.Equal(t, 3, counter)
	})
}
