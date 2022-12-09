package models

import (
	"github.com/TAAL-GmbH/arc/client"
)

// ModelOps allow functional options to be supplied
// that overwrite default model options
type ModelOps func(m *Model)

// New set this model to a new record
func New() ModelOps {
	return func(m *Model) {
		m.New()
	}
}

// WithClient will set the client on the model
func WithClient(c client.Interface) ModelOps {
	return func(m *Model) {
		if c != nil {
			m.client = c
		}
	}
}

// WithMetadata will add the metadata record to the model
func WithMetadata(key string, value interface{}) ModelOps {
	return func(m *Model) {
		if m.Metadata == nil {
			m.Metadata = make(Metadata)
		}
		m.Metadata[key] = value
	}
}

// WithMetadatas will add multiple metadata records to the model
func WithMetadatas(metadata map[string]interface{}) ModelOps {
	return func(m *Model) {
		if len(metadata) > 0 {
			if m.Metadata == nil {
				m.Metadata = make(Metadata)
			}
			for key, value := range metadata {
				m.Metadata[key] = value
			}
		}
	}
}

// WithPageSize will set the pageSize to use on the model in queries
func WithPageSize(pageSize int) ModelOps {
	return func(m *Model) {
		m.pageSize = pageSize
	}
}
