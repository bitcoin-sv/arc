package metamorph

import (
	"context"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorRequest struct {
	Ctx             context.Context
	Data            *store.StoreData
	ResponseChannel chan StatusAndError
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
