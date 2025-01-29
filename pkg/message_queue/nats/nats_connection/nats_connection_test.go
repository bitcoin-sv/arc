package nats_connection

import (
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/pkg/test_utils"
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
		name          string
		url           string
		expectedError error
	}{
		{
			name: "success",
			url:  natsURL,
		},
		{
			name: "error",
			url:  "wrong url",

			expectedError: ErrNatsConnectionFailed,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.url, slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
