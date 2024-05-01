package metamorph

import (
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type ProcessorStats struct {
	ChannelMapSize int32
}

type ProcessorRequest struct {
	Data            *store.StoreData
	ResponseChannel chan processor_response.StatusAndError
	Timeout         time.Duration
}

type PeerTxMessage struct {
	Start  time.Time
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	Peer   string
	Err    error
}
