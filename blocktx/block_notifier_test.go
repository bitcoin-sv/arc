package blocktx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/config"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
)

func TestNewBlockNotifier(t *testing.T) {
	hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
	require.NoError(t, err)
	hostname, err := os.Hostname()
	require.NoError(t, err)

	tt := []struct {
		name            string
		hostname        string
		getBlockGapsErr error
	}{
		{
			name:     "success",
			hostname: hostname,
		},
		{
			name:            "error getting block gaps",
			hostname:        hostname,
			getBlockGapsErr: errors.New("failed to get block gaps"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			storeMock := &store.InterfaceMock{
				GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
					return []*store.BlockGap{
						{
							Height: 822014,
							Hash:   hash822014,
						},
					}, tc.getBlockGapsErr
				},
				PrimaryBlocktxFunc: func(ctx context.Context) (string, error) {
					return tc.hostname, nil
				},
			}

			peerSettings := []config.Peer{
				{
					Host: "127.0.0.1",
					Port: config.PeerPort{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
				{
					Host: "127.0.0.2",
					Port: config.PeerPort{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
				{
					Host: "127.0.0.3",
					Port: config.PeerPort{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			peerHandler := NewPeerHandler(logger, storeMock, 100)
			notifier, err := NewBlockNotifier(storeMock, logger, peerHandler, peerSettings, wire.TestNet3, WithFillGapsInterval(time.Millisecond*30))
			require.NoError(t, err)

			time.Sleep(120 * time.Millisecond)
			peerHandler.Shutdown()
			notifier.Shutdown()
		})
	}
}
