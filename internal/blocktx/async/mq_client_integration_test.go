package async

import (
	"context"
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

		_, err = natsSubscribeMqClient.QueueSubscribe(minedTxsTopic, "queue", func(msg *nats.Msg) {
			serialized := &blocktx_api.TransactionBlocks{}
			unmarshalErr := proto.Unmarshal(msg.Data, serialized)
			if unmarshalErr != nil {
				t.Logf("failed to unmarshal message: %v", err)
				return
			}

			minedTxsChan <- serialized
		})

		txsBlocks := []*blocktx_api.TransactionBlock{
			txBlock, txBlock, txBlock, txBlock, txBlock,
		}

		natsPublishMqClient, err := nats_mq.NewNatsClient(natsURL)
		require.NoError(t, err)

		mqClient := NewNatsMQClient(natsPublishMqClient, nil, nil, WithMaxBatchSize(5))
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

			err = mqClient.Shutdown()
			require.NoError(t, err)

			close(minedTxsChan)
		}

		require.Len(t, txsBlocks, counter)

		err = natsSubscribeMqClient.Drain()
		require.NoError(t, err)
		natsSubscribeMqClient.Close()
	})
}
