package blocktx

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/gocore"
)

const (
	maximumBlockSize = 4000000000 // 4Gb
	logLevelDefault  = slog.LevelInfo
)

type subscriber struct {
	height uint64
	stream blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer
}

type BlockNotifier struct {
	logger      *slog.Logger
	storeI      store.Interface
	subscribers map[subscriber]bool

	newSubscriptions  chan subscriber
	deadSubscriptions chan subscriber
	blockCh           chan *blocktx_api.Block
	quitCh            chan bool
}

func WithLogger(logger *slog.Logger) func(*BlockNotifier) {
	return func(p *BlockNotifier) {
		p.logger = logger.With(slog.String("service", "blocknotifier"))
	}
}

type Option func(f *BlockNotifier)

func NewBlockNotifier(storeI store.Interface, network wire.BitcoinNet, peerSettings []Peer, opts ...Option) (*BlockNotifier, error) {

	bn := &BlockNotifier{
		storeI:            storeI,
		logger:            slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevelDefault})).With(slog.String("service", "blocknotifier")),
		subscribers:       make(map[subscriber]bool),
		newSubscriptions:  make(chan subscriber, 128),
		deadSubscriptions: make(chan subscriber, 128),
		blockCh:           make(chan *blocktx_api.Block),
	}

	for _, opt := range opts {
		opt(bn)
	}

	p2pLogger := gocore.Log("blocknotifier", gocore.NewLogLevelFromString("INFO"))
	pm := p2p.NewPeerManager(p2pLogger, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

	peerHandler := NewPeerHandler(bn.logger, storeI, bn.blockCh)

	for _, peerSetting := range peerSettings {
		var peer *p2p.Peer
		peerUrl, err := peerSetting.GetP2PUrl()
		if err != nil {
			return nil, fmt.Errorf("error getting peer url: %v", err)
		}

		peer, err = p2p.NewPeer(p2pLogger, peerUrl, peerHandler, network, p2p.WithMaximumMessageSize(maximumBlockSize))
		if err != nil {
			return nil, fmt.Errorf("error creating peer %s: %v", peerUrl, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			return nil, fmt.Errorf("error adding peer %s: %v", peerUrl, err)
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
				bn.logger.Info("NewHandler MinedTransactions subscription received", slog.Int("subscribers", len(bn.subscribers)))
				// go func() {
				// 	TODO - send all the transactions that were mined since the last time the client was connected
				// }()

			case s := <-bn.deadSubscriptions:
				delete(bn.subscribers, s)
				bn.logger.Info("BlockNotification subscription removed", slog.Int("subscribers", len(bn.subscribers)))

			case block := <-bn.blockCh:
				for sub := range bn.subscribers {
					go func(s subscriber) {
						if err := s.stream.Send(block); err != nil {
							bn.logger.Error("Error sending block to subscriber", slog.String("err", err.Error()))
							bn.deadSubscriptions <- s
						}
					}(sub)
				}
			}
		}
	}()

	return bn, nil
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
