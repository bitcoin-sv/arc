package blocktx

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/store"

	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

type ProcessorBitcoinI interface {
	GetBlock(hash string) (*bitcoin.Block, error)
	GetBlockHeaderHex(hash string) (*string, error)
	GetBlockHash(height int) (string, error)
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
	go p.Catchup()

	zmqHost, _ := gocore.Config().Get("peer_1_host", "localhost")
	zmqPort, _ := gocore.Config().GetInt("peer_1_zmqPort", 28332)

	z := NewZMQ(p, zmqHost, zmqPort)

	z.Start()
}

func (p *Processor) Close() {
	close(p.quitCh)
}

func (p *Processor) GetBlockHashForHeight(height int) (string, error) {
	hash, err := p.bitcoin.GetBlockHash(height)
	if err != nil {
		return "", fmt.Errorf("Could not get block hash for height %d: %w", height, err)
	}
	return hash, err
}

func (p *Processor) ProcessBlock(hashStr string) {
	p.once.Do(func() {
		p.Mtb = NewHandler(p.logger)
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
		p.Mtb.SendTx(reversedBlockHash, utils.ReverseSlice(txHash))

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

func (p *Processor) Catchup() {
	var height int

	block, err := p.store.GetLastProcessedBlock(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			p.logger.Infof("No blocks in database, starting from current block")
			// TODO get current block height
			height = 770000
		} else {
			p.logger.Fatal(err)
		}
	} else {
		height = int(block.Height)
	}

	p.logger.Infof("Starting catchup from height: %d", height)

	for {
		hash, err := p.GetBlockHashForHeight(height)
		if err != nil {
			p.logger.Errorf("Could not get hash for block height %d: %v", height, err)
			break
		}

		if hash == "" {
			p.logger.Infof("No block found for height %d", height)
			break
		}

		p.ProcessBlock(hash)

		height++
	}
}
