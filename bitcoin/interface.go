package bitcoin

import "context"

type Node interface {
	GetTx(ctx context.Context, txID string) (tx []byte, err error)
	SubmitTx(ctx context.Context, tx []byte) (txID string, err error)
}
