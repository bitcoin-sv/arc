package blocktx

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx/store"

	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"

	pb "github.com/TAAL-GmbH/arc/blocktx_api"
)

type Processor struct {
	store   store.Interface
	bitcoin *bitcoin.Bitcoind
	logger  *gocore.Logger
	ch      chan string
	Mtb     *MinedTransactionHandler
}

func NewProcessor(storeI store.Interface, mtb *MinedTransactionHandler) (*Processor, error) {
	host, _ := gocore.Config().Get("bitcoinHost", "localhost")
	port, _ := gocore.Config().GetInt("rpcPort", 8332)
	username, _ := gocore.Config().Get("rpcUsername", "bitcoin")
	password, _ := gocore.Config().Get("rpcPassword", "bitcoin")

	b, err := bitcoin.New(host, port, username, password, false)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to bitcoin: %w", err)
	}

	p := &Processor{
		store:   storeI,
		bitcoin: b,
		logger:  gocore.Log("processor"),
		ch:      make(chan string, 10),
		Mtb:     mtb,
	}

	go func() {
		for blockHash := range p.ch {
			if err := p.processBlock(blockHash); err != nil {
				p.logger.Errorf("Error processing block %s: %v", blockHash, err)
			}
		}
	}()

	return p, nil
}

func (p *Processor) GetBlockHashForHeight(height int) (string, error) {
	hash, err := p.bitcoin.GetBlockHash(height)
	if err != nil {
		return "", fmt.Errorf("Could not get block hash for height %d: %w", height, err)
	}
	return hash, err
}

func (p *Processor) ProcessBlock(hashStr string) {
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

	block := &pb.Block{
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

	var transactions []*pb.Transaction

	for _, txid := range blockJson.Tx {
		txHash, err := hex.DecodeString(txid) // Do not reverse the bytes for storage in database
		if err != nil {
			return err
		}

		// The following line will send the transaction to the MinedTransactionHandler and
		// we need all hashes to be little endian
		p.Mtb.SendTx(utils.ReverseSlice(block.Hash), utils.ReverseSlice(txHash))

		transactions = append(transactions, &pb.Transaction{Hash: txHash})
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
			p.logger.Infof("No blocks in database, starting from genesis")
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
