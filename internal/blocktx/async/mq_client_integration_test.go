package async

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	natsPort = "4222"
)

var (
	natsConnClient *nats.Conn
	natsConn       *nats.Conn
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	port := "4333"
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
	natsURL := fmt.Sprintf("nats://localhost:%s", hostPort)

	natsConnClient, err = nats_mq.NewNatsClient(natsURL)
	if err != nil {
		log.Fatalf("failed to create nats connection: %v", err)
	}

	natsConn, err = nats_mq.NewNatsClient(natsURL)
	if err != nil {
		log.Fatalf("failed to create nats connection: %v", err)
	}

	code := m.Run()

	err = natsConn.Drain()
	if err != nil {
		log.Fatalf("failed to drain nats connection: %v", err)
	}

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

		minedTxsChan := make(chan *blocktx_api.TransactionBlocks, 100)

		_, err := natsConn.QueueSubscribe(minedTxsTopic, "queue", func(msg *nats.Msg) {
			serialized := &blocktx_api.TransactionBlocks{}
			unmarshalErr := proto.Unmarshal(msg.Data, serialized)
			if unmarshalErr != nil {
				t.Logf("failed to unmarshal message: %v", unmarshalErr)
				return
			}

			minedTxsChan <- serialized
		})
		require.NoError(t, err)

		txsBlocks := []*blocktx_api.TransactionBlock{
			txBlock, txBlock, txBlock, txBlock, txBlock,
		}

		mqClient := NewNatsMQClient(natsConnClient, nil, nil, WithMaxBatchSize(5))

		time.Sleep(1 * time.Second)

		t.Log("publish mined txs")
		err = mqClient.PublishMinedTxs(context.Background(), txsBlocks)
		require.NoError(t, err)

		counter := 0
		for minedTxBlocks := range minedTxsChan {
			for _, minedTxBlock := range minedTxBlocks.TransactionBlocks {
				require.Equal(t, minedTxBlock.BlockHash, txBlock.BlockHash)
				require.Equal(t, minedTxBlock.BlockHeight, txBlock.BlockHeight)
				require.Equal(t, minedTxBlock.TransactionHash, txBlock.TransactionHash)
				require.Equal(t, minedTxBlock.MerklePath, txBlock.MerklePath)
				counter++
			}

			close(minedTxsChan)
		}

		require.Len(t, txsBlocks, counter)
	})

	t.Run("subscribe register txs", func(t *testing.T) {
		registerTxsChannel := make(chan []byte, 10)

		mqClient := NewNatsMQClient(natsConnClient, registerTxsChannel, nil, WithMaxBatchSize(5))

		t.Log("subscribe register txs")
		err := mqClient.SubscribeRegisterTxs()
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		t.Log("publish tx")
		err = natsConn.Publish(registerTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(registerTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(registerTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(registerTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)

		counter := 0

		t.Log("wait for registered tx")
	loop:
		for {
			select {
			case <-time.NewTimer(15 * time.Second).C:
				t.Fatal("receiving registered tx timed out")
			case data := <-registerTxsChannel:
				counter++
				require.Equal(t, testdata.TX1Hash[:], data)
				if counter == 4 {
					break loop
				}
			}
		}

	})

	t.Run("subscribe request txs", func(t *testing.T) {
		requestTxsChannel := make(chan []byte, 10)

		mqClient := NewNatsMQClient(natsConnClient, nil, requestTxsChannel, WithMaxBatchSize(5))

		t.Log("subscribe request txs")
		err := mqClient.SubscribeRequestTxs()
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		t.Log("publish tx")
		err = natsConn.Publish(requestTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(requestTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(requestTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)
		err = natsConn.Publish(requestTxTopic, testdata.TX1Hash[:])
		require.NoError(t, err)

		counter := 0

		t.Log("wait for requested tx")

	loop:
		for {
			select {
			case <-time.NewTimer(15 * time.Second).C:
				t.Fatal("receiving requested tx timed out")
			case data := <-requestTxsChannel:
				counter++
				require.Equal(t, testdata.TX1Hash[:], data)
				if counter == 4 {
					break loop
				}
			}
		}

		err = mqClient.Shutdown()
		require.NoError(t, err)
	})
}
