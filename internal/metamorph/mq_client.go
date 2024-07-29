package metamorph

type MessageQueueClient interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, msgFunc func([]byte) error) error
	Shutdown()
}
