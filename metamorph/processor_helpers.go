package metamorph

import (
	"fmt"
	"os"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-utils"
)

func (p *Processor) GetStats() *ProcessorStats {
	filterFunc := func(p *ProcessorResponse) bool {
		return p.status < metamorph_api.Status_SEEN_ON_NETWORK
	}

	for _, value := range p.tx2ChMap.Items(filterFunc) {
		fmt.Printf("tx2ChMap: %s\n", value.String())
	}

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
