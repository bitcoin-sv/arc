package blocktx

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
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
			storeMock := &store.BlocktxStoreMock{}

			server := NewServer(storeMock, logger, nil)

			go func() {
				err := server.StartGRPCServer("localhost:8000")
				require.NoError(t, err)
			}()
			time.Sleep(10 * time.Millisecond)

			server.Shutdown()
		})
	}
}
