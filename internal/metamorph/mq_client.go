package metamorph

import (
	"context"

	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	RequestTxTopic  = "request-tx"
)

type MessageQueue interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Shutdown()
}

type MessageQueueClient interface {
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Shutdown()
}
