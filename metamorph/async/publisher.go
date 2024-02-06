package async

import "context"

type Publisher interface {
	PublishTransaction(ctx context.Context, hash []byte) error
}
