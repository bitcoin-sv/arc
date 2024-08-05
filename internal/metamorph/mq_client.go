package metamorph

const (
	SubmitTxTopic   = "submit-tx"
	MinedTxsTopic   = "mined-txs"
	RegisterTxTopic = "register-tx"
	RequestTxTopic  = "request-tx"
)

type MessageQueueClient interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Shutdown()
}
