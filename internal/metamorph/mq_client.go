package metamorph

type MessageQueueClient interface {
	PublishRegisterTxs(hash []byte) error
	PublishRequestTx(hash []byte) error
	SubscribeMinedTxs() error
	SubscribeSubmittedTx() error
	Shutdown() error
}
