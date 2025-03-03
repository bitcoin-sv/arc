package metamorph

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	CallbackTopic   = "callback"
)

type MessageQueue interface {
	Publish(ctx context.Context, topic string, data []byte) error
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Status() nats.Status
	Shutdown()
}

type MessageQueueClient interface {
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Shutdown()
}
