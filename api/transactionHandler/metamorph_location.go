package transactionHandler

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
)

type MetamorphLocationI interface {
	GetServer(ctx context.Context, transaction *blocktx_api.Transaction) (string, error)
	SetServer(ctx context.Context, transaction *blocktx_api.Transaction) error
}

type MetamorphTxLocationService struct {
	blockTx blocktx.ClientI
}

func NewMetamorphTxLocationService(blockTx blocktx.ClientI) MetamorphLocationI {
	return &MetamorphTxLocationService{
		blockTx: blockTx,
	}
}

func (l *MetamorphTxLocationService) GetServer(ctx context.Context, transaction *blocktx_api.Transaction) (string, error) {
	return l.blockTx.LocateTransaction(ctx, transaction)

}

func (l *MetamorphTxLocationService) SetServer(ctx context.Context, transaction *blocktx_api.Transaction) error {
	return l.blockTx.RegisterTransaction(ctx, transaction)
}
