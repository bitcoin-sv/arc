package bitcoin

import "net/http"

type Options func(c *options)

type (
	handler struct {
		options *options
	}

	options struct {
		endpoint string
		username string
		password string
		client   *http.Client
	}
)

// WithEndpoint sets the endpoint to use for the call to the bitcoin node
func WithEndpoint(endpoint string) Options {
	return func(o *options) {
		o.endpoint = endpoint
	}
}

// WithClient sets the http client to use
func WithClient(client *http.Client) Options {
	return func(o *options) {
		o.client = client
	}
}
