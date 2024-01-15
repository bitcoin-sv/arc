package blocktx

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/config"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
)

const maximumBlockSize = 4000000000 // 4Gb

type BlockNotifier struct {
	logger         *slog.Logger
	storeI         store.Interface
	fillGapsTicker *time.Ticker

	quitFillBlockGap         chan struct{}
	quitFillBlockGapComplete chan struct{}
}

func WithFillGapsInterval(interval time.Duration) func(notifier *BlockNotifier) {
	return func(notifier *BlockNotifier) {
		notifier.fillGapsTicker = time.NewTicker(interval)
	}
}

func NewBlockNotifier(storeI store.Interface, l *slog.Logger, blockCh chan *blocktx_api.Block, peerHandler *PeerHandler, peerSettings []config.Peer, network wire.BitcoinNet, opts ...func(notifier *BlockNotifier)) (*BlockNotifier, error) {
	bn := &BlockNotifier{
		storeI:                   storeI,
		logger:                   l,
		fillGapsTicker:           time.NewTicker(15 * time.Minute),
		quitFillBlockGap:         make(chan struct{}),
		quitFillBlockGapComplete: make(chan struct{}),
	}
	pm := p2p.NewPeerManager(l, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

	for _, opt := range opts {
		opt(bn)
	}

	peers := make([]*p2p.Peer, len(peerSettings))
	for i, peerSetting := range peerSettings {
		var peer *p2p.Peer
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, fmt.Errorf("error getting peer url: %v", err)
		}
		peer, err = p2p.NewPeer(l, peerUrl, peerHandler, network, p2p.WithMaximumMessageSize(maximumBlockSize))
		if err != nil {
			return nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
		}

		peers[i] = peer
	}

	go func() {
		defer func() {
			bn.quitFillBlockGapComplete <- struct{}{}
		}()

		peerIndex := 0
		for {
			select {
			case <-bn.quitFillBlockGap:
				return
			case <-bn.fillGapsTicker.C:
				if peerIndex >= len(peers) {
					peerIndex = 0
				}

				l.Info("requesting missing blocks from peer", slog.Int("index", peerIndex))

				err := peerHandler.FillGaps(peers[peerIndex])
				if err != nil {
					l.Error("failed to fill gaps", slog.String("error", err.Error()))
				}

				peerIndex++
			}
		}
	}()

	return bn, nil
}

// Shutdown stops the handler.
func (bn *BlockNotifier) Shutdown() {
	bn.quitFillBlockGap <- struct{}{}
	<-bn.quitFillBlockGapComplete

	bn.fillGapsTicker.Stop()
}
