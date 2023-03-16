package surrealdb

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/TAAL-GmbH/arc/blocktx/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"github.com/surrealdb/surrealdb.go"
)

type SurrealDB struct {
	db *surrealdb.DB
}

func New(url string) (store.Interface, error) {
	db, err := surrealdb.New(url)
	if err != nil {
		return nil, fmt.Errorf("could not connect to surrealdb: %s", err.Error())
	}
	_, err = db.Signin(map[string]interface{}{"user": "root", "pass": "root"})
	if err != nil {
		return nil, err
	}

	_, err = db.Use("blocktx", "blocktx")
	if err != nil {
		return nil, err
	}

	fmt.Println(db.Info())

	s := &SurrealDB{
		db: db,
	}

	return s, nil
}

func (s SurrealDB) RegisterTransaction(_ context.Context, transaction *blocktx_api.TransactionAndSource) (string, []byte, uint64, error) {
	txID := utils.ReverseAndHexEncodeSlice(transaction.Hash)
	if _, err := s.db.Create(
		"transaction:"+txID,
		map[string]any{"source": transaction.Source},
	); err != nil {
		return "", nil, 0, err
	}

	// return source, block hash, block height, error
	return transaction.Source, nil, 0, nil
}

func (s SurrealDB) GetTransactionSource(ctx context.Context, hash *chainhash.Hash) (string, error) {
	return "", nil
}

func (s SurrealDB) GetBlock(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
	return nil, nil
}

func (s SurrealDB) GetBlockForHeight(ctx context.Context, height uint64) (*blocktx_api.Block, error) {
	return nil, nil
}

func (s SurrealDB) GetBlockTransactions(ctx context.Context, block *blocktx_api.Block) (*blocktx_api.Transactions, error) {
	return nil, nil
}

func (s SurrealDB) GetLastProcessedBlock(ctx context.Context) (*blocktx_api.Block, error) {
	return nil, nil
}

func (s SurrealDB) GetTransactionBlock(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Block, error) {
	return nil, nil
}

func (s SurrealDB) GetTransactionBlocks(ctx context.Context, transaction *blocktx_api.Transaction) (*blocktx_api.Blocks, error) {
	return nil, nil
}

func (s SurrealDB) InsertBlock(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
	return 0, nil
}

func (s SurrealDB) InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource) error {
	return nil
}

func (s SurrealDB) MarkBlockAsDone(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
	return nil
}

func (s SurrealDB) OrphanHeight(ctx context.Context, height uint64) error {
	return nil
}

func (s SurrealDB) SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error {
	return nil
}

func (s SurrealDB) GetMinedTransactionsForBlock(ctx context.Context, blockAndSource *blocktx_api.BlockAndSource) (*blocktx_api.MinedTransactions, error) {
	return nil, nil
}

func (s SurrealDB) Close() error {
	s.db.Close()
	return nil
}
