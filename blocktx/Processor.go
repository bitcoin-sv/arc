package blocktx

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/mrz1836/go-logger"

	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

type ProcessorBitcoinI interface {
	GetBlock(hash string) (*bitcoin.Block, error)
	GetBlockHeaderHex(hash string) (*string, error)
	GetBlockHash(height int) (string, error)
	GetBestBlockHash() (string, error)
}

type Processor struct {
	once      sync.Once
	store     store.Interface
	bitcoin   ProcessorBitcoinI
	logger    *gocore.Logger
	ch        chan string
	catchupCh chan string
	quitCh    chan bool
	Mtb       *MinedTransactionHandler
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
	p.Mtb = NewHandler(p.logger)

	pm := p2p.NewPeerManager(nil)

	peerStore := NewBlockTxPeerStore(p.store)

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

	// Force a catchup on startup

	// catchupHash, err := p.GetCatchupBlockHash()
	// if err != nil {
	// 	p.logger.Fatalf("Error getting catchup block hash: %v", err)
	// }

	// pm.GetBlocks(catchupHash)
}

func (p *Processor) Close() {
	close(p.quitCh)
}

func (p *Processor) GetBlockHashForHeight(height int) (string, error) {
	hash, err := p.bitcoin.GetBlockHash(height)
	if err != nil {
		return "", fmt.Errorf("could not get block hash for height %d: %w", height, err)
	}
	return hash, err
}

func (p *Processor) ProcessBlock(hashStr string) {
	p.once.Do(func() {
		p.catchupCh = make(chan string, 10)
		p.ch = make(chan string, 10)

		go func() {
			for {
				select {
				case <-p.quitCh:
					return

				case blockHash := <-p.catchupCh:
					if err := p.processBlock(blockHash); err != nil {
						p.logger.Errorf("Error processing catchup block %s: %v", blockHash, err)
					}

				case blockHash := <-p.ch:
					if err := p.processBlock(blockHash); err != nil {
						p.logger.Errorf("Error processing zmq block %s: %v", blockHash, err)
					}
				}
			}
		}()
	})

	p.ch <- hashStr
}

func (p *Processor) processBlock(hashStr string) error {
	ctx := context.Background()

	start := time.Now()

	blockHeaderHex, err := p.bitcoin.GetBlockHeaderHex(hashStr)
	if err != nil {
		return err
	}

	header, err := hex.DecodeString(*blockHeaderHex) // No NOT reverse the bytes
	if err != nil {
		return err
	}

	blockJson, err := p.bitcoin.GetBlock(hashStr)
	if err != nil {
		return err
	}

	blockHash, err := hex.DecodeString(hashStr) // No not reverse the bytes for storage in database
	if err != nil {
		return err
	}

	block := &blocktx_api.Block{
		Hash:   blockHash,
		Height: blockJson.Height,
		Header: header,
	}

	if err := p.store.OrphanHeight(ctx, blockJson.Height); err != nil {
		return err
	}

	blockId, err := p.store.InsertBlock(ctx, block)
	if err != nil {
		return err
	}

	var transactions []*blocktx_api.Transaction

	reversedBlockHash := utils.ReverseSlice(block.Hash)

	for _, txid := range blockJson.Tx {
		txHash, err := hex.DecodeString(txid) // Do not reverse the bytes for storage in database
		if err != nil {
			return err
		}

		// The following line will send the transaction to the MinedTransactionHandler and
		// we need all hashes to be little endian
		p.Mtb.SendTx(reversedBlockHash, block.Height, utils.ReverseSlice(txHash))

		transactions = append(transactions, &blocktx_api.Transaction{Hash: txHash})
	}

	if err := p.store.InsertBlockTransactions(ctx, blockId, transactions); err != nil {
		return err
	}

	if err := p.store.MarkBlockAsDone(ctx, blockId); err != nil {
		return err
	}

	p.logger.Infof("Processed block height %d (%d txns in %d ms)", block.Height, len(transactions), time.Since(start).Milliseconds())

	return nil
}

func (p *Processor) GetCatchupBlockHash() ([]byte, error) {
	block, err := p.store.GetLastProcessedBlock(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			p.logger.Warnf("No blocks in blocktx, retrieving best block from bitcoin node")
			bestBlockHash, err := p.bitcoin.GetBestBlockHash()
			if err != nil {
				p.logger.Fatal(err)
			}
			block, err := p.bitcoin.GetBlock(bestBlockHash)
			if err != nil {
				p.logger.Fatal(err)
			}

			return utils.DecodeAndReverseHexString(block.Hash)

		} else {
			p.logger.Fatal(err)
		}
	}

	// Rewind 10 blocks
	for i := 0; i < 10; i++ {
		b, err := p.store.GetBlockForHeight(context.Background(), block.Height-1)
		if err != nil {
			if err == sql.ErrNoRows {
				break // block is still pointing to the last iteration
			} else {
				p.logger.Fatal(err)
			}
		}

		block = b
	}

	return block.Hash, nil
}
