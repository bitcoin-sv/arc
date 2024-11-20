package metamorph

import "context"

const (
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	RequestTxTopic  = "request-tx"
)

type MessageQueueClient interface {
	Publish(ctx context.Context, topic string, data []byte) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Shutdown()
}
