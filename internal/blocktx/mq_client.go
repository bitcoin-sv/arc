package blocktx

import (
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type MessageQueueClient interface {
	Subscribe(topic string, cb nats.MsgHandler) error
	PublishMarshal(topic string, m proto.Message) error
	Shutdown()
}
