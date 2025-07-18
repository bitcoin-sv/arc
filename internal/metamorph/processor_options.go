package metamorph

import (
	"log/slog"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/mq"
)

func WithStatTimeLimits(notSeenLimit time.Duration, notFinalLimit time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.stats = newProcessorStats(WithLimits(notSeenLimit, notFinalLimit))
	}
}

func WithReAnnounceSeenLastConfirmedAgo(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.reAnnounceSeenLastConfirmedAgo = d
	}
}

func WithReRegisterSeen(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.reRegisterSeen = d
	}
}

func WithRejectPendingSeenEnabled(rejectPendingSeen bool) func(*Processor) {
	return func(p *Processor) {
		p.rejectPendingSeenEnabled = rejectPendingSeen
	}
}

func WithReAnnounceSeenPendingSince(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.reAnnounceSeenPendingSince = d
	}
}

func WithRejectPendingSeenLastRequestedAgo(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.rejectPendingSeenLastRequestedAgo = d
	}
}

func WithRejectPendingBlocksSince(blocks uint64) func(*Processor) {
	return func(p *Processor) {
		p.rejectPendingBlocksSince = blocks
	}
}

func WithReBroadcastExpiration(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.rebroadcastExpiration = d
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

func WithReAnnounceUnseenInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.reAnnounceUnseenInterval = d
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

func WithStatusUpdatesInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.statusUpdatesInterval = d
	}
}

func WithDoubleSpendCheckInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.doubleSpendTxStatusCheck = d
	}
}

func WithDoubleSpendTxStatusOlderThanInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.doubleSpendTxStatusOlderThan = d
	}
}

func WithProcessTransactionsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processTransactionsInterval = d
	}
}

func WithProcessTransactionsBatchSize(batchSize int) func(*Processor) {
	return func(p *Processor) {
		p.processTransactionsBatchSize = batchSize
	}
}

func WithProcessMinedInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.processMinedInterval = d
	}
}

func WithProcessMinedBatchSize(batchSize int) func(*Processor) {
	return func(p *Processor) {
		p.processMinedBatchSize = batchSize
	}
}

func WithProcessStatusUpdatesBatchSize(size int) func(*Processor) {
	return func(p *Processor) {
		p.statusUpdatesBatchSize = size
	}
}

func WithMessageQueueClient(mqClient mq.MessageQueueClient) func(processor *Processor) {
	return func(p *Processor) {
		p.mqClient = mqClient
	}
}

func WithMinedTxsChan(minedTxsChan chan *blocktx_api.TransactionBlocks) func(processor *Processor) {
	return func(p *Processor) {
		p.minedTxsChan = minedTxsChan
	}
}

func WithSubmittedTxsChan(submittedTxsChan chan *metamorph_api.PostTransactionRequest) func(processor *Processor) {
	return func(p *Processor) {
		p.submittedTxsChan = submittedTxsChan
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

func WithTracerProcessor(attr ...attribute.KeyValue) func(*Processor) {
	return func(p *Processor) {
		p.tracingEnabled = true
		if len(attr) > 0 {
			p.tracingAttributes = append(p.tracingAttributes, attr...)
		}
		_, file, _, ok := runtime.Caller(1)
		if ok {
			p.tracingAttributes = append(p.tracingAttributes, attribute.String("file", file))
		}
	}
}

func WithBlocktxClient(client blocktx.Client) func(*Processor) {
	return func(p *Processor) {
		p.blocktxClient = client
	}
}

func WithRegisterBatchSizeDefault(size int) func(*Processor) {
	return func(p *Processor) {
		p.registerBatchSize = size
	}
}
