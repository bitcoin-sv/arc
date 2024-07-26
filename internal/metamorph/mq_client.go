package metamorph

type MessageQueueClient interface {
	Publish(topic string, hash []byte) error
	Shutdown()
}
