package blocktx

import "github.com/bitcoin-sv/arc/blocktx/blocktx_api"

type MessageQueueClient interface {
	SubscribeRegisterTxs() error
	PublishMinedTxs(txsBlocks []*blocktx_api.TransactionBlock) error
	Shutdown() error
}
