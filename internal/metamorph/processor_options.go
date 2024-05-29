package metamorph

import (
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/blocktx/blocktx_api"
)

func WithStatTimeLimits(notSeenLimit time.Duration, notMinedLimit time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.stats = newProcessorStats(WithLimits(notSeenLimit, notMinedLimit))
	}
}

func WithSeenOnNetworkTxTime(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.seenOnNetworkTxTime = d
	}
}

func WithSeenOnNetworkTxTimeUntil(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.seenOnNetworkTxTimeUntil = d
	}
}

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

func WithMaxRetries(maxRetries int) func(*Processor) {
	return func(p *Processor) {
		p.maxRetries = maxRetries
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

func WithCallbackSender(callbackSender CallbackSender) func(processor *Processor) {
	return func(p *Processor) {
		p.callbackSender = callbackSender
	}
}

func WithStatCollectionInterval(statCollectionInterval time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.statCollectionInterval = statCollectionInterval
	}
}

func WithMinimumHealthyConnections(minimumHealthyConnections int) func(*Processor) {
	return func(p *Processor) {
		p.minimumHealthyConnections = minimumHealthyConnections
	}
}

func WithMonitorPeersInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.monitorPeersInterval = d
	}
}
