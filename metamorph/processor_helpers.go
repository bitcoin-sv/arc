package metamorph

import (
	"log"
	"os"
	"time"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

func (p *Processor) GetStats() *ProcessorStats {
	if p.logger.LogLevel() == int(gocore.DEBUG) {
		p.processorResponseMap.PrintItems()
	}

	return &ProcessorStats{
		StartTime:                p.startTime,
		UptimeMillis:             time.Since(p.startTime).Milliseconds(),
		WorkerCount:              p.workerCount,
		QueueLength:              p.queueLength.Load(),
		QueuedCount:              p.queuedCount.Load(),
		StoredCount:              p.storedCount.Load(),
		StoredMillis:             p.storedMillis.Load(),
		AnnouncedToNetworkCount:  p.announcedToNetworkCount.Load(),
		AnnouncedToNetworkMillis: p.announcedToNetworkMillis.Load(),
		SentToNetworkCount:       p.sentToNetworkCount.Load(),
		SentToNetworkMillis:      p.sentToNetworkMillis.Load(),
		SeenOnNetworkCount:       p.seenOnNetworkCount.Load(),
		SeenOnNetworkMillis:      p.seenOnNetworkMillis.Load(),
		MinedCount:               p.minedCount.Load(),
		MinedMillis:              p.minedMillis.Load(),
		RejectedCount:            p.rejectedCount.Load(),
		RejectedMillis:           p.rejectedMillis.Load(),
		ChannelMapSize:           int32(p.processorResponseMap.Len()),
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

			storedAvg := 0.0
			if stats.StoredCount > 0 {
				storedAvg = float64(stats.StoredMillis) / float64(stats.StoredCount)
			}

			announcedAvg := 0.0
			if stats.AnnouncedToNetworkCount > 0 {
				announcedAvg = float64(stats.AnnouncedToNetworkMillis)/float64(stats.AnnouncedToNetworkCount) - storedAvg
			}

			sentToNetworkAvg := 0.0
			if stats.SentToNetworkCount > 0 {
				sentToNetworkAvg = float64(stats.SentToNetworkMillis)/float64(stats.SentToNetworkCount) - (storedAvg + announcedAvg)
			}

			seenOnNetworkAvg := 0.0
			if stats.SeenOnNetworkCount > 0 {
				seenOnNetworkAvg = float64(stats.SeenOnNetworkMillis)/float64(stats.SeenOnNetworkCount) - (storedAvg + announcedAvg + sentToNetworkAvg)
			}

			minedAvg := "0"
			if stats.MinedCount > 0 {
				avg := int64(float64(stats.MinedMillis)/float64(stats.MinedCount) - (storedAvg + announcedAvg + sentToNetworkAvg))
				minedAvg = (time.Duration(avg) * time.Millisecond).String()
			}

			rejectedAvg := 0.0
			if stats.RejectedCount > 0 {
				rejectedAvg = float64(stats.RejectedMillis)/float64(stats.RejectedCount) - (storedAvg + announcedAvg)
			}

			log.Printf(`Peer stats (started: %s):
------------------------
Workers:          %5d
Uptime:           %5.2f s
Queued:           %5d
Stored:           %5d
StoredAvg:        %5.2f ms
Announced:        %5d
AnnouncedAvg:     %5.2f ms
SentToNetwork:    %5d
SentToNetworkAvg: %5.2f ms
SeenOnNetwork:    %5d
SeenOnNetworkAvg: %5.2f ms
Mined:            %5d
MinedAvg:         %5s
Rejected:         %5d
RejectedAvg:      %5.2f ms
Waiting:          %5d
MapSize:          %5d
------------------------
`,

				stats.StartTime.UTC().Format(time.RFC3339),
				stats.WorkerCount,
				float64(stats.UptimeMillis)/1000.0,
				stats.QueuedCount,
				stats.StoredCount,
				storedAvg,
				stats.AnnouncedToNetworkCount,
				announcedAvg,
				stats.SentToNetworkCount,
				sentToNetworkAvg,
				stats.SeenOnNetworkCount,
				seenOnNetworkAvg,
				stats.MinedCount,
				minedAvg,
				stats.RejectedCount,
				rejectedAvg,
				stats.QueueLength,
				stats.ChannelMapSize,
			)
		}
	}()

	return func() {
		utils.RestoreTTY(ttyState)
	}
}
