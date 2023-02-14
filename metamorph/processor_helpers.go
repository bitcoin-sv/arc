package metamorph

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	gcutils "github.com/ordishs/gocore/utils"
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

type statResponse struct {
	Txid                  string                 `json:"hash"`
	Start                 time.Time              `json:"start"`
	Retries               uint32                 `json:"retries"`
	Err                   error                  `json:"error"`
	AnnouncedPeers        []string               `json:"announcedPeers"`
	Status                metamorph_api.Status   `json:"status"`
	NoStats               bool                   `json:"noStats"`
	LastStatusUpdateNanos int64                  `json:"lastStatusUpdateNanos"`
	Log                   []ProcessorResponseLog `json:"log"`
}

func (p *Processor) HandleStats(w http.ResponseWriter, r *http.Request) {
	// add cache headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	txid := r.URL.Query().Get("tx")
	if txid != "" {
		p.writeTransaction(w, txid)
		return
	}

	printTxs := r.URL.Query().Get("printTxs") == "true"
	var txids strings.Builder
	if printTxs {
		items := p.processorResponseMap.Items()
		processorResponses := make([]*ProcessorResponse, 0, len(items))
		for _, item := range items {
			processorResponses = append(processorResponses, item)
		}

		sort.Slice(processorResponses, func(i, j int) bool {
			return processorResponses[i].Start.Before(processorResponses[j].Start)
		})

		txids.WriteString(`<br/><h2 class="subtitle">Transactions</h2>`)

		if len(processorResponses) > 0 {
			txids.WriteString("<div>")
			txids.WriteString(`<table class="table is-bordered">`)
			txids.WriteString(`<tr><th>Start</th><th>Retries</th><th>Status</th><th>Tx ID</th></tr>`)
			for _, processorResponse := range processorResponses {
				txids.WriteString(`<tr>`)

				txid := utils.HexEncodeAndReverseBytes(processorResponse.Hash)

				txids.WriteString(fmt.Sprintf(`<td>%s</td>`, processorResponse.Start.UTC().Format(time.RFC3339Nano)))
				txids.WriteString(fmt.Sprintf(`<td>%d</td>`, processorResponse.retries.Load()))
				txids.WriteString(fmt.Sprintf(`<td>%s</td>`, processorResponse.status.String()))
				txids.WriteString(fmt.Sprintf(`<td><a href="/pstats?tx=%s">%s</a></td>`, txid, txid))

				txids.WriteString(`</tr>`)
			}
			txids.WriteString(`</table>`)
			txids.WriteString("</div>")
		} else {
			txids.WriteString("<div><br/>No transactions found</div>")
		}
	}

	stats := p.GetStats()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	indent := ""

	printLink := `<a href="/pstats?printTxs=true">Print Transactions</a>`
	if printTxs {
		printLink = `<a href="/pstats">Hide Transactions</a>`
	}

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
        Metamorph Stats
      </h1>
      <div>
		%s
      </div>
      <table class="table is-bordered">
`, printLink))

	writeStat(w, "Uptime", stats.UptimeMillis)
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

	_, _ = io.WriteString(w, fmt.Sprintf(`</table>
      </div>
			%s
</section>

  </body>
</html>`, txids.String()))
}

func (p *Processor) writeTransaction(w http.ResponseWriter, txid string) {
	prm, found := p.processorResponseMap.Get(txid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	announcedPeers := make([]string, 0, len(prm.announcedPeers))
	for _, peer := range prm.announcedPeers {
		announcedPeers = append(announcedPeers, peer.String())
	}

	res := &statResponse{
		Txid:                  utils.HexEncodeAndReverseBytes(prm.Hash),
		Start:                 prm.Start,
		Retries:               prm.retries.Load(),
		Err:                   prm.err,
		AnnouncedPeers:        announcedPeers,
		Status:                prm.status,
		NoStats:               prm.noStats,
		LastStatusUpdateNanos: prm.lastStatusUpdateNanos.Load(),
		Log:                   prm.Log,
	}

	txJson, err := json.MarshalIndent(&res, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resLog := strings.Builder{}
	resLog.WriteString("<table class=table>")
	resLog.WriteString("<tr><th>Δ𝑡</th><th>Status</th><th>Source</th><th>Info</th></tr>")
	for _, logEntry := range res.Log {
		resLog.WriteString("<tr>")

		resLog.WriteString(fmt.Sprintf("<td>%s</td>", gcutils.HumanTimeUnitHTML(time.Duration(logEntry.DeltaT))))
		resLog.WriteString(fmt.Sprintf("<td>%s</td>", logEntry.Status))
		resLog.WriteString(fmt.Sprintf("<td>%s</td>", logEntry.Source))
		resLog.WriteString(fmt.Sprintf("<td>%s</td>", logEntry.Info))

		resLog.WriteString("</tr>")
	}
	resLog.WriteString("</table>")

	_, _ = io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Metamorph Transaction %s</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
  </head>
  <body>
  <section class="section">
    <div class="container">
      <h1 class="title">
        Transaction 
      </h1>
      <h2 class="subtitle">
        <a href="https://whatsonchain.com/tx/%s" target="_blank">%s</a>
      </h2>
	  <a href="javascript:history.back()">Back to overview</a>
      <table class="table is-bordered">
`, txid, txid, txid))

	writeStat(w, "Hash", res.Txid)
	writeStat(w, "Start", res.Start.UTC())
	writeStat(w, "Retries", res.Retries)
	writeStat(w, "Err", res.Err)
	writeStat(w, "AnnouncedPeers", res.AnnouncedPeers)
	writeStat(w, "Status", res.Status.String())
	writeStat(w, "NoStats", res.NoStats)
	writeStat(w, "LastStatusUpdateNanos", res.LastStatusUpdateNanos)
	writeStat(w, "Log", resLog.String())

	_, _ = io.WriteString(w, fmt.Sprintf(`</table>
      </div>
	  <pre>%s</pre>
  </section>

  </body>
  </html>`, txJson))
}

func (p *Processor) _(w http.ResponseWriter, txid string) {
	prm, found := p.processorResponseMap.Get(txid)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	announcedPeers := make([]string, 0, len(prm.announcedPeers))
	for _, peer := range prm.announcedPeers {
		announcedPeers = append(announcedPeers, peer.String())
	}

	res := &statResponse{
		Txid:                  utils.HexEncodeAndReverseBytes(prm.Hash),
		Start:                 prm.Start,
		Retries:               prm.retries.Load(),
		Err:                   prm.err,
		AnnouncedPeers:        announcedPeers,
		Status:                prm.status,
		NoStats:               prm.noStats,
		LastStatusUpdateNanos: prm.lastStatusUpdateNanos.Load(),
		Log:                   prm.Log,
	}

	payload, err := json.MarshalIndent(&res, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(payload)
}

func writeStat(w io.Writer, label string, value interface{}) {
	_, _ = io.WriteString(w, fmt.Sprintf(`<tr><td>%s</td><td><pre class="has-background-white" style="padding: 2px">%v</pre></td></tr>`, label, value))
}
