package integration_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/mq"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

func TestNatsClient(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("publish - work queue policy", func(t *testing.T) {
		pool, err := dockertest.NewPool("")
		require.NoError(t, err)

		ctx := context.Background()
		port := "4337"
		enableJetStreamCmd := "--js"
		name := "nats-jetstream"

		resource, natsURL, err := testutils.RunNats(pool, port, name, enableJetStreamCmd)
		require.NoError(t, err)
		defer pool.Purge(resource)

		t.Log("nats url:", natsURL)

		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		require.NoError(t, err)

		list, err := dockerClient.ContainerList(ctx, container.ListOptions{})
		require.NoError(t, err)

		for _, cont := range list {
			t.Log(cont.Names)
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		natsConn, err := nats_connection.New(natsURL, logger)
		require.NoError(t, err)
		oppositeClient, err := nats_jetstream.New(natsConn, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(mq.SubmitTxTopic))
		require.NoError(t, err)

		receivedCounter := 0
		msgReceived := func(bytes []byte) error {
			logger.Info("message received", "msg", string(bytes))
			receivedCounter++
			return nil
		}

		err = oppositeClient.Subscribe(mq.SubmitTxTopic, msgReceived)
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

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

		connOpts := []nats_connection.Option{
			nats_connection.WithMaxReconnects(1),
			nats_connection.WithReconnectWait(100 * time.Millisecond),
			nats_connection.WithRetryOnFailedConnect(false),
		}

		mqClient, err := mq.NewMqClient(ctx, logger, cfg, nil, jsOpts, connOpts)
		require.NoError(t, err)
		defer mqClient.Shutdown()
		t.Log("message client created")

		var newMessage = []byte("new message")
		err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		err = dockerClient.ContainerPause(ctx, resource.Container.ID)
		require.NoError(t, err)
		t.Log("message queue paused")

		time.Sleep(5 * time.Second)
		err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
		require.Error(t, err)
		t.Log("publishing failed")
		time.Sleep(60 * time.Second)

		err = dockerClient.ContainerUnpause(ctx, resource.Container.ID)
		require.NoError(t, err)
		t.Log("message queue unpaused")

		time.Sleep(5 * time.Second)

		natsConn, err = nats_connection.New(natsURL, logger)
		require.NoError(t, err)

		oppositeClient, err = nats_jetstream.New(natsConn, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(mq.SubmitTxTopic))
		require.NoError(t, err)

		err = oppositeClient.Subscribe(mq.SubmitTxTopic, msgReceived)
		require.NoError(t, err)

		err = mqClient.Publish(ctx, mq.SubmitTxTopic, newMessage)
		assert.NoError(t, err)

		mqClient.Shutdown()
		oppositeClient.Shutdown()
		time.Sleep(5 * time.Second)

		require.Equal(t, 3, receivedCounter)
	})
}
