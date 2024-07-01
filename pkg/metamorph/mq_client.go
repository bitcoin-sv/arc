package metamorph

import "github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"

type MessageQueueClient interface {
	PublishSubmitTx(tx *metamorph_api.TransactionRequest) error
	PublishSubmitTxs(txs *metamorph_api.TransactionRequests) error
	Shutdown() error
}
