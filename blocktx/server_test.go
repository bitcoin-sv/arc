package blocktx

import (
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/config"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p/wire"
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
			storeMock := &store.InterfaceMock{}
			blockCh := make(chan *blocktx_api.Block)
			peerHandler := NewPeerHandler(logger, storeMock, blockCh, 100)
			peerSettings := []config.Peer{
				{
					Host: "127.0.0.1",
					Port: config.PeerPort{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
			}

			notifier, err := NewBlockNotifier(storeMock, logger, blockCh, peerHandler, peerSettings, wire.TestNet3, WithFillGapsInterval(time.Millisecond*30))
			require.NoError(t, err)

			server := NewServer(storeMock, notifier, logger)

			go func() {
				err := server.StartGRPCServer("localhost:8000")
				require.NoError(t, err)
			}()
			time.Sleep(10 * time.Millisecond)

			server.Shutdown()
		})
	}
}
