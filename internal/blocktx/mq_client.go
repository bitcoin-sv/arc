package blocktx

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

type MessageQueueClient interface {
	SubscribeRegisterTxs() error
	SubscribeRequestTxs() error
	PublishMinedTxs(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) error
	Shutdown() error
}
