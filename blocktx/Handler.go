package blocktx

import (
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"

	"github.com/ordishs/go-utils"
)

type subscriber struct {
	height uint64
	stream blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer
}

type BlockHandler struct {
	logger      utils.Logger
	subscribers map[subscriber]bool

	newSubscriptions  chan subscriber
	deadSubscriptions chan subscriber
	blockCh           chan *blocktx_api.Block
	quitCh            chan bool
}

func NewHandler(l utils.Logger) *BlockHandler {
	h := &BlockHandler{
		logger:            l,
		subscribers:       make(map[subscriber]bool),
		newSubscriptions:  make(chan subscriber, 128),
		deadSubscriptions: make(chan subscriber, 128),
		blockCh:           make(chan *blocktx_api.Block),
	}

	go func() {
	OUT:
		for {
			select {
			case <-h.quitCh:
				break OUT

			case s := <-h.newSubscriptions:
				h.subscribers[s] = true
				h.logger.Infof("NewHandler MinedTransactions subscription received (Total=%d).", len(h.subscribers))
				// go func() {
				// 	TODO - send all the transactions that were mined since the last time the client was connected
				// }()

			case s := <-h.deadSubscriptions:
				delete(h.subscribers, s)
				h.logger.Infof("BlockNotification subscription removed (Total=%d).", len(h.subscribers))

			case block := <-h.blockCh:
				for sub := range h.subscribers {
					go func(s subscriber) {
						if err := s.stream.Send(block); err != nil {
							h.logger.Errorf("Error sending config")
							h.deadSubscriptions <- s
						}
					}(sub)
				}
			}
		}
	}()

	return h
}

// Shutdown stops the handler
func (h *BlockHandler) Shutdown() {
	h.quitCh <- true
}

// NewSubscription adds a new subscription to the handler
func (h *BlockHandler) NewSubscription(heightAndSource *blocktx_api.Height, s blocktx_api.BlockTxAPI_GetBlockNotificationStreamServer) {
	h.newSubscriptions <- subscriber{
		height: heightAndSource.Height,
		stream: s,
	}

	// Keep this subscription alive without endless loop - use a channel that blocks forever.
	ch := make(chan bool)
	for {
		<-ch
	}
}

func (h *BlockHandler) SendBlock(blockHash []byte, blockHeight uint64, txHash []byte) {
	block := &blocktx_api.Block{
		Hash:   blockHash,
		Height: blockHeight,
	}

	h.blockCh <- block
}
