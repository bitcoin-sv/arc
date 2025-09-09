package callbacker

import (
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bitcoin-sv/arc/internal/callbacker/store"
)

const maxParallelRoutines = 100

func CallbackStoreCleanup(p *Processor) {
	n := time.Now()
	midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	olderThan := midnight.Add(-1 * p.clearRetentionPeriod)

	err := p.store.Clear(p.ctx, olderThan)
	if err != nil {
		p.logger.Error("Failed to delete old callbacks in delay", slog.String("err", err.Error()))
	}
}

func LoadAndSendSingleCallbacks(p *Processor) {
	LoadAndSendCallbacks(p, false, p.sendSingleCallbacks)
}

func LoadAndSendBatchCallbacks(p *Processor) {
	LoadAndSendCallbacks(p, true, p.sendBatchCallback)
}

type callbackKey struct {
	txID string
	url  string
}

func LoadAndSendCallbacks(p *Processor, isBatch bool, sendFunc func(url string, cbs []*store.CallbackData)) {
	callbackRecords, err := p.store.GetUnsent(p.ctx, p.batchSize, p.expiration, isBatch)
	if err != nil {
		p.logger.Error("Failed to get many", slog.String("err", err.Error()))
		return
	}
	if len(callbackRecords) == 0 {
		return
	}

	hashCallbacksMap := map[callbackKey][]*store.CallbackData{}
	for _, callbackRecord := range callbackRecords {
		key := callbackKey{
			txID: callbackRecord.TxID,
			url:  callbackRecord.URL,
		}
		hashCallbacksMap[key] = append(hashCallbacksMap[key], callbackRecord)
	}

	g, _ := errgroup.WithContext(p.ctx)
	g.SetLimit(maxParallelRoutines)

	for key, callbacks := range hashCallbacksMap {
		if len(callbacks) == 0 {
			continue
		}

		url := key.url

		g.Go(func() error {
			sendFunc(url, callbacks)
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		p.logger.Error("Failed send callbacks", slog.String("err", err.Error()))
	}
}

func (p *Processor) sendSingleCallbacks(url string, cbs []*store.CallbackData) {
	cbIDs := make([]int64, len(cbs))
	for i, cb := range cbs {
		cbIDs[i] = cb.ID
	}
	for _, cb := range cbs {
		cbEntry := toEntry(cb)
		success, retry := p.sender.Send(url, cbEntry.Token, cbEntry.Data)
		if retry || !success {
			err := p.store.UnsetPending(p.ctx, cbIDs)
			if err != nil {
				p.logger.Error("Failed to set not pending", slog.String("err", err.Error()))
			}
			break
		}

		err := p.store.SetSent(p.ctx, []int64{cb.ID})
		if err != nil {
			p.logger.Error("Failed to set sent", slog.String("err", err.Error()))
		}

		time.Sleep(p.singleSendInterval)
	}
}

func (p *Processor) sendBatchCallback(url string, cbs []*store.CallbackData) {
	batch := make([]*Callback, len(cbs))
	cbIDs := make([]int64, len(cbs))
	for i, cb := range cbs {
		batch[i] = toCallback(cb)
		cbIDs[i] = cb.ID
	}
	success, retry := p.sender.SendBatch(url, cbs[0].Token, batch)
	if retry || !success {
		err := p.store.UnsetPending(p.ctx, cbIDs)
		if err != nil {
			p.logger.Error("Failed to set not pending", slog.String("err", err.Error()))
		}
		return
	}

	err := p.store.SetSent(p.ctx, cbIDs)
	if err != nil {
		p.logger.Error("Failed to set sent", slog.String("err", err.Error()))
	}
}
