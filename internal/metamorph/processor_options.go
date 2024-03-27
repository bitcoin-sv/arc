package metamorph

import (
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

func WithCacheExpiryTime(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.mapExpiryTime = d
	}
}

func WithProcessorLogger(l *slog.Logger) func(*Processor) {
	return func(p *Processor) {
		p.logger = l
	}
}

func WithNow(nowFunc func() time.Time) func(*Processor) {
	return func(p *Processor) {
		p.now = nowFunc
	}
}

func WithProcessExpiredTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processExpiredTxsInterval = d
	}
}

func WithLockTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.lockTransactionsInterval = d
	}
}

func WithProcessStatusUpdatesInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processStatusUpdatesInterval = d
	}
}

func WithProcessStatusUpdatesBatchSize(size int) func(*Processor) {
	return func(p *Processor) {
		p.processStatusUpdatesBatchSize = size
		p.storageStatusUpdateCh = make(chan store.UpdateStatus, size)
	}
}

func WithDataRetentionPeriod(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.dataRetentionPeriod = d
	}
}

func WithMessageQueueClient(mqClient MessageQueueClient) func(processor *Processor) {
	return func(p *Processor) {
		p.mqClient = mqClient
	}
}

func WithMinedTxsChan(minedTxsChan chan *blocktx_api.TransactionBlocks) func(processor *Processor) {
	return func(p *Processor) {
		p.minedTxsChan = minedTxsChan
	}
}

func WithHttpClient(httpClient HttpClient) func(processor *Processor) {
	return func(p *Processor) {
		p.httpClient = httpClient
	}
}
