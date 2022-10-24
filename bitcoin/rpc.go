package bitcoin

import (
	"context"
	"net/http"

	"github.com/TAAL-GmbH/mapi/config"
)

type RPC handler

func NewRPC(config *config.RPCClientConfig, opts ...Options) Node {
	nodeInterface := &RPC{
		&options{
			endpoint: config.Host,
			username: config.User,
			password: config.Password,
			client:   &http.Client{},
		},
	}

	// Overwrite defaults with any custom options provided by the user
	for _, opt := range opts {
		opt(nodeInterface.options)
	}

	return nodeInterface
}

// GetTx ...
func (r *RPC) GetTx(ctx context.Context, txID string) (tx []byte, err error) {

	// get the tx via RPC interface

	return nil, nil
}

// SubmitTx ...
func (r *RPC) SubmitTx(ctx context.Context, tx []byte) (txID string, err error) {

	// submit the tx via RPC interface

	return "", nil
}
