package store

import (
	"context"

	pb "github.com/TAAL-GmbH/arc/blocktx/api"
)

type Interface interface {
	InsertTransaction(ctx context.Context, transaction *pb.Transaction) error
	GetTransactionSource(ctx context.Context, txid []byte) (string, error)
	GetBlockForHeight(ctx context.Context, height uint64) (*pb.Block, error)
	GetBlockTransactions(ctx context.Context, block *pb.Block) (*pb.Transactions, error)
	GetLastProcessedBlock(ctx context.Context) (*pb.Block, error)
	GetTransactionBlock(ctx context.Context, transaction *pb.Transaction) (*pb.Block, error)
	GetTransactionBlocks(ctx context.Context, transaction *pb.Transaction) (*pb.Blocks, error)
	InsertBlock(ctx context.Context, block *pb.Block) (uint64, error)
	InsertBlockTransactions(ctx context.Context, blockId uint64, transactions []*pb.Transaction) error
	MarkBlockAsDone(ctx context.Context, blockId uint64) error
	OrphanHeight(ctx context.Context, height uint64) error
	SetOrphanHeight(ctx context.Context, height uint64, orphaned bool) error
}
