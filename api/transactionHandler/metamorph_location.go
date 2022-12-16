package transactionHandler

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx"
	btcpb "github.com/TAAL-GmbH/arc/blocktx/api"
)

type MetamorphLocationI interface {
	GetServer(ctx context.Context, transaction *btcpb.Transaction) (string, error)
	SetServer(ctx context.Context, transaction *btcpb.Transaction) error
}

type MetamorphTxLocationService struct {
	blockTx blocktx.ClientI
}

func NewMetamorphTxLocationService(blockTx blocktx.ClientI) MetamorphLocationI {
	return &MetamorphTxLocationService{
		blockTx: blockTx,
	}
}

func (l *MetamorphTxLocationService) GetServer(ctx context.Context, transaction *btcpb.Transaction) (string, error) {
	return l.blockTx.LocateTransaction(ctx, transaction)

}

func (l *MetamorphTxLocationService) SetServer(ctx context.Context, transaction *btcpb.Transaction) error {
	return l.blockTx.RegisterTransaction(ctx, transaction)
}
