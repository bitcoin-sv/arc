package blocktx

import (
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/TAAL-GmbH/arc/p2p/wire"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

type subscriber struct {
	height uint64
	stream blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer
}

type BlockNotifier struct {
	logger      utils.Logger
	storeI      store.Interface
	subscribers map[subscriber]bool

	newSubscriptions  chan subscriber
	deadSubscriptions chan subscriber
	blockCh           chan *blocktx_api.Block
	quitCh            chan bool
}

func NewBlockNotifier(storeI store.Interface, l utils.Logger) *BlockNotifier {
	bn := &BlockNotifier{
		storeI:            storeI,
		logger:            l,
		subscribers:       make(map[subscriber]bool),
		newSubscriptions:  make(chan subscriber, 128),
		deadSubscriptions: make(chan subscriber, 128),
		blockCh:           make(chan *blocktx_api.Block),
	}

	network := wire.TestNet
	if gocore.Config().GetBool("mainnet", false) {
		network = wire.MainNet
	}

	pm := p2p.NewPeerManager(l, nil, network)

	peerStore := NewBlockTxPeerStore(storeI, l, bn.blockCh)

	peerCount, _ := gocore.Config().GetInt("peerCount", 0)
	if peerCount == 0 {
		l.Fatalf("peerCount must be set")
	}

	for i := 1; i <= peerCount; i++ {
		p2pURL, err, found := gocore.Config().GetURL(fmt.Sprintf("peer_%d_p2p", i))
		if !found {
			l.Fatalf("peer_%d_p2p must be set", i)
		}
		if err != nil {
			l.Fatalf("error reading peer_%d_p2p: %v", i, err)
		}

		if err := pm.AddPeer(p2pURL.Host, peerStore); err != nil {
			l.Fatalf("error adding peer %s: %v", p2pURL.Host, err)
		}
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
							bn.logger.Errorf("Error sending config")
							bn.deadSubscriptions <- s
						}
					}(sub)
				}
			}
		}
	}()

	return bn
}

// Shutdown stops the handler
func (bn *BlockNotifier) Shutdown() {
	bn.quitCh <- true
}

// NewSubscription adds a new subscription to the handler
func (bn *BlockNotifier) NewSubscription(heightAndSource *blocktx_api.Height, s blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer) {
	bn.newSubscriptions <- subscriber{
		height: heightAndSource.Height,
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
