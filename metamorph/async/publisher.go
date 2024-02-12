package async

type Publisher interface {
	PublishTransaction(hash []byte) error
}
