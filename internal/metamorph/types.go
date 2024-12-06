package metamorph

import (
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorRequest struct {
	Data            *store.Data
	ResponseChannel chan StatusAndError
}

type StatusAndError struct {
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Err          error
	CompetingTxs []string
}
