package blocktx

import (
	"bytes"
	"time"

	pb "github.com/TAAL-GmbH/arc/blocktx_api"

	"github.com/ordishs/go-utils"
	batcher "github.com/ordishs/go-utils/batcher"
)

type blockTx struct {
	blockHash []byte
	txHash    []byte
}

type MinedTransactionHandler struct {
	logger      utils.Logger
	subscribers map[pb.BlockTxAPI_GetMinedBlockTransactionsServer]bool

	newSubscriptions  chan pb.BlockTxAPI_GetMinedBlockTransactionsServer
	deadSubscriptions chan pb.BlockTxAPI_GetMinedBlockTransactionsServer
	mtCh              chan *pb.MinedTransaction
	quitCh            chan bool
	txBatcher         *batcher.Batcher[blockTx]
}

func NewHandler(l utils.Logger) *MinedTransactionHandler {
	h := &MinedTransactionHandler{
		logger:            l,
		subscribers:       make(map[pb.BlockTxAPI_GetMinedBlockTransactionsServer]bool),
		newSubscriptions:  make(chan pb.BlockTxAPI_GetMinedBlockTransactionsServer, 128),
		deadSubscriptions: make(chan pb.BlockTxAPI_GetMinedBlockTransactionsServer, 128),
		mtCh:              make(chan *pb.MinedTransaction),
	}
	h.txBatcher = batcher.New(500, 500*time.Millisecond, h.sendTxBatch, true)

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
				h.logger.Infof("MinedTransaction subscription removed (Total=%d).", len(h.subscribers))

			case mt := <-h.mtCh:
				if len(mt.Txs) == 0 {
					continue
				}

				for sub := range h.subscribers {
					go func(s pb.BlockTxAPI_GetMinedBlockTransactionsServer) {
						if err := s.Send(mt); err != nil {
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
func (h *MinedTransactionHandler) Shutdown() {
	h.quitCh <- true
}

// NewSubscription adds a new subscription to the handler
func (h *MinedTransactionHandler) NewSubscription(s pb.BlockTxAPI_GetMinedBlockTransactionsServer) {
	h.newSubscriptions <- s

	// Keep this subscription alive without endless loop - use a channel that blocks forever.
	ch := make(chan bool)
	for {
		<-ch
	}
}

func (h *MinedTransactionHandler) SendTx(blockHash []byte, txHash []byte) {
	h.txBatcher.Put(&blockTx{
		blockHash: blockHash,
		txHash:    txHash,
	})
}

// sendTxBatch sends a batch of transactions to the subscribers
// The batch is grouped by block hash
func (h *MinedTransactionHandler) sendTxBatch(batch []*blockTx) {
	mt := &pb.MinedTransaction{
		// Txs: make([][]byte, 0),
	}

	for _, btx := range batch {
		if mt.Blockhash == nil || !bytes.Equal(mt.Blockhash, btx.blockHash) {
			h.mtCh <- mt

			mt = &pb.MinedTransaction{
				Blockhash: btx.blockHash,
				Txs:       nil,
			}
		}

		mt.Txs = append(mt.Txs, btx.txHash)
	}

	h.mtCh <- mt
}
