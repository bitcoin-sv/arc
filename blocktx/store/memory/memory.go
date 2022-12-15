package memory

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/TAAL-GmbH/arc/blocktx/store"
	pb "github.com/TAAL-GmbH/arc/blocktx_api"
)

type memBlock struct {
	block        *pb.Block
	height       uint64
	transactions *pb.Transactions
	processed    bool
	orphaned     bool
}

type Store struct {
	mu            sync.RWMutex
	blocks        map[string]*memBlock
	blockByHeight map[uint64]*memBlock
	blockById     map[uint64]*memBlock
}

func New() (store.Interface, error) {
	return &Store{
		blocks:        make(map[string]*memBlock),
		blockByHeight: make(map[uint64]*memBlock),
		blockById:     make(map[uint64]*memBlock),
	}, nil
}

func (s *Store) GetBlockForHeight(_ context.Context, height uint64) (*pb.Block, error) {
	blk, ok := s.blockByHeight[height]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return blk.block, nil
}

func (s *Store) GetBlockTransactions(_ context.Context, block *pb.Block) (*pb.Transactions, error) {
	blk, ok := s.blocks[hex.EncodeToString(block.Hash)]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return blk.transactions, nil
}

func (s *Store) GetLastProcessedBlock(_ context.Context) (*pb.Block, error) {
	// get the last memBlock in the map
	var lastBlock *memBlock
	for _, blk := range s.blocks { // HL
		if lastBlock.processed && (lastBlock == nil || blk.height > lastBlock.height) {
			lastBlock = blk
		}
	}
	if lastBlock == nil {
		return nil, sql.ErrNoRows
	}

	return lastBlock.block, nil
}

func (s *Store) GetTransactionBlock(_ context.Context, transaction *pb.Transaction) (*pb.Block, error) {
	// get the memBlock the transaction is found in, but only if it is not orphaned
	for _, blk := range s.blocks {
		if blk.orphaned {
			continue
		}
		for _, tx := range blk.transactions.Transactions {
			if bytes.Equal(tx.Hash, transaction.Hash) {
				return blk.block, nil
			}
		}
	}

	return nil, sql.ErrNoRows
}

func (s *Store) GetTransactionBlocks(_ context.Context, transaction *pb.Transaction) (*pb.Blocks, error) {
	// get all blocks the transaction is found in
	blocks := &pb.Blocks{}
	blocks.Blocks = make([]*pb.Block, 0)
	for _, blk := range s.blocks {
		for _, tx := range blk.transactions.Transactions {
			if bytes.Equal(tx.Hash, transaction.Hash) {
				blocks.Blocks = append(blocks.Blocks, blk.block)
			}
		}
	}

	if len(blocks.Blocks) == 0 {
		return nil, sql.ErrNoRows
	}

	return blocks, nil
}

func (s *Store) InsertBlock(_ context.Context, block *pb.Block) (uint64, error) {
	// insert new memBlock
	s.mu.Lock()
	defer s.mu.Unlock()

	insertBlock := memBlock{
		block:     block,
		height:    block.Height,
		processed: false,
		orphaned:  false,
	}

	blockID := uint64(len(s.blocks) + 1)

	s.blocks[hex.EncodeToString(block.Hash)] = &insertBlock
	s.blockByHeight[block.Height] = &insertBlock
	s.blockById[blockID] = &insertBlock

	return blockID, nil
}

func (s *Store) InsertBlockTransactions(_ context.Context, blockId uint64, transactions []*pb.Transaction) error {
	// insert transactions for block
	s.mu.Lock()
	defer s.mu.Unlock()

	blk, ok := s.blockById[blockId]
	if !ok {
		return fmt.Errorf("block not found")
	}
	blk.transactions = &pb.Transactions{
		Transactions: transactions,
	}

	return nil
}

func (s *Store) MarkBlockAsDone(_ context.Context, blockId uint64) error {
	// mark block as processed
	s.mu.Lock()
	defer s.mu.Unlock()

	blk, ok := s.blockById[blockId]
	if !ok {
		return fmt.Errorf("block not found")
	}
	blk.processed = true

	return nil
}

func (s *Store) OrphanHeight(_ context.Context, height uint64) error {
	blk, ok := s.blockByHeight[height]
	if !ok {
		// no need to return an error, this is a fire-and-forget function
		return nil
	}
	blk.orphaned = true
	// delete the block from the height map
	delete(s.blockByHeight, height)

	return nil
}

func (s *Store) SetOrphanHeight(_ context.Context, height uint64, orphaned bool) error {
	blk, ok := s.blockByHeight[height]
	if !ok {
		// no need to return an error, this is a fire-and-forget function
		return nil
	}
	blk.orphaned = orphaned
	// delete the block from the height map
	if orphaned {
		delete(s.blockByHeight, height)
	} else {
		// add the block back to the height map
		s.blockByHeight[height] = blk
	}

	return nil
}
