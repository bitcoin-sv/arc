package metamorph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/ordishs/go-utils"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	CallbackTries           = 5
	CallbackIntervalSeconds = 5
)

type CallbackerStats struct {
	callbackSeenOnNetworkCount       prometheus.Gauge
	callbackSeenInOrphanMempoolCount prometheus.Gauge
	callbackRejectedCount            prometheus.Gauge
	callbackMinedCount               prometheus.Gauge
	callbackFailedCount              prometheus.Gauge
}

type Callback struct {
	BlockHash   *string   `json:"blockHash,omitempty"`
	BlockHeight *uint64   `json:"blockHeight,omitempty"`
	ExtraInfo   *string   `json:"extraInfo"`
	MerklePath  *string   `json:"merklePath"`
	Timestamp   time.Time `json:"timestamp"`
	TxStatus    *string   `json:"txStatus,omitempty"`
	Txid        string    `json:"txid"`
}

type Callbacker struct {
	httpClient      HttpClient
	wg              sync.WaitGroup
	mu              sync.Mutex
	disposed        bool
	callbackerStats *CallbackerStats
}

func NewCallbacker(httpClient HttpClient) (*Callbacker, error) {
	callbacker := &Callbacker{
		httpClient: httpClient,
		callbackerStats: &CallbackerStats{
			callbackSeenOnNetworkCount: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "arc_callback_seen_on_network_count",
				Help: "Number of arc_callback_seen_on_network_count transactions",
			}),
			callbackSeenInOrphanMempoolCount: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "arc_callback_seen_in_orphan_mempool_count",
				Help: "Number of arc_callback_seen_in_orphan_mempool_count transactions",
			}),
			callbackRejectedCount: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "arc_callback_rejected_count",
				Help: "Number of arc_callback_rejected_count transactions",
			}),
			callbackMinedCount: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "arc_callback_mined_count",
				Help: "Number of arc_callback_mined_count transactions",
			}),
			callbackFailedCount: prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "arc_callback_failed_count",
				Help: "Number of arc_callback_failed_count transactions",
			}),
		},
	}

	err := registerStats(
		callbacker.callbackerStats.callbackSeenOnNetworkCount,
		callbacker.callbackerStats.callbackSeenInOrphanMempoolCount,
		callbacker.callbackerStats.callbackRejectedCount,
		callbacker.callbackerStats.callbackMinedCount,
		callbacker.callbackerStats.callbackFailedCount,
	)
	if err != nil {
		return nil, err
	}

	return callbacker, nil
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func (p *Callbacker) SendCallback(logger *slog.Logger, tx *store.StoreData) {
	p.mu.Lock()
	if p.disposed {
		logger.Error("cannot send callback, callbacker is disposed already")
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	p.wg.Add(1)
	defer p.wg.Done()

	sleepDuration := CallbackIntervalSeconds
	statusString := tx.Status.String()
	blockHash := ""
	if tx.BlockHash != nil {
		blockHash = utils.ReverseAndHexEncodeSlice(tx.BlockHash.CloneBytes())
	}

	status := &Callback{
		BlockHash:   &blockHash,
		BlockHeight: &tx.BlockHeight,
		TxStatus:    &statusString,
		Txid:        tx.Hash.String(),
		Timestamp:   time.Now(),
		MerklePath:  &tx.MerklePath,
	}

	if tx.RejectReason != "" {
		status.ExtraInfo = &tx.RejectReason
	}

	for _, callback := range tx.Callbacks {
		p.sendCallback(logger, tx, callback, status, sleepDuration)
	}
}

func (p *Callbacker) sendCallback(logger *slog.Logger, tx *store.StoreData, callback store.StoreCallback, status *Callback, sleepDuration int) {
	for i := 0; i < CallbackTries; i++ {
		logger.Debug("Sending callback for transaction", slog.String("hash", tx.Hash.String()), slog.String("url", callback.CallbackURL), slog.String("token", callback.CallbackToken), slog.String("status", *status.TxStatus), slog.Uint64("block height", tx.BlockHeight), slog.String("block hash", *status.BlockHash))

		statusBytes, err := json.Marshal(status)
		if err != nil {
			logger.Error("Couldn't marshal status", slog.String("err", err.Error()))
			return
		}

		var request *http.Request
		request, err = http.NewRequest("POST", callback.CallbackURL, bytes.NewBuffer(statusBytes))
		if err != nil {
			logger.Error("Couldn't marshal status", slog.String("url", callback.CallbackURL), slog.String("token", callback.CallbackToken), slog.String("hash", tx.Hash.String()), slog.String("err", errors.Join(err, fmt.Errorf("failed to post callback for transaction id %s", tx.Hash)).Error()))
			return
		}
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		if callback.CallbackToken != "" {
			request.Header.Set("Authorization", "Bearer "+callback.CallbackToken)
		}

		var response *http.Response
		response, err = p.httpClient.Do(request)
		if err != nil {
			logger.Debug("Couldn't send transaction callback", slog.String("url", callback.CallbackURL), slog.String("token", callback.CallbackToken), slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
			continue
		}

		defer func() {
			_ = response.Body.Close()
		}()

		// if callback was sent successfully we stop here
		if response.StatusCode == http.StatusOK {
			switch tx.Status {
			case metamorph_api.Status_SEEN_ON_NETWORK:
				p.callbackerStats.callbackSeenOnNetworkCount.Inc()
			case metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL:
				p.callbackerStats.callbackSeenInOrphanMempoolCount.Inc()
			case metamorph_api.Status_MINED:
				p.callbackerStats.callbackMinedCount.Inc()
			case metamorph_api.Status_REJECTED:
				p.callbackerStats.callbackRejectedCount.Inc()
			}
			return
		}

		logger.Debug("Callback response status code not ok", slog.String("url", callback.CallbackURL), slog.String("token", callback.CallbackToken), slog.String("hash", tx.Hash.String()), slog.Int("status", response.StatusCode))

		// sleep before trying again
		time.Sleep(time.Duration(sleepDuration) * time.Second)
		// increase intervals on each failure
		sleepDuration *= 2
	}

	p.callbackerStats.callbackFailedCount.Inc()
	logger.Warn("Couldn't send transaction callback after tries", slog.String("url", callback.CallbackURL), slog.String("token", callback.CallbackToken), slog.String("hash", tx.Hash.String()), slog.Int("retries", CallbackTries))
}

func (p *Callbacker) Shutdown(logger *slog.Logger) {
	p.mu.Lock()
	if p.disposed {
		logger.Info("callbacker is down already")
		p.mu.Unlock()
		return
	}

	unregisterStats(
		p.callbackerStats.callbackSeenOnNetworkCount,
		p.callbackerStats.callbackSeenInOrphanMempoolCount,
		p.callbackerStats.callbackRejectedCount,
		p.callbackerStats.callbackMinedCount,
		p.callbackerStats.callbackFailedCount,
	)

	p.disposed = true
	p.mu.Unlock()

	logger.Info("Shutting down callbacker")
	p.wg.Wait()
}
