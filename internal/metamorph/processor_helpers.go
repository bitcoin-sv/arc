package metamorph

import (
	"time"
)

func (p *Processor) GetStats(debugItems bool) *ProcessorStats {
	if debugItems {
		p.ProcessorResponseMap.logMapItems(p.logger)
	}

	return &ProcessorStats{
		StartTime:      p.startTime,
		UptimeMillis:   time.Since(p.startTime).String(),
		QueueLength:    p.queueLength.Load(),
		QueuedCount:    p.queuedCount.Load(),
		ChannelMapSize: int32(p.ProcessorResponseMap.Len()),
	}
}
