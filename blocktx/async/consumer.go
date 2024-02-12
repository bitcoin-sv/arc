package async

type Consumer interface {
	ConsumeTransactions() error
}
