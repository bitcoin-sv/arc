package metamorph

import (
	"context"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessorRequest(t *testing.T) {
	t.Run("should return a new ProcessorRequest", func(t *testing.T) {
		s := &store.StoreData{}
		responseChannel := make(chan StatusAndError)

		processorRequest := NewProcessorRequest(context.Background(), s, responseChannel)

		assert.NotNil(t, processorRequest)
		assert.Equal(t, s, processorRequest.StoreData)
		assert.Equal(t, responseChannel, processorRequest.ResponseChannel)
	})
}
