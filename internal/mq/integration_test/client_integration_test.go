package integration_test

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	natsURL     string
	containerID string
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
		log.Print(err)
		return 1
	}

	port := "4338"
	enableJetStreamCmd := "--js"
	name := "message-queue"
	var resource *dockertest.Resource

	resource, natsURL, err = testutils.RunNats(pool, port, name, enableJetStreamCmd)
	if err != nil {
		log.Print(err)
		return 1
	}

	containerID = resource.Container.ID

	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Print(err)
		}
	}()
	time.Sleep(5 * time.Second)
	return m.Run()
}

func TestMessageQueueClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	t.Log("nats url:", natsURL)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)

	list, err := dockerClient.ContainerList(ctx, container.ListOptions{})
	require.NoError(t, err)

	for _, cont := range list {
		t.Log(cont.Names)
	}

	const waitTime = 2 * time.Second

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	var receivedCounter *atomic.Int32

	msgReceived := func(bytes []byte) error {
		logger.Info("message received", "msg", string(bytes))
		receivedCounter.Add(1)
		return nil
	}

	tt := []struct {
		name          string
		autoReconnect bool

		expectedPublishErr      error
		expectedReceivedCounter int32
	}{
		{
			name:          "auto reconnect enabled",
			autoReconnect: true,

			expectedReceivedCounter: 8,
		},
		{
			name:          "auto reconnect disabled",
			autoReconnect: false,

			expectedPublishErr:      nats_jetstream.ErrFailedToPublish,
			expectedReceivedCounter: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			receivedCounter = &atomic.Int32{}

			natsConn, err := nats_connection.New(natsURL, logger, nats_connection.WithMaxReconnects(-1))
			require.NoError(t, err)
			oppositeClient, err := nats_jetstream.New(natsConn, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(mq.SubmitTxTopic))
			require.NoError(t, err)
			defer oppositeClient.Shutdown()

			err = oppositeClient.Subscribe(mq.SubmitTxTopic, msgReceived)
			require.NoError(t, err)

			cfg := &config.MessageQueueConfig{
				Streaming: config.MessageQueueStreaming{
					Enabled:     true,
					FileStorage: false,
				},
				URL: natsURL,
			}

			jsOpts := []nats_jetstream.Option{
				nats_jetstream.WithWorkQueuePolicy(mq.SubmitTxTopic),
			}

			closedCh := make(chan struct{}, 1)
			connOpts := []nats_connection.Option{
				nats_connection.WithReconnectWait(100 * time.Millisecond),
				nats_connection.WithRetryOnFailedConnect(false),
				nats_connection.WithClientClosedChannel(closedCh),
				nats_connection.WithPingInterval(500 * time.Millisecond),
				nats_connection.WithMaxPingsOutstanding(1),
			}

			maxReconnects := nats_connection.WithMaxReconnects(1)
			if tc.autoReconnect {
				maxReconnects = nats_connection.WithMaxReconnects(-1)
			}
			connOpts = append(connOpts, maxReconnects)

			time.Sleep(waitTime)

			mqClient, err := mq.NewMqClient(logger, cfg, nil, jsOpts, connOpts)
			require.NoError(t, err)
			defer mqClient.Shutdown()
			t.Log("message client created")

			var newMessage = []byte("new message")
			err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
			require.NoError(t, err)

			err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
			require.NoError(t, err)

			err = dockerClient.ContainerPause(ctx, containerID)
			require.NoError(t, err)
			t.Log("message queue paused")

			time.Sleep(waitTime)
			err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
			require.Error(t, err)
			t.Log("publishing failed")

			err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
			require.Error(t, err)
			t.Log("publishing failed")

			if !tc.autoReconnect {
				t.Log("waiting for connection to be closed")
				select {
				case <-closedCh:
					t.Log("connection closed")
				case <-time.NewTimer(10 * time.Second).C:
					t.Fatal("connection was not closed")
				}
			}

			time.Sleep(waitTime)

			err = dockerClient.ContainerUnpause(ctx, containerID)
			require.NoError(t, err)
			t.Log("message queue unpaused")

			time.Sleep(waitTime)

			for range 4 {
				err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
				if tc.expectedPublishErr != nil {
					require.ErrorIs(t, err, tc.expectedPublishErr)
				} else {
					require.NoError(t, err)
				}
			}

			time.Sleep(waitTime)

			require.Equal(t, tc.expectedReceivedCounter, receivedCounter.Load())
		})
	}
}
