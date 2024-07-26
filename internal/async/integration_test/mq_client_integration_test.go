package integration_test

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/async"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/nats_mq"
	"github.com/bitcoin-sv/arc/internal/testdata"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	natsConnClient, err = nats_mq.NewNatsClient(natsURL, logger)
	if err != nil {
		log.Fatalf("failed to create nats connection: %v", err)
	}

	natsConn, err = nats_mq.NewNatsClient(natsURL, logger)
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
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	txBlock := &blocktx_api.TransactionBlock{
		BlockHash:       testdata.Block1Hash[:],
		BlockHeight:     1,
		TransactionHash: testdata.TX1Hash[:],
		MerklePath:      "mp-1",
	}

	minedTxsChan := make(chan *blocktx_api.TransactionBlock, 100)
	submittedTxsChan := make(chan *metamorph_api.TransactionRequest, 100)

	mqClient := async.NewNatsMQClient(natsConnClient)

	t.Run("publish", func(t *testing.T) {
		t.Log("subscribe to topic")
		_, err := natsConnClient.QueueSubscribe(async.SubmitTxTopic, "queue", func(msg *nats.Msg) {
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
		txRequests := &metamorph_api.TransactionRequests{Transactions: []*metamorph_api.TransactionRequest{txRequest, txRequest, txRequest, txRequest}}

		time.Sleep(1 * time.Second)

		t.Log("publish")
		err = mqClient.PublishMarshal(async.SubmitTxTopic, txRequests)
		require.NoError(t, err)

		counter := 0
		t.Log("wait for submitted txs")
	loop:
		for {
			select {
			case <-time.NewTimer(15 * time.Second).C:
				t.Fatal("receiving submitted tx timed out")
			case data := <-submittedTxsChan:
				counter++
				require.Equal(t, txRequest.CallbackUrl, data.CallbackUrl)
				require.Equal(t, txRequest.CallbackToken, data.CallbackToken)
				require.Equal(t, txRequest.RawTx, data.RawTx)
				require.Equal(t, txRequest.WaitForStatus, data.WaitForStatus)
				if counter >= 4 {
					break loop
				}
			}
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		err := mqClient.Subscribe(async.MinedTxsTopic, func(msg *nats.Msg) {
			serialized := &blocktx_api.TransactionBlock{}
			err := proto.Unmarshal(msg.Data, serialized)
			require.NoError(t, err)
			minedTxsChan <- serialized
		})
		require.NoError(t, err)

		txBlockBatch := []*blocktx_api.TransactionBlock{txBlock, txBlock, txBlock}

		data, err := proto.Marshal(&blocktx_api.TransactionBlocks{TransactionBlocks: txBlockBatch})
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		err = natsConn.Publish(async.MinedTxsTopic, data)
		require.NoError(t, err)

		counter := 0
		for minedTxBlock := range minedTxsChan {

			require.Equal(t, minedTxBlock.BlockHash, txBlock.BlockHash)
			require.Equal(t, minedTxBlock.BlockHeight, txBlock.BlockHeight)
			require.Equal(t, minedTxBlock.TransactionHash, txBlock.TransactionHash)
			require.Equal(t, minedTxBlock.MerklePath, txBlock.MerklePath)
			counter++

			t.Logf("counter, %d", counter)
			if counter > 1 {
				break
			}
		}

		require.Len(t, txBlockBatch, counter)

		err = natsConn.Publish(async.MinedTxsTopic, []byte("not a valid data format"))
		require.NoError(t, err)

	})
}
