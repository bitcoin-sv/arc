package metamorph

type MessageQueueClient interface {
	Publish(topic string, hash []byte) error
	SubscribeMinedTxs() error
	SubscribeSubmittedTx() error
	Shutdown() error
}
