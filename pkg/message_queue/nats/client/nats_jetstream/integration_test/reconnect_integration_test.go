package integration_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_jetstream"
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/nats_connection"
)

func TestReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)

	list, err := dockerClient.ContainerList(ctx, container.ListOptions{})
	require.NoError(t, err)

	for _, cont := range list {
		t.Log(cont.Names)
	}

	var receivedCounter *atomic.Int32
	msgReceived := func(bytes []byte) error {
		logger.Info("message received", "msg", string(bytes))
		receivedCounter.Add(1)
		return nil
	}

	tt := []struct {
		name          string
		autoReconnect bool
		topic         string

		expectedPublishErr      error
		expectedReceivedCounter int32
	}{
		{
			name:          "auto reconnect enabled",
			autoReconnect: true,
			topic:         "recon-topic-enabled",

			expectedReceivedCounter: 8,
		},
		{
			name:          "auto reconnect disabled",
			autoReconnect: false,
			topic:         "recon-topic-disabled",

			expectedPublishErr:      nats_jetstream.ErrFailedToPublish,
			expectedReceivedCounter: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			receivedCounter = &atomic.Int32{}

			natsConn, err := nats_connection.New(natsURL, logger, nats_connection.WithMaxReconnects(-1))
			require.NoError(t, err)

			streamName := fmt.Sprintf("%s-stream", tc.topic)
			consName := fmt.Sprintf("%s-cons", tc.topic)

			jsOpts := []nats_jetstream.Option{
				nats_jetstream.WithStream(tc.topic, streamName, jetstream.WorkQueuePolicy, false),
				nats_jetstream.WithConsumer(tc.topic, streamName, consName, true, jetstream.AckExplicitPolicy),
			}

			oppositeClient, err := nats_jetstream.New(natsConn, logger, jsOpts...)
			require.NoError(t, err)
			defer oppositeClient.Shutdown()

			err = oppositeClient.Consume(tc.topic, msgReceived)
			require.NoError(t, err)

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

			natsConn, err = nats_connection.New(natsURL, logger, connOpts...)
			require.NoError(t, err)

			mqClient, err := nats_jetstream.New(natsConn, logger)
			require.NoError(t, err)
			defer mqClient.Shutdown()
			t.Log("message client created")

			var newMessage = []byte("new message")

			for range 2 {
				err = mqClient.Publish(ctx, tc.topic, newMessage)
				require.NoError(t, err)
			}

			err = dockerClient.ContainerPause(ctx, containerID)
			require.NoError(t, err)
			t.Log("message queue paused")

			if !tc.autoReconnect {
				t.Log("waiting for connection to be closed")
				select {
				case <-closedCh:
					t.Log("connection closed")
				case <-time.NewTimer(10 * time.Second).C:
					t.Fatal("connection was not closed")
				}
			}

			for range 2 {
				err = mqClient.Publish(ctx, tc.topic, newMessage)
				require.ErrorIs(t, err, nats_jetstream.ErrFailedToPublish)
				t.Log("publishing failed")
			}

			err = dockerClient.ContainerUnpause(ctx, containerID)
			require.NoError(t, err)
			t.Log("message queue unpaused")

			for range 4 {
				err = mqClient.Publish(ctx, tc.topic, newMessage)
				if tc.expectedPublishErr != nil {
					require.ErrorIs(t, err, tc.expectedPublishErr)
				} else {
					require.NoError(t, err)
				}
			}

			time.Sleep(1 * time.Second)

			require.Equal(t, tc.expectedReceivedCounter, receivedCounter.Load())
		})
	}
}

func TestInitialAutoReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("auto reconnect after server is initially unavailable", func(t *testing.T) {
		ctx := context.Background()

		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		require.NoError(t, err)

		connOpts := []nats_connection.Option{
			nats_connection.WithReconnectWait(100 * time.Millisecond),
			nats_connection.WithRetryOnFailedConnect(true),
			nats_connection.WithPingInterval(500 * time.Millisecond),
			nats_connection.WithMaxPingsOutstanding(1),
			nats_connection.WithMaxReconnects(-1),
		}

		const (
			topic      = "recon-topic"
			streamName = "recon-topic-stream"
			consName   = "recon-topic-cons"
		)
		jsOpts := []nats_jetstream.Option{
			nats_jetstream.WithStream(topic, streamName, jetstream.WorkQueuePolicy, false),
			nats_jetstream.WithConsumer(topic, streamName, consName, true, jetstream.AckExplicitPolicy),
		}
		natsConn, err := nats_connection.New(natsURL, logger, connOpts...)
		require.NoError(t, err)
		mqClient, err := nats_jetstream.New(natsConn, logger, jsOpts...)
		require.NoError(t, err)
		defer mqClient.Shutdown()

		err = dockerClient.ContainerPause(ctx, containerID)
		require.NoError(t, err)
		t.Log("message queue paused")

		natsConnRecon, err := nats_connection.New(natsURL, logger, connOpts...)
		require.NoError(t, err)
		defer natsConnRecon.Status()

		mqClientRecon, err := nats_jetstream.New(natsConnRecon, logger)
		require.NoError(t, err)
		t.Log("message client created")
		defer mqClientRecon.Shutdown()

		var newMessage = []byte("new message")

		const publishTimeout = 100 * time.Millisecond
		for range 3 {
			ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
			err = mqClientRecon.Publish(ctx, topic, newMessage)
			defer cancel()
			require.ErrorIs(t, err, nats_jetstream.ErrFailedToPublish)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			t.Log("failed to publish, waiting for reconnect")
		}

		err = dockerClient.ContainerUnpause(ctx, containerID)
		require.NoError(t, err)
		t.Log("message queue unpaused")

		for range 3 {
			ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
			err = mqClientRecon.Publish(ctx, topic, newMessage)
			defer cancel()
			require.NoError(t, err)
			t.Log("message published")
		}
	})
}
