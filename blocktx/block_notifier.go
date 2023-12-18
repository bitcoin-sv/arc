package blocktx

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/bitcoin-sv/arc/config"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/spf13/viper"
)

const maximumBlockSize = 4000000000 // 4Gb

type subscriber struct {
	height uint64
	stream blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer
}

type BlockNotifier struct {
	logger         utils.Logger
	storeI         store.Interface
	subscribers    map[subscriber]bool
	fillGapsTicker *time.Ticker

	newSubscriptions  chan subscriber
	deadSubscriptions chan subscriber
	blockCh           chan *blocktx_api.Block
	quitCh            chan bool
}

func NewBlockNotifier(storeI store.Interface, l utils.Logger, blockCh chan *blocktx_api.Block, peerHandler *PeerHandler, peerSettings []config.Peer, network wire.BitcoinNet) (*BlockNotifier, error) {
	bn := &BlockNotifier{
		storeI:            storeI,
		logger:            l,
		subscribers:       make(map[subscriber]bool),
		newSubscriptions:  make(chan subscriber, 128),
		deadSubscriptions: make(chan subscriber, 128),
		blockCh:           blockCh,
		fillGapsTicker:    time.NewTicker(5 * time.Minute),
	}
	pm := p2p.NewPeerManager(l, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

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
	OUT:
		for {
			select {
			case <-bn.quitCh:
				break OUT

			case s := <-bn.newSubscriptions:
				bn.subscribers[s] = true
				bn.logger.Infof("NewHandler MinedTransactions subscription received (Total=%d).", len(bn.subscribers))
				// go func() {
				// 	TODO - send all the transactions that were mined since the last time the client was connected
				// }()

			case s := <-bn.deadSubscriptions:
				delete(bn.subscribers, s)
				bn.logger.Infof("BlockNotification subscription removed (Total=%d).", len(bn.subscribers))

			case block := <-bn.blockCh:
				for sub := range bn.subscribers {
					go func(s subscriber) {
						if err := s.stream.Send(block); err != nil {
							bn.logger.Errorf("Error sending block to subscriber: %v", err)
							bn.deadSubscriptions <- s
						}
					}(sub)
				}
			}
		}
	}()

	go func() {
		for range bn.fillGapsTicker.C {
			err := peerHandler.FillGaps(peers[rand.Intn(len(peers))])
			if err != nil {
				l.Errorf("failed to fill gaps: %v", err)
			}
		}
	}()

	return bn, nil
}

// Shutdown stops the handler.
func (bn *BlockNotifier) Shutdown() {
	bn.quitCh <- true
	bn.fillGapsTicker.Stop()
}

// NewSubscription adds a new subscription to the handler.
func (bn *BlockNotifier) NewSubscription(heightAndSource *blocktx_api.Height, s blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer) {
	bn.newSubscriptions <- subscriber{
		height: heightAndSource.GetHeight(),
		stream: s,
	}

	// Keep this subscription alive without endless loop - use a channel that blocks forever.
	ch := make(chan bool)
	for {
		<-ch
	}
}

func (bn *BlockNotifier) SendBlock(block *blocktx_api.Block) {
	bn.blockCh <- block
}
