package blocktx

import (
	"runtime"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"go.opentelemetry.io/otel/attribute"
)

func WithTransactionBatchSize(size int) func(*Processor) {
	return func(p *Processor) {
		p.transactionStorageBatchSize = size
	}
}

func WithRetentionDays(dataRetentionDays int) func(*Processor) {
	return func(p *Processor) {
		p.dataRetentionDays = dataRetentionDays
	}
}

func WithRegisterTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.registerTxsInterval = d
	}
}

func WithRegisterTxsChan(registerTxsChan chan []byte) func(*Processor) {
	return func(processor *Processor) {
		processor.registerTxsChan = registerTxsChan
	}
}

func WithMinedTxsChan(minedTxsChan chan *blocktx_api.TransactionBlocks) func(*Processor) {
	return func(processor *Processor) {
		processor.minedTxsChan = minedTxsChan
	}
}

func WithRegisterTxsBatchSize(size int) func(*Processor) {
	return func(processor *Processor) {
		processor.registerTxsBatchSize = size
	}
}

func WithTracer(attr ...attribute.KeyValue) func(*Processor) {
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

func WithMaxBlockProcessingDuration(d time.Duration) func(*Processor) {
	return func(processor *Processor) {
		processor.maxBlockProcessingDuration = d
	}
}

func WithIncomingIsLongest(enabled bool) func(*Processor) {
	return func(processor *Processor) {
		processor.incomingIsLongest = enabled
	}
}

func WithPublishMinedMessageSize(size int) func(*Processor) {
	return func(processor *Processor) {
		processor.publishMinedMessageSize = size
	}
}
