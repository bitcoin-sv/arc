package metamorph

import (
	"os"
	"time"

	"github.com/ordishs/go-utils"
)

func (p *Processor) GetStats() *ProcessorStats {
	p.tx2ChMap.PrintItems()

	return &ProcessorStats{
		StartTime:       p.startTime,
		UptimeMillis:    time.Since(p.startTime).Milliseconds(),
		WorkerCount:     p.workerCount,
		QueueLength:     p.queueLength.Load(),
		QueuedCount:     p.queuedCount.Load(),
		ProcessedCount:  p.processedCount.Load(),
		ProcessedMillis: p.processedMillis.Load(),
		ChannelMapSize:  int32(p.tx2ChMap.Len()),
	}
}

func (p *Processor) PrintStatsOnKeypress() func() {
	// The following util sets the terminal to non-canonical mode so that we can read
	// single characters from the terminal without having to press enter.
	ttyState := utils.DisableCanonicalMode(p.logger)

	// Print stats when the user presses a key...
	go func() {
		var b = make([]byte, 1)
		for {
			_, _ = os.Stdin.Read(b)

			stats := p.GetStats()

			avg := 0.0
			if stats.ProcessedCount > 0 {
				avg = float64(stats.ProcessedMillis) / float64(stats.ProcessedCount)
			}

			p.logger.Infof(`Peer stats (started: %s):
------------------------
Workers:   %5d
Uptime:    %5.2f s
Queued:    %5d
Processed: %5d
Waiting:   %5d
Average:   %5.2f ms
MapSize:   %5d
------------------------
`,
				stats.StartTime.UTC().Format(time.RFC3339),
				stats.WorkerCount,
				float64(stats.UptimeMillis)/1000.0,
				stats.QueuedCount,
				stats.ProcessedCount,
				stats.QueueLength,
				avg,
				stats.ChannelMapSize,
			)
		}
	}()

	return func() {
		utils.RestoreTTY(ttyState)
	}
}
