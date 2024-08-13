package metamorph

import (
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorRequest struct {
	Data            *store.StoreData
	ResponseChannel chan StatusAndError
	Timeout         time.Duration
}

type StatusAndError struct {
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Err          error
	CompetingTxs []string
}

type PeerTxMessage struct {
	Start        time.Time
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Peer         string
	Err          error
	CompetingTxs []string
}
