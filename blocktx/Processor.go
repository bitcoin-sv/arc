package blocktx

import (
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/mrz1836/go-logger"

	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

type ProcessorBitcoinI interface {
	GetBlock(hash string) (*bitcoin.Block, error)
	GetBlockHeaderHex(hash string) (*string, error)
	GetBlockHash(height int) (string, error)
	GetBestBlockHash() (string, error)
}

type Processor struct {
	store        store.Interface
	bitcoin      ProcessorBitcoinI
	logger       *gocore.Logger
	quitCh       chan bool
	blockHandler *BlockHandler
}

func NewBlockTxProcessor(storeI store.Interface, bitcoin ProcessorBitcoinI) (*Processor, error) {
	logger := gocore.Log("processor")

	p := &Processor{
		store:   storeI,
		bitcoin: bitcoin,
		logger:  logger,
		quitCh:  make(chan bool),
	}

	return p, nil
}

func (p *Processor) Start() {
	p.blockHandler = NewHandler(p.logger)

	pm := p2p.NewPeerManager(nil)

	peerStore := NewBlockTxPeerStore(p)

	peerCount, _ := gocore.Config().GetInt("peerCount", 0)
	if peerCount == 0 {
		logger.Fatalf("peerCount must be set")
	}

	for i := 1; i <= peerCount; i++ {
		p2pURL, err, found := gocore.Config().GetURL(fmt.Sprintf("peer_%d_p2p", i))
		if !found {
			logger.Fatalf("peer_%d_p2p must be set", i)
		}
		if err != nil {
			logger.Fatalf("error reading peer_%d_p2p: %v", i, err)
		}

		if err := pm.AddPeer(p2pURL.Host, peerStore); err != nil {
			logger.Fatalf("error adding peer %s: %v", p2pURL.Host, err)
		}
	}
}

func (p *Processor) Close() {
	close(p.quitCh)
}
