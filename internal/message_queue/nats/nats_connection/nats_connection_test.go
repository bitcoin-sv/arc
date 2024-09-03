package nats_connection

import (
	"log"
	"log/slog"
	"os"
	"testing"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

var (
	natsURL string
)

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "4334"
	resource, natsInfo, err := testutils.RunNats(pool, port, "")
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	natsURL = natsInfo
	return m.Run()
}

func TestNewNatsConnection(t *testing.T) {
	tt := []struct {
		name string
		url  string

		expectedErrorStr string
	}{
		{
			name: "success",
			url:  natsURL,
		},
		{
			name: "error",
			url:  "wrong url",

			expectedErrorStr: "failed to connect to NATS server",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.url, slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			}

			require.NoError(t, err)
		})
	}
}
