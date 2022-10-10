package bitcoin

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type Rest handler

func NewRest(endpoint string, opts ...Options) Node {
	nodeInterface := &Rest{
		&options{
			endpoint: endpoint,
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
func (r *Rest) GetTx(ctx context.Context, txID string) (tx []byte, err error) {

	url := fmt.Sprintf("%s/rest/tx/%s.bin", r.options.endpoint, txID)
	return r.doRequest(ctx, "GET", url, nil)
}

// SubmitTx ...
func (r *Rest) SubmitTx(ctx context.Context, tx []byte) (txID string, err error) {

	// TODO submit the tx via Rest interface

	return "", nil
}

func (r *Rest) doRequest(ctx context.Context, method string, url string, body io.Reader) (responseBody []byte, err error) {
	var request *http.Request
	if request, err = http.NewRequestWithContext(ctx, method, url, body); err != nil {
		return
	}

	var response *http.Response
	response, err = r.options.client.Do(request)
	if err != nil {
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()

	return io.ReadAll(response.Body)
}
