package integration_test

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	natsPort      = "4222"
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
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	port := "4336"
	opts := dockertest.RunOptions{
		Repository:   "nats",
		Tag:          "2.10.10",
		ExposedPorts: []string{natsPort},
		PortBindings: map[docker.Port][]docker.PortBinding{
			natsPort: {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
		Name: "nats-server",
		Cmd:  []string{"--js"},
	}

	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	hostPort := resource.GetPort(fmt.Sprintf("%s/tcp", natsPort))
	natsURL := fmt.Sprintf("nats://localhost:%s", hostPort)

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	natsConnClient, err = nats_connection.New(natsURL, logger)
	if err != nil {
		log.Fatalf("failed to create nats connection: %v", err)
	}

	natsConn, err = nats_connection.New(natsURL, logger)
	if err != nil {
		log.Fatalf("failed to create nats connection: %v", err)
	}

	time.Sleep(5 * time.Second)

	code := m.Run()

	err = natsConn.Drain()
	if err != nil {
		log.Fatalf("failed to drain nats connection: %v", err)
	}

	mqClient.Shutdown()

	err = pool.Purge(resource)
	if err != nil {
		log.Fatalf("failed to purge pool: %v", err)
	}

	os.Exit(code)
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
		mqClient, err = nats_jetstream.New(natsConnClient, logger, []string{SubmitTxTopic})
		require.NoError(t, err)
		submittedTxsChan := make(chan *metamorph_api.TransactionRequest, 100)
		t.Log("subscribe to topic")
		_, err = natsConnClient.QueueSubscribe(SubmitTxTopic, "queue", func(msg *nats.Msg) {
			serialized := &metamorph_api.TransactionRequest{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			submittedTxsChan <- serialized
		})
		require.NoError(t, err)

		txRequest := &metamorph_api.TransactionRequest{
			CallbackUrl:   "callback.example.com",
			CallbackToken: "test-token",
			RawTx:         testdata.TX1Raw.Bytes(),
			WaitForStatus: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		}

		time.Sleep(1 * time.Second)

		t.Log("publish")
		err = mqClient.PublishMarshal(SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(SubmitTxTopic, txRequest)
		require.NoError(t, err)
		err = mqClient.PublishMarshal(SubmitTxTopic, txRequest)
		require.NoError(t, err)

		counter := 0
		t.Log("wait for submitted txs")

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
		mqClient, err = nats_jetstream.New(natsConnClient, logger, []string{MinedTxsTopic})
		require.NoError(t, err)
		minedTxsChan := make(chan *blocktx_api.TransactionBlock, 100)

		err = mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
			serialized := &blocktx_api.TransactionBlock{}
			err := proto.Unmarshal(msg, serialized)
			require.NoError(t, err)
			minedTxsChan <- serialized
			return nil
		})
		require.ErrorIs(t, err, nats_jetstream.ErrConsumerNotInitialized)

		mqClient, err = nats_jetstream.New(natsConnClient, logger, []string{MinedTxsTopic}, nats_jetstream.WithSubscribedTopics(MinedTxsTopic))
		require.NoError(t, err)

		err = mqClient.Subscribe(MinedTxsTopic, func(msg []byte) error {
			serialized := &blocktx_api.TransactionBlock{}
			err := proto.Unmarshal(msg, serialized)
			require.NoError(t, err)
			minedTxsChan <- serialized
			return nil
		})

		require.NoError(t, err)

		data, err := proto.Marshal(txBlock)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)
		err = natsConn.Publish(MinedTxsTopic, data)
		require.NoError(t, err)

		counter := 0

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
}
