package blocktx

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/stretchr/testify/require"
)

//go:generate moq -out ./peer_manager_mock.go . PeerManager

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
			pm := &PeerManagerMock{ShutdownFunc: func() {}}
			server := NewServer(storeMock, logger, pm, 0)

			err := server.StartGRPCServer("localhost:7000", 10000, "", logger)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)

			server.Shutdown()
		})
	}
}
