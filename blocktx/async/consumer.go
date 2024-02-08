package async

import "context"

type Consumer interface {
	ConsumeTransactions(ctx context.Context) error
}
