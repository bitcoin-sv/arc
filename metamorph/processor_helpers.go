package metamorph

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
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
	"github.com/TAAL-GmbH/arc/metamorph/processor_response"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	gcutils "github.com/ordishs/gocore/utils"
)

func (p *Processor) GetStats(debugItems bool) *ProcessorStats {
	if debugItems && p.logger.LogLevel() == int(gocore.DEBUG) {
		for i := 0; i < 256; i++ {
			p.processorResponseMap[[1]byte{byte(i)}].PrintItems()
		}
	}

	return &ProcessorStats{
		StartTime:          p.startTime,
		UptimeMillis:       time.Since(p.startTime).String(),
		QueueLength:        p.queueLength.Load(),
		QueuedCount:        p.queuedCount.Load(),
		Stored:             p.stored,
		AnnouncedToNetwork: p.announcedToNetwork,
		RequestedByNetwork: p.requestedByNetwork,
		SentToNetwork:      p.sentToNetwork,
		AcceptedByNetwork:  p.acceptedByNetwork,
		SeenOnNetwork:      p.seenOnNetwork,
		Rejected:           p.rejected,
		Mined:              p.mined,
		Retries:            p.retries,
		ChannelMapSize:     int32(p.Len()),
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

			stats := p.GetStats(true)

			indent := "               "

			log.Printf(`Peer stats (started: %s):
-----------------------------------------------------
Uptime:        %s
Queued:        %d
Stored:        %s
Announced:     %s
Requested:     %s
SentToNetwork: %s
SeenOnNetwork: %s
Mined:         %s
Rejected:      %s
Waiting:       %d
MapSize:       %d
-----------------------------------------------------
`,

				stats.StartTime.UTC().Format(time.RFC3339),
				stats.UptimeMillis,
				stats.QueuedCount,
				stats.Stored.String(),
				stats.AnnouncedToNetwork.String(indent),
				stats.RequestedByNetwork.String(indent),
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
	Txid                  string                                    `json:"hash"`
	Start                 time.Time                                 `json:"start"`
	Retries               uint32                                    `json:"Retries"`
	Err                   error                                     `json:"error"`
	AnnouncedPeers        []string                                  `json:"AnnouncedPeers"`
	Status                metamorph_api.Status                      `json:"Status"`
	NoStats               bool                                      `json:"noStats"`
	LastStatusUpdateNanos int64                                     `json:"LastStatusUpdateNanos"`
	Log                   []processor_response.ProcessorResponseLog `json:"log"`
}

func (p *Processor) HandleStats(w http.ResponseWriter, r *http.Request) {
	// add cache headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	format := r.URL.Query().Get("format")

	txid := r.URL.Query().Get("tx")
	if txid != "" {
		hash, err := chainhash.NewHashFromStr(txid)
		if err != nil {
			return
		}

		p.writeTransaction(w, hash, format)
		return
	}

	printTxs := r.URL.Query().Get("printTxs") == "true"

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		stats := p.GetStats(false)
		if printTxs {
			m := p.items(nil)

			txMap := make(map[string]*processor_response.ProcessorResponse)

			for k, v := range m {
				txMap[k.String()] = v
			}

			err := json.NewEncoder(w).Encode(struct {
				*ProcessorStats
				Txs map[string]*processor_response.ProcessorResponse `json:"txs"`
			}{
				ProcessorStats: stats,
				Txs:            txMap,
			})
			if err != nil {
				p.logger.Errorf("could not encode JSON: %v", err)
			}
		} else {
			_ = json.NewEncoder(w).Encode(stats)
		}
		return
	}

	var txids strings.Builder
	if printTxs {
		items := p.items(nil)
		processorResponses := make([]*processor_response.ProcessorResponse, 0, len(items))
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
			txids.WriteString(`<tr><th>Start</th><th>GetRetries</th><th>Status</th><th>Tx ID</th></tr>`)
			for _, processorResponse := range processorResponses {
				txids.WriteString(`<tr>`)

				txids.WriteString(fmt.Sprintf(`<td>%s</td>`, processorResponse.Start.UTC().Format(time.RFC3339Nano)))
				txids.WriteString(fmt.Sprintf(`<td>%d</td>`, processorResponse.Retries.Load()))
				txids.WriteString(fmt.Sprintf(`<td>%s</td>`, processorResponse.Status.String()))
				txids.WriteString(fmt.Sprintf(`<td><a href="/pstats?tx=%v">%v</a></td>`, processorResponse.Hash, processorResponse.Hash))

				txids.WriteString(`</tr>`)
			}
			txids.WriteString(`</table>`)
			txids.WriteString("</div>")
		} else {
			txids.WriteString("<div><br/>No transactions found</div>")
		}
	}

	stats := p.GetStats(false)

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
	writeStat(w, "Queued", stats.QueuedCount)
	writeStat(w, "Stored", stats.Stored.String())
	writeStat(w, "Announced", stats.AnnouncedToNetwork.String(indent))
	writeStat(w, "Requested", stats.RequestedByNetwork.String(indent))
	writeStat(w, "SentToNetwork", stats.SentToNetwork.String(indent))
	writeStat(w, "Accepted", stats.AcceptedByNetwork.String(indent))
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

func (p *Processor) writeTransaction(w http.ResponseWriter, hash *chainhash.Hash, format string) {
	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		prm, found := p.processorResponseMap[[1]byte{hash[0]}].Get(hash)
		if !found {
			storeData, _ := p.store.Get(context.Background(), hash[:])
			if storeData != nil {
				_ = json.NewEncoder(w).Encode(storeData)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, fmt.Sprintf(`{"error": "tx not found", "txid": "%v"}`, hash))
			return
		}
		_ = json.NewEncoder(w).Encode(prm)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// write header for page
	_, _ = io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Metamorph Transaction %v</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
  </head>
  <body>
  <section class="section">
    <div class="container">
`, hash))

	prm, found := p.processorResponseMap[[1]byte{hash[0]}].Get(hash)
	if !found {
		storeData, _ := p.store.Get(context.Background(), hash[:])
		if storeData != nil {
			b, _ := json.MarshalIndent(storeData, "", "  ")
			_, _ = io.WriteString(w, fmt.Sprintf(`
					<h1 class="title">Transaction (from store)</h1>
					<h2 class="subtitle">
					    <a href="https://whatsonchain.com/tx/%v" target="_blank">%v</a>
					</h2>
					<a href="/pstats?printTxs=true"><< Back</a>
					<table class=table>
						<tr><td>Stored At</td><td>%s</td></tr>
						<tr><td>Announced At</td><td>%s</td></tr>
						<tr><td>Mined At</td><td>%s</td></tr>
						<tr><td>Status</td><td>%s</td></tr>
						<tr><td>Block height</td><td>%d</td></tr>
						<tr><td>Block hash</td><td><a href="https://whatsonchain.com/block/%v" target="_blank">%v</a></td></tr>
						<tr><td>Callback URL</td><td>%s</td></tr>
						<tr><td>Callback token</td><td>%s</td></tr>
						<tr><td>Merkle proof</td><td>%v</td></tr>
						<tr><td>Reject reason</td><td>%s</td></tr>
						<tr><td>TX hex</td><td style="word-break: break-all; width: 75%%;">%s</td></tr>
					</table>
					<pre>%s</pre>
				`, hash, hash,
				storeData.StoredAt.UTC().Format(time.RFC3339Nano),
				storeData.AnnouncedAt.UTC().Format(time.RFC3339Nano),
				storeData.MinedAt.UTC().Format(time.RFC3339Nano),
				storeData.Status.String(),
				storeData.BlockHeight,
				storeData.BlockHash,
				storeData.BlockHash,
				storeData.CallbackUrl,
				storeData.CallbackToken,
				storeData.MerkleProof,
				storeData.RejectReason,
				hex.EncodeToString(storeData.RawTx),
				string(b),
			))

			logFile, _ := gocore.Config().Get("metamorph_logFile") //, "./data/metamorph.log")
			if logFile != "" {
				processorResponseStats := grepFile(logFile, hash.String())
				if processorResponseStats != "" {
					var prStats *processor_response.ProcessorResponse
					_ = json.Unmarshal([]byte(processorResponseStats), &prStats)
					if prStats != nil {
						_, _ = io.WriteString(w, fmt.Sprintf(`
								<hr/>
								<h1 class="title">Transaction stats from log</h1>
								<h2 class="subtitle">logfile: %s</h2>
							`, logFile))
						txJson := p.processorResponseStatsTable(w, prStats)
						_, _ = io.WriteString(w, fmt.Sprintf(`<pre>%s</pre>`, txJson))
					}
				}

			} else {
				// transaction not found, return error and close html
				_, _ = io.WriteString(w, `
					<h1 class="title">Could not find transaction</h1>
					<a href="/pstats?printTxs=true"><< Back</a>
				`)
			}
		}

		_, _ = io.WriteString(w, `</table>
      </div>
  </section>

  </body>
  </html>`)
		return
	}

	_, _ = io.WriteString(w, fmt.Sprintf(`
      <h1 class="title">
        Transaction
      </h1>
      <h2 class="subtitle">
        <a href="https://whatsonchain.com/tx/%v" target="_blank">%v</a>
      </h2>
	  <a href="javascript:history.back()">Back to overview</a>
	`, hash, hash))

	txJson := p.processorResponseStatsTable(w, prm)

	_, _ = io.WriteString(w, fmt.Sprintf(`
      </div>
	  <pre>%s</pre>
  </section>

  </body>
  </html>`, txJson))
}

func (p *Processor) processorResponseStatsTable(w http.ResponseWriter, prm *processor_response.ProcessorResponse) []byte {

	announcedPeers := make([]string, 0, len(prm.AnnouncedPeers))
	for _, peer := range prm.AnnouncedPeers {
		if peer != nil {
			announcedPeers = append(announcedPeers, peer.String())
		}
	}

	res := &statResponse{
		Txid:                  prm.Hash.String(),
		Start:                 prm.Start,
		Retries:               prm.Retries.Load(),
		Err:                   prm.Err,
		AnnouncedPeers:        announcedPeers,
		Status:                prm.Status,
		NoStats:               prm.NoStats,
		LastStatusUpdateNanos: prm.LastStatusUpdateNanos.Load(),
		Log:                   prm.Log,
	}

	resLog := strings.Builder{}

	txJson, err := json.MarshalIndent(&res, "", "  ")
	if err == nil {
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
	}

	_, _ = io.WriteString(w, `
      <table class="table is-bordered">
	`)

	writeStat(w, "Hash", res.Txid)
	writeStat(w, "Start", res.Start.UTC())
	writeStat(w, "GetRetries", res.Retries)
	writeStat(w, "Err", res.Err)
	writeStat(w, "AnnouncedPeers", res.AnnouncedPeers)
	writeStat(w, "Status", res.Status.String())
	writeStat(w, "NoStats", res.NoStats)
	writeStat(w, "LastStatusUpdateNanos", res.LastStatusUpdateNanos)
	writeStat(w, "Log", resLog.String())

	_, _ = io.WriteString(w, `</table>`)

	return txJson
}

func writeStat(w io.Writer, label string, value interface{}) {
	_, _ = io.WriteString(w, fmt.Sprintf(`<tr><td>%s</td><td><pre class="has-background-white" style="padding: 2px">%v</pre></td></tr>`, label, value))
}

func grepFile(file string, grepStr string) string {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	b := []byte(grepStr)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), b) {
			return scanner.Text()
		}
	}

	return ""
}
