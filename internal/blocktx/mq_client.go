package blocktx

import (
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

type MessageQueueClient interface {
	SubscribeRegisterTxs() error
	SubscribeRequestTxs() error
	PublishMinedTxs(txsBlocks []*blocktx_api.TransactionBlock) error
	Shutdown() error
}
