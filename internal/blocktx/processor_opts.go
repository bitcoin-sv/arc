package blocktx

import (
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func WithMessageQueueClient(mqClient MessageQueueClient) func(*Processor) {
	return func(p *Processor) {
		p.mqClient = mqClient
	}
}

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

func WithRegisterRequestTxsInterval(d time.Duration) func(*Processor) {
	return func(p *Processor) {
		p.registerRequestTxsInterval = d
	}
}

func WithRegisterTxsChan(registerTxsChan chan []byte) func(*Processor) {
	return func(processor *Processor) {
		processor.registerTxsChan = registerTxsChan
	}
}

func WithRequestTxChan(requestTxChannel chan []byte) func(*Processor) {
	return func(processor *Processor) {
		processor.requestTxChannel = requestTxChannel
	}
}

func WithRegisterTxsBatchSize(size int) func(*Processor) {
	return func(processor *Processor) {
		processor.registerTxsBatchSize = size
	}
}

func WithRegisterRequestTxsBatchSize(size int) func(*Processor) {
	return func(processor *Processor) {
		processor.registerRequestTxsBatchSize = size
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
