package metamorph

import (
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bsv-blockchain/go-bt/v2/chainhash"
)

type ProcessorRequest struct {
	Data            *store.TransactionData
	ResponseChannel chan StatusAndError
}

type StatusAndError struct {
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Err          error
	CompetingTxs []string
}
