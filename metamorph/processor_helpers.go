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


	_, _ = io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Metamorph Stats</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
  </head>
  <body>
  <section class="section">
    <div class="container">
      <h1 class="title">
        Metamorph Stats (%s)
      </h1>
      <table class="table is-bordered">
`, stats.UptimeMillis))

        writeStat(w, "Workers", stats.WorkerCount)
	writeStat(w, "Queued", stats.QueuedCount)
	writeStat(w, "Stored", stats.Stored.String())
	writeStat(w, "Announced", stats.AnnouncedToNetwork.String(indent))
	writeStat(w, "SentToNetwork", stats.SentToNetwork.String(indent))
	writeStat(w, "SeenOnNetwork", stats.SeenOnNetwork.String(indent))
	writeStat(w, "Mined", stats.Mined.String())
	writeStat(w, "Rejected", stats.Rejected.String(indent))
	writeStat(w, "Waiting", stats.QueueLength)
	writeStat(w, "MapSize", stats.ChannelMapSize)

	_, _ = io.WriteString(w, `</table>
      </div>
    </section>
  </body>
</html>`)
}

func writeStat(w io.Writer, label string, value interface{}) {
  _, _ = io.WriteString(w, fmt.Sprintf(`<tr><td>%s</td><td><pre class="has-background-white" style="padding: 2px">%v</pre></td></tr>`, label, value))
}
