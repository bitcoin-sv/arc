package integration_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
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

		port := "4337"
		enableJetStreamCmd := "--js"
		name := "nats-jetstream"

		resource, natsURL, err := testutils.RunNats(pool, port, name, enableJetStreamCmd)
		require.NoError(t, err)

		t.Log("nats url:", natsURL)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

		natsConn, err := nats_connection.New(natsURL, logger)
		require.NoError(t, err)
		oppositeClient, err := nats_jetstream.New(natsConn, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(mq.SubmitTxTopic))
		require.NoError(t, err)
		err = oppositeClient.Subscribe(mq.SubmitTxTopic, func(bytes []byte) error {
			logger.Info("message received", "msg", string(bytes))
			return nil
		})
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		ctx := context.Background()

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
		}

		mqClient, err := mq.NewMqClient(ctx, logger, cfg, nil, jsOpts, connOpts)
		require.NoError(t, err)
		defer mqClient.Shutdown()
		t.Log("message client created")

		err = mqClient.Publish(ctx, mq.SubmitTxTopic, []byte("abc"))
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		err = resource.Close()

		t.Log("message queue removed")

		err = mqClient.Publish(ctx, mq.SubmitTxTopic, []byte("abc"))
		require.Error(t, err)

		resource, natsURL, err = testutils.RunNats(pool, port, name, enableJetStreamCmd)
		require.NoError(t, err)
		t.Log("message queue recreated")

		time.Sleep(5 * time.Second)

		t.Log("nats url:", natsURL)

		natsConn, err = nats_connection.New(natsURL, logger)
		require.NoError(t, err)

		oppositeClient, err = nats_jetstream.New(natsConn, logger, nats_jetstream.WithSubscribedWorkQueuePolicy(mq.SubmitTxTopic))
		require.NoError(t, err)

		err = oppositeClient.Subscribe(mq.SubmitTxTopic, func(bytes []byte) error {
			logger.Info("message received", "msg", string(bytes))
			return nil
		})
		require.NoError(t, err)

		err = mqClient.Publish(ctx, mq.SubmitTxTopic, []byte("abc"))
		require.NoError(t, err)

		//err = pool.Purge(resource)
		//require.NoError(t, err)
	})
}
