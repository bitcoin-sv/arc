package metamorph

import (
	"google.golang.org/protobuf/proto"
)

type MessageQueueClient interface {
	PublishMarshal(topic string, m proto.Message) error
	Shutdown()
}
