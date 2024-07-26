package blocktx

import (
	"google.golang.org/protobuf/proto"
)

type MessageQueueClient interface {
	SubscribeRegisterTxs() error
	SubscribeRequestTxs() error
	PublishMarshal(topic string, m proto.Message) error
	Shutdown() error
}
