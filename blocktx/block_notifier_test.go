package blocktx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/config"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/gocore"
	"github.com/stretchr/testify/require"
)

func TestNewBlockNotifier(t *testing.T) {
	hash822014, err := chainhash.NewHashFromStr("0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
	require.NoError(t, err)

	tt := []struct {
		name            string
		getBlockGapsErr error
	}{
		{
			name: "success",
		},
		{
			name:            "error getting block gaps",
			getBlockGapsErr: errors.New("failed to get block gaps"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			storeMock := &store.InterfaceMock{
				GetBlockGapsFunc: func(ctx context.Context) ([]*store.BlockGap, error) {
					return []*store.BlockGap{
						{
							Height: 822014,
							Hash:   hash822014,
						},
					}, tc.getBlockGapsErr
				},
			}
			blockCh := make(chan *blocktx_api.Block)

			peerSettings := []config.Peer{
				{
					Host: "127.0.0.1",
					Port: config.PeerPort{
						P2P: 18333,
						ZMQ: 28332,
					},
				},
			}

			logger := gocore.Log("test", gocore.NewLogLevelFromString("INFO"))
			peerHandler := NewPeerHandler(logger, storeMock, blockCh, 100)
			notifier, err := NewBlockNotifier(storeMock, logger, blockCh, peerHandler, peerSettings, wire.TestNet3, WithFillGapsInterval(time.Millisecond*30))
			require.NoError(t, err)

			time.Sleep(55 * time.Millisecond)
			peerHandler.Shutdown()
			notifier.Shutdown()
		})
	}
}
