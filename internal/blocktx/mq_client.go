package blocktx

import (
	"google.golang.org/protobuf/proto"
)

const (
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	RequestTxTopic  = "request-tx"
)

type MessageQueueClient interface {
	PublishMarshal(topic string, m proto.Message) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Shutdown()
}
