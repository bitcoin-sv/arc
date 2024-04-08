package metamorph

import (
	"time"
)

func (p *Processor) GetStats(debugItems bool) *ProcessorStats {
	if debugItems {
		p.ProcessorResponseMap.logMapItems(p.logger)
	}

	return &ProcessorStats{
		StartTime:           p.startTime,
		UptimeMillis:        time.Since(p.startTime).String(),
		QueueLength:         p.queueLength.Load(),
		QueuedCount:         p.queuedCount.Load(),
		Stored:              p.stored,
		AnnouncedToNetwork:  p.announcedToNetwork,
		RequestedByNetwork:  p.requestedByNetwork,
		SentToNetwork:       p.sentToNetwork,
		AcceptedByNetwork:   p.acceptedByNetwork,
		SeenInOrphanMempool: p.seenInOrphanMempool,
		SeenOnNetwork:       p.seenOnNetwork,
		Rejected:            p.rejected,
		Mined:               p.mined,
		Retries:             p.retries,
		ChannelMapSize:      int32(p.ProcessorResponseMap.Len()),
	}
}
