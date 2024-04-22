package async

import (
	"fmt"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"log"
	"log/slog"
	"os"
	"testing"
)

const (
	natsPort = "4222"
)

var (
	natsURL string
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	port := "4335"
	opts := dockertest.RunOptions{
		Repository:   "nats",
		Tag:          "2.10.10",
		ExposedPorts: []string{natsPort},
		PortBindings: map[docker.Port][]docker.PortBinding{
			natsPort: {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
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
	natsURL = fmt.Sprintf("nats://localhost:%s", hostPort)

	code := m.Run()

	err = pool.Purge(resource)
	if err != nil {
		log.Fatalf("failed to purge pool: %v", err)
	}

	os.Exit(code)
}

func TestNatsClient(t *testing.T) {
	txBlock := &blocktx_api.TransactionBlock{
		BlockHash:       testdata.Block1Hash[:],
		BlockHeight:     1,
		TransactionHash: testdata.TX1Hash[:],
		MerklePath:      "mp-1",
	}

	t.Run("publish mined txs", func(t *testing.T) {
		natsSubscribeMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)

		minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 100)

		natsPublishMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)

		mqClient := NewNatsMQClient(natsSubscribeMqClient, minedTxsChan, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
		t.Log("subscribing to mined txs")
		err = mqClient.SubscribeMinedTxs()
		require.NoError(t, err)

		txBlockBatch := []*blocktx_api.TransactionBlock{txBlock, txBlock, txBlock}

		data, err := proto.Marshal(&blocktx_api.TransactionBlocks{TransactionBlocks: txBlockBatch})
		require.NoError(t, err)

		err = natsPublishMqClient.Publish(minedTxsTopic, data)
		require.NoError(t, err)

		counter := 0
		for minedTxBlocks := range minedTxsChan {
			for _, minedTxBlock := range minedTxBlocks.TransactionBlocks {
				require.Equal(t, minedTxBlock.BlockHash, txBlock.BlockHash)
				require.Equal(t, minedTxBlock.BlockHeight, txBlock.BlockHeight)
				require.Equal(t, minedTxBlock.TransactionHash, txBlock.TransactionHash)
				require.Equal(t, minedTxBlock.MerklePath, txBlock.MerklePath)
				counter++

				t.Logf("counter, %d", counter+1)
			}

			t.Log("shutdown")
			err = mqClient.Shutdown()
			require.NoError(t, err)
		}

		require.Len(t, txBlockBatch, counter)

		t.Log("drain")
		err = natsPublishMqClient.Drain()
		require.NoError(t, err)

		t.Log("close")
		natsPublishMqClient.Close()
	})

	t.Run("publish register txs", func(t *testing.T) {
		natsSubscribeMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)

		natsPublishMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)
		registerTxsChannel := make(chan []byte, 10)

		t.Log("subscribe to register txs")
		_, err = natsSubscribeMqClient.QueueSubscribe(registerTxTopic, "queue", func(msg *nats.Msg) {
			t.Log("register")
			registerTxsChannel <- msg.Data
		})
		require.NoError(t, err)

		mqClient := NewNatsMQClient(natsPublishMqClient, nil, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
		require.NoError(t, err)

		t.Log("publish")
		err = mqClient.PublishRegisterTxs(testdata.TX1Hash[:])
		require.NoError(t, err)

		t.Log("wait for register txs")
		data := <-registerTxsChannel
		require.Equal(t, testdata.TX1Hash[:], data)

		t.Log("drain")
		err = natsSubscribeMqClient.Drain()
		require.NoError(t, err)

		t.Log("close")
		natsSubscribeMqClient.Close()
	})

	t.Run("publish request txs", func(t *testing.T) {
		natsSubscribeMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)

		natsPublishMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)
		requestChannel := make(chan []byte, 10)

		t.Log("subscribe to request txs")
		_, err = natsSubscribeMqClient.QueueSubscribe(requestTxTopic, "queue", func(msg *nats.Msg) {
			t.Log("request")
			requestChannel <- msg.Data
		})
		require.NoError(t, err)

		mqClient := NewNatsMQClient(natsPublishMqClient, nil, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
		require.NoError(t, err)

		t.Log("publish")
		err = mqClient.PublishRequestTx(testdata.TX1Hash[:])
		require.NoError(t, err)

		t.Log("wait for request txs")
		data := <-requestChannel
		require.Equal(t, testdata.TX1Hash[:], data)

		t.Log("drain")
		err = natsSubscribeMqClient.Drain()
		require.NoError(t, err)

		t.Log("close")
		natsSubscribeMqClient.Close()
	})
}
