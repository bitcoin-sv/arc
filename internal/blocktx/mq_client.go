package blocktx

import (
	"context"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
)

type MessageQueueClient interface {
	PublishMarshal(ctx context.Context, topic string, m proto.Message) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Status() nats.Status
	Shutdown()
}
