package blocktx

import (
	"time"

	"go.opentelemetry.io/otel"
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

func WithFillGapsInterval(interval time.Duration) func(handler *Processor) {
	return func(handler *Processor) {
		handler.fillGapsInterval = interval
	}
}

func WithRegisterTxsInterval(d time.Duration) func(handler *Processor) {
	return func(p *Processor) {
		p.registerTxsInterval = d
	}
}

func WithRegisterRequestTxsInterval(d time.Duration) func(handler *Processor) {
	return func(p *Processor) {
		p.registerRequestTxsInterval = d
	}
}

func WithRegisterTxsChan(registerTxsChan chan []byte) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerTxsChan = registerTxsChan
	}
}

func WithRequestTxChan(requestTxChannel chan []byte) func(handler *Processor) {
	return func(handler *Processor) {
		handler.requestTxChannel = requestTxChannel
	}
}

func WithRegisterTxsBatchSize(size int) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerTxsBatchSize = size
	}
}

func WithRegisterRequestTxsBatchSize(size int) func(handler *Processor) {
	return func(handler *Processor) {
		handler.registerRequestTxsBatchSize = size
	}
}

func WithTracer() func(handler *Processor) {
	return func(_ *Processor) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
}
