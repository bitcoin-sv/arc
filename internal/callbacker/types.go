package callbacker

import (
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorRequest struct {
	Data            *store.CallbackData
	ResponseChannel chan StatusAndError
}

type StatusAndError struct {
	Hash         *chainhash.Hash
	Status       callbacker_api.Status
	Err          error
	CompetingTxs []string
}
