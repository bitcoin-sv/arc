package transactionHandler

import (
	"context"

	"github.com/TAAL-GmbH/arc/blocktx"
)

type MetamorphLocationI interface {
	GetServer(ctx context.Context, txID string) (string, error)
	SetServer(ctx context.Context, txID, server string) error
}

type MetamorphTxLocationService struct {
	blockTx blocktx.ClientI
}

func NewMetamorphTxLocationService(blockTx blocktx.ClientI) MetamorphLocationI {
	return &MetamorphTxLocationService{
		blockTx: blockTx,
	}
}

func (l *MetamorphTxLocationService) GetServer(ctx context.Context, txID string) (string, error) {
	return l.blockTx.GetTx(ctx, txID)
}

func (l *MetamorphTxLocationService) SetServer(ctx context.Context, txID, server string) error {
	return l.blockTx.SetTx(ctx, txID, server)
}
