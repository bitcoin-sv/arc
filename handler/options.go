package handler

import "github.com/taal/mapi/config"

type Options func(c *handlerOptions)

type (
	handlerOptions struct {
		security *config.SecurityConfig
	}
)

// WithSecurityConfig will set the security config being used
func WithSecurityConfig(security *config.SecurityConfig) Options {
	return func(c *handlerOptions) {
		if security != nil {
			c.security = security
		}
	}
}
