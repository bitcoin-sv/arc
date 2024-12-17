package callbacker

import (
	"github.com/nats-io/nats.go/jetstream"
)

const (
	CallbackTopic = "callback"
)

type MessageQueueClient interface {
	SubscribeMsg(topic string, msgFunc func(msg jetstream.Msg) error) error
	Shutdown()
}
