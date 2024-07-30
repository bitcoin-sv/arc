package metamorph

import "github.com/nats-io/nats.go"

type MessageQueueClient interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, cb nats.MsgHandler) error
}
