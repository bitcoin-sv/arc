package metamorph

import (
	"time"

	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/ordishs/go-utils/stat"
)

type ProcessorStats struct {
	StartTime          time.Time
	UptimeMillis       string
	QueueLength        int32
	QueuedCount        int32
	Stored             *stat.AtomicStat
	AnnouncedToNetwork *stat.AtomicStats
	RequestedByNetwork *stat.AtomicStats
	SentToNetwork      *stat.AtomicStats
	AcceptedByNetwork  *stat.AtomicStats
	SeenOnNetwork      *stat.AtomicStats
	Rejected           *stat.AtomicStats
	Mined              *stat.AtomicStat
	Retries            *stat.AtomicStat
	ChannelMapSize     int32
}

type ProcessorRequest struct {
	Data            *store.StoreData
	ResponseChannel chan StatusAndError
}
