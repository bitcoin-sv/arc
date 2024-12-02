package blocktx

import (
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func WithMessageQueueClient(mqClient MessageQueueClient) func(handler *Processor) {
	return func(p *Processor) {
		p.mqClient = mqClient
	}
}

func WithTransactionBatchSize(size int) func(handler *Processor) {
	return func(p *Processor) {
		p.transactionStorageBatchSize = size
	}
}

func WithRetentionDays(dataRetentionDays int) func(handler *Processor) {
	return func(p *Processor) {
		p.dataRetentionDays = dataRetentionDays
	}
}

func WithRequestTxsInterval(d time.Duration) func(handler *Processor) {
	return func(p *Processor) {
		p.requestTxsInterval = d
	}
}

func WithRequestTxChan(requestTxChannel chan []byte) func(handler *Processor) {
	return func(handler *Processor) {
		handler.requestTxChannel = requestTxChannel
	}
}

func WithRequestTxsBatchSize(size int) func(handler *Processor) {
	return func(handler *Processor) {
		handler.requestTxsBatchSize = size
	}
}

func WithTracer(attr ...attribute.KeyValue) func(s *Processor) {
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
