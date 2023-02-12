package metamorph

import (
	"fmt"
	"io"
	"log"
	"net/http"
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
		StartTime:          p.startTime,
		UptimeMillis:       time.Since(p.startTime).String(),
		WorkerCount:        p.workerCount,
		QueueLength:        p.queueLength.Load(),
		QueuedCount:        p.queuedCount.Load(),
		Stored:             p.stored,
		AnnouncedToNetwork: p.announcedToNetwork,
		SentToNetwork:      p.sentToNetwork,
		SeenOnNetwork:      p.seenOnNetwork,
		Rejected:           p.rejected,
		Mined:              p.mined,
		ChannelMapSize:     int32(p.processorResponseMap.Len()),
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

			indent := "               "

			log.Printf(`Peer stats (started: %s):
-----------------------------------------------------
Workers:       %d
Uptime:        %s
Queued:        %d
Stored:        %s
Announced:     %s
SentToNetwork: %s
SeenOnNetwork: %s
Mined:         %s
Rejected:      %s
Waiting:       %d
MapSize:       %d
-----------------------------------------------------
`,

				stats.StartTime.UTC().Format(time.RFC3339),
				stats.WorkerCount,
				stats.UptimeMillis,
				stats.QueuedCount,
				stats.Stored.String(),
				stats.AnnouncedToNetwork.String(indent),
				stats.SentToNetwork.String(indent),
				stats.SeenOnNetwork.String(indent),
				stats.Mined.String(),
				stats.Rejected.String(indent),
				stats.QueueLength,
				stats.ChannelMapSize,
			)
		}
	}()

	return func() {
		utils.RestoreTTY(ttyState)
	}
}

func (p *Processor) HandleStats(w http.ResponseWriter, r *http.Request) {
	stats := p.GetStats()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	indent := ""

	_, _ = io.WriteString(w, "<html>")
	_, _ = io.WriteString(w, "<body>")
	_, _ = io.WriteString(w, "<table>")

	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", "Workers", stats.WorkerCount))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "Uptime", stats.UptimeMillis))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", "Queued", stats.QueuedCount))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "Stored", stats.Stored.String()))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "Announced", stats.AnnouncedToNetwork.String(indent)))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "SentToNetwork", stats.SentToNetwork.String(indent)))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "SeenOnNetwork", stats.SeenOnNetwork.String(indent)))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "Mined", stats.Mined.String()))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", "Rejected", stats.Rejected.String(indent)))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", "Waiting", stats.QueueLength))
	_, _ = io.WriteString(w, fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", "MapSize", stats.ChannelMapSize))

	_, _ = io.WriteString(w, "</table>")
	_, _ = io.WriteString(w, "</body>")
	_, _ = io.WriteString(w, "</html>")
}
