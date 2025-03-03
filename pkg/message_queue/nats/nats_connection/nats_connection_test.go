package nats_connection

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/test_api"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	natsURL  string
	resource *dockertest.Resource
	err      error
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
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "4334"
	resource, natsURL, err = testutils.RunNats(pool, port, "")
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		purgeErr := pool.Purge(resource)
		if purgeErr != nil {
			log.Println(purgeErr)
		}
	}()

	time.Sleep(5 * time.Second)
	return m.Run()
}

func TestNewNatsConnection(t *testing.T) {
	t.Run("error - wrong URL", func(t *testing.T) {
		_, err = New("wrong url", slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
		require.ErrorIs(t, err, ErrNatsConnectionFailed)
	})

	t.Run("success", func(t *testing.T) {
		clientClosedCh := make(chan struct{}, 1)

		conn, err := New(natsURL,
			slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
			WithClientClosedChannel(clientClosedCh),
			WithMaxReconnects(1),
			WithReconnectWait(100*time.Millisecond),
		)
		require.NoError(t, err)
		testMessage := &test_api.TestMessage{
			Ok: true,
		}
		var data []byte
		data, err = proto.Marshal(testMessage)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		err = conn.Publish("abc", data)
		require.NoError(t, err)

		err = resource.Close()
		require.NoError(t, err)
		timer := time.NewTimer(15 * time.Second)
		select {
		case <-clientClosedCh:
			t.Log("client closed signal received")
		case <-timer.C:
			t.Log("client closed signal not received")
			t.Fail()
		}
	})
}
