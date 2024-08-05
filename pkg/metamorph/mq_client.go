package metamorph

import (
	"google.golang.org/protobuf/proto"
)

const (
	SubmitTxTopic = "submit-tx"
)

type MessageQueueClient interface {
	PublishMarshal(topic string, m proto.Message) error
	Shutdown()
}
