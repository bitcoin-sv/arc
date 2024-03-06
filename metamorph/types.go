package metamorph

import (
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils/stat"
)

type ProcessorStats struct {
	StartTime           time.Time
	UptimeMillis        string
	QueueLength         int32
	QueuedCount         int32
	Stored              *stat.AtomicStat
	AnnouncedToNetwork  *stat.AtomicStats
	RequestedByNetwork  *stat.AtomicStats
	SentToNetwork       *stat.AtomicStats
	AcceptedByNetwork   *stat.AtomicStats
	SeenOnNetwork       *stat.AtomicStats
	SeenInOrphanMempool *stat.AtomicStats
	Rejected            *stat.AtomicStats
	Mined               *stat.AtomicStat
	Retries             *stat.AtomicStat
	ChannelMapSize      int32
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
