package integration_test

import (
	"context"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/bitcoin-sv/arc/internal/testdata"
)

const (
	SubmitTxTopic = "submit-tx"
	MinedTxsTopic = "mined-txs"
)

var (
	natsConnClient *nats.Conn
	natsConn       *nats.Conn
	mqClient       *nats_jetstream.Client
	err            error
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

	txBlock := &blocktx_api.TransactionBlock{
		BlockHash:       testdata.Block1Hash[:],
		BlockHeight:     1,
		TransactionHash: testdata.TX1Hash[:],
		MerklePath:      "mp-1",
	}

	t.Run("publish", func(t *testing.T) {
		// given
		mqClient, err = nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithWorkQueuePolicy(SubmitTxTopic))
		require.NoError(t, err)
		submittedTxsChan := make(chan *metamorph_api.TransactionRequest, 100)
		txRequest := &metamorph_api.TransactionRequest{
			CallbackUrl:   "callback.example.com",
			CallbackToken: "test-token",
			RawTx:         testdata.TX1Raw.Bytes(),
			WaitForStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		}

		// when
		t.Log("subscribe to topic")
		_, err = natsConnClient.QueueSubscribe(SubmitTxTopic, "queue", func(msg *nats.Msg) {
			serialized := &metamorph_api.TransactionRequest{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			submittedTxsChan <- serialized
		})
		require.NoError(t, err)
		t.Log("publish")
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(context.TODO(), SubmitTxTopic, txRequest)
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
			case data := <-submittedTxsChan:
				counter++
				require.Equal(t, txRequest.CallbackUrl, data.CallbackUrl)
				require.Equal(t, txRequest.CallbackToken, data.CallbackToken)
				require.Equal(t, txRequest.RawTx, data.RawTx)
				require.Equal(t, txRequest.WaitForStatus, data.WaitForStatus)
			}
		}

		require.Equal(t, 4, counter)
	})

	t.Run("subscribe", func(t *testing.T) {
		// given
		mqClient, err = nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithWorkQueuePolicy(MinedTxsTopic))
		require.NoError(t, err)
		minedTxsChan := make(chan *blocktx_api.TransactionBlock, 100)

		// subscribe without initialized consumer, expect error
		err = mqClient.Subscribe(MinedTxsTopic, func(_ []byte) error {
			return nil
		})
		require.ErrorIs(t, err, nats_jetstream.ErrConsumerNotInitialized)

		// subscribe with initialized consumer
		mqClient, err = nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(MinedTxsTopic))
		require.NoError(t, err)

		err = mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
			serialized := &blocktx_api.TransactionBlock{}
			unmarshalErr := proto.Unmarshal(msg, serialized)
			if unmarshalErr != nil {
				return unmarshalErr
			}
			minedTxsChan <- serialized
			return nil
		})
		require.NoError(t, err)

		// when
		data, err := proto.Marshal(txBlock)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, []byte("not valid data"))
		require.NoError(t, err)

		counter := 0

		// then
	loop:
		for {
			select {
			case <-time.NewTimer(500 * time.Millisecond).C:
				break loop
			case minedTxBlock := <-minedTxsChan:
				counter++
				require.Equal(t, minedTxBlock.BlockHash, txBlock.BlockHash)
				require.Equal(t, minedTxBlock.BlockHeight, txBlock.BlockHeight)
				require.Equal(t, minedTxBlock.TransactionHash, txBlock.TransactionHash)
				require.Equal(t, minedTxBlock.MerklePath, txBlock.MerklePath)
			}
		}
		require.Equal(t, 3, counter)
	})

	t.Run("shutdown", func(t *testing.T) {
		mqClient, err = nats_jetstream.New(natsConnClient, logger, nats_jetstream.WithWorkQueuePolicy(SubmitTxTopic))
		require.NoError(t, err)
		mqClient.Shutdown()
	})
}
