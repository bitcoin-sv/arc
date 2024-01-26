package callbacker

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/callbacker/store/mock_gen"
	"github.com/stretchr/testify/require"
)

func TestStartGRPCServer(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "start and shutdown",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			store := &mock_gen.StoreMock{}

			cb, err := New(store, nil)
			require.NoError(t, err)

			server := NewServer(logger, cb)

			go func() {
				err := server.StartGRPCServer("localhost:8000")
				require.NoError(t, err)
			}()
			time.Sleep(10 * time.Millisecond)

			server.Shutdown()
		})
	}
}
