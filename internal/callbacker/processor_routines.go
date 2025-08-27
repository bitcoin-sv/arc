package callbacker

import (
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

const maxParallelRequests = 10

func CallbackStoreCleanup(p *Processor) {
	n := time.Now()
	midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	olderThan := midnight.Add(-1 * p.clearRetentionPeriod)

	err := p.store.Clear(p.ctx, olderThan)
	if err != nil {
		p.logger.Error("Failed to delete old callbacks in delay", slog.String("err", err.Error()))
	}
}

func sendCallbacks(p *Processor) {
	callbackRecords, err := p.store.GetUnsent(p.ctx, p.batchSize, p.expiration, false)
	if err != nil {
		p.logger.Error("Failed to get many", slog.String("err", err.Error()))
		return
	}

	urlCallbacksMap := map[string][]*store.CallbackData{}
	for _, callbackRecord := range callbackRecords {
		urlCallbacksMap[callbackRecord.URL] = append(urlCallbacksMap[callbackRecord.URL], callbackRecord)
	}

	g, _ := errgroup.WithContext(p.ctx)

	g.SetLimit(maxParallelRequests)

	for url, callbacks := range urlCallbacksMap {
		g.Go(func() error {
			p.sendCallback(url, callbacks)
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		p.logger.Error("Failed send callbacks", slog.String("err", err.Error()))
	}
}

func SendBatchCallbacks(p *Processor) {
	callbackRecords, err := p.store.GetUnsent(p.ctx, p.batchSize, p.expiration, true)
	if err != nil {
		p.logger.Error("Failed to get many", slog.String("err", err.Error()))
		return
	}

	urlCallbacksMap := map[string][]*store.CallbackData{}
	for _, callbackRecord := range callbackRecords {
		urlCallbacksMap[callbackRecord.URL] = append(urlCallbacksMap[callbackRecord.URL], callbackRecord)
	}

	g, _ := errgroup.WithContext(p.ctx)
	g.SetLimit(maxParallelRequests)

	for url, callbacks := range urlCallbacksMap {
		g.Go(func() error {
			p.sendBatchCallback(url, callbacks)
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		p.logger.Error("Failed send callbacks", slog.String("err", err.Error()))
	}
}
