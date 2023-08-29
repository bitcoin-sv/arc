package blocktx

import (
	"fmt"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/spf13/viper"

	"github.com/ordishs/go-utils"
)

const maximumBlockSize = 4000000000 // 4Gb

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

type Peer struct {
	Host    string
	PortP2P int `mapstructure:"port_p2p"`
	PortZMQ int `mapstructure:"port_zmq"`
}

func GetPeerSettings() ([]Peer, error) {
	var peers []Peer
	err := viper.UnmarshalKey("peers", &peers)
	if err != nil {
		return []Peer{}, err
	}

	return peers, nil
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

	networkStr := viper.GetString("bitcoinNetwork")
	if networkStr == "" {
		l.Fatalf("bitcoin_network must be set")
	}

	var network wire.BitcoinNet

	switch networkStr {
	case "mainnet":
		network = wire.MainNet
	case "testnet":
		network = wire.TestNet3
	case "regtest":
		network = wire.TestNet
	default:
		l.Fatalf("unknown bitcoin_network: %s", networkStr)
	}

	pm := p2p.NewPeerManager(l, network, p2p.WithExcessiveBlockSize(maximumBlockSize))

	peerHandler := NewPeerHandler(l, storeI, bn.blockCh)

	peerSettings, err := GetPeerSettings()
	if err != nil {
		l.Fatalf("error getting peer settings: %v", err)
	}

	if len(peerSettings) == 0 {
		l.Fatalf("no peers configured")
	}

	for _, peerSetting := range peerSettings {
		url := fmt.Sprintf("p2p://%s:%d", peerSetting.Host, peerSetting.PortP2P)

		var peer *p2p.Peer

		peer, err := p2p.NewPeer(l, url, peerHandler, network, p2p.WithMaximumMessageSize(maximumBlockSize))
		if err != nil {
			l.Fatalf("error creating peer %s: %v", url, err)
		}

		if err = pm.AddPeer(peer); err != nil {
			l.Fatalf("error adding peer %s: %v", url, err)
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
							bn.logger.Errorf("Error sending block to subscriber: %v", err)
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
