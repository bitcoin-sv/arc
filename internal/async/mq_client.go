package async

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

const (
	maxBatchSizeDefault = 20

	SubmitTxTopic = "submit-tx"
	submitTxGroup = "submit-tx-group"
)

var tracer trace.Tracer

type NatsClient interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
	Publish(subj string, data []byte) error
	Drain() error
}

type MQClient struct {
	nc                      NatsClient
	logger                  *slog.Logger
	registerTxsSubscription *nats.Subscription
	requestSubscription     *nats.Subscription
	minedTxsSubscription    *nats.Subscription
	maxBatchSize            int
	registerTxsChannel      chan []byte
	requestTxChannel        chan []byte
	minedTxsChan            chan *blocktx_api.TransactionBlocks
	submittedTxsChan        chan *metamorph_api.TransactionRequest
}

func WithMaxBatchSize(size int) func(*MQClient) {
	return func(m *MQClient) {
		m.maxBatchSize = size
	}
}

func WithTracer() func(handler *MQClient) {
	return func(_ *MQClient) {
		tracer = otel.GetTracerProvider().Tracer("")
	}
}

func WithLogger(logger *slog.Logger) func(handler *MQClient) {
	return func(m *MQClient) {
		m.logger = logger
	}
}

func WithMinedTxsChan(minedTxsChan chan *blocktx_api.TransactionBlocks) func(handler *MQClient) {
	return func(m *MQClient) {
		m.minedTxsChan = minedTxsChan
	}
}

func WithSubmittedTxsChan(submittedTxsChan chan *metamorph_api.TransactionRequest) func(handler *MQClient) {
	return func(m *MQClient) {
		m.submittedTxsChan = submittedTxsChan
	}
}

func WithRegisterTxsChan(registerTxsChannel chan []byte) func(handler *MQClient) {
	return func(m *MQClient) {
		m.registerTxsChannel = registerTxsChannel
	}
}

func WithRequestTxsChan(requestTxChannel chan []byte) func(handler *MQClient) {
	return func(m *MQClient) {
		m.requestTxChannel = requestTxChannel
	}
}

func NewNatsMQClient(nc NatsClient, opts ...func(client *MQClient)) *MQClient {
	m := &MQClient{
		nc:           nc,
		logger:       slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		maxBatchSize: maxBatchSizeDefault,
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (c MQClient) Shutdown() error {
	if c.nc != nil {
		err := c.nc.Drain()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c MQClient) Publish(topic string, hash []byte) error {
	err := c.nc.Publish(topic, hash)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}

func (c MQClient) PublishMarshal(topic string, m proto.Message) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	err = c.nc.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish on %s topic: %w", topic, err)
	}

	return nil
}
