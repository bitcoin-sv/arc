package handler

import "github.com/TAAL-GmbH/mapi/config"

type Options func(c *handlerOptions)

type (
	handlerOptions struct {
		security *config.SecurityConfig
	}
)

// TransactionOptions options passed from header when creating transactions
type TransactionOptions struct {
	CallbackURL   string `json:"callback_url,omitempty"`
	CallbackToken string `json:"callback_token,omitempty"`
	MerkleProof   bool   `json:"merkle_proof,omitempty"`
}

// WithSecurityConfig will set the security config being used
func WithSecurityConfig(security *config.SecurityConfig) Options {
	return func(c *handlerOptions) {
		if security != nil {
			c.security = security
		}
	}
}
