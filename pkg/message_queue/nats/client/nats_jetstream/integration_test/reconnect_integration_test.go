package integration_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
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
		t.Logf("message received: %s", string(bytes))
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
			topic1      = "topic-1"
			streamName1 = "topic-1-stream"
			topic2      = "topic-2"
			streamName2 = "topic-2-stream"
		)
		jsOpts := []nats_jetstream.Option{
			nats_jetstream.WithStream(topic1, streamName1, jetstream.WorkQueuePolicy, false),
			nats_jetstream.WithStream(topic2, streamName2, jetstream.WorkQueuePolicy, false),
		}
		natsConn1, err := nats_connection.New(natsURL, logger.With(slog.String("conn", "1")), connOpts...)
		require.NoError(t, err)
		mqClient1, err := nats_jetstream.New(natsConn1, logger.With(slog.String("client", "1")), jsOpts...)
		require.NoError(t, err)
		defer mqClient1.Shutdown()

		// nats server is unavailable
		err = dockerClient.ContainerPause(context.TODO(), containerID)
		require.NoError(t, err)
		t.Log("message queue paused")

		// create a new client while the nats server is unavailable
		natsConn2, err := nats_connection.New(natsURL, logger.With(slog.String("conn", "2")), connOpts...)
		require.NoError(t, err)
		defer natsConn2.Status()

		mqClient2, err := nats_jetstream.New(natsConn2, logger.With(slog.String("client", "2")))
		require.NoError(t, err)
		t.Log("message client created")
		defer mqClient2.Shutdown()

		const publishTimeout = 100 * time.Millisecond
		ctxErrPublish, cancel := context.WithTimeout(context.TODO(), publishTimeout)
		defer cancel()

		for range 3 {
			err = mqClient2.Publish(ctxErrPublish, topic2, []byte("new message sent from client 2"))
			require.ErrorIs(t, err, nats_jetstream.ErrFailedToPublish)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			t.Log("failed to publish, waiting for reconnect")
		}

		receivedCounter1 := &atomic.Int32{}
		msgReceived1 := func(bytes []byte) error {
			t.Logf("client 1 message received: %s", string(bytes))
			receivedCounter1.Add(1)
			return nil
		}
		receivedCounter2 := &atomic.Int32{}
		msgReceived2 := func(bytes []byte) error {
			t.Logf("client 2 message received: %s", string(bytes))
			receivedCounter2.Add(1)
			return nil
		}

		// subscribe while nats server unavailable
		err = mqClient1.QueueSubscribe(topic2, msgReceived1)
		require.NoError(t, err)
		err = mqClient2.QueueSubscribe(topic1, msgReceived2)
		require.NoError(t, err)

		// nats server is available
		err = dockerClient.ContainerUnpause(context.TODO(), containerID)
		require.NoError(t, err)
		t.Log("message queue unpaused")

		time.Sleep(5 * time.Second)

		t.Log("publishing messages")
		for range 3 {
			err = mqClient2.Publish(context.TODO(), topic2, []byte("new message sent from client 2"))
			require.NoError(t, err)
			t.Log("message published")
		}
		for range 3 {
			err = mqClient1.Publish(context.TODO(), topic1, []byte("new message sent from client 1"))
			require.NoError(t, err)
			t.Log("message published")
		}

		time.Sleep(3 * time.Second)

		assert.Equal(t, int32(6), receivedCounter1.Load())
		assert.Equal(t, int32(3), receivedCounter2.Load())
	})
}
