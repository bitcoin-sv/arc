package blocktxstore

import (
	"context"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
)

// Todo: Remove after message queue has been implemented

type Publisher struct {
	btxStore BlocktxStore
}

type BlocktxStore interface {
	RegisterTransaction(ctx context.Context, transaction *blocktx_api.TransactionAndSource) error
}

func NewBlocktxStorePublisher(btxStore BlocktxStore) *Publisher {
	return &Publisher{btxStore: btxStore}
}

func (p Publisher) PublishTransaction(ctx context.Context, hash []byte) error {
	return p.btxStore.RegisterTransaction(ctx, &blocktx_api.TransactionAndSource{
		Hash: hash,
	})
}
