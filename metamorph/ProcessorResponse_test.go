package metamorph

import (
	"fmt"
	"sync"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/test"
	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		response := NewProcessorResponse(test.TX1Bytes)
		assert.IsType(t, "string", response.String())
	})
}

func TestNewProcessorResponse(t *testing.T) {
	t.Run("NewProcessorResponse", func(t *testing.T) {
		response := NewProcessorResponse(test.TX1Bytes)
		assert.NotNil(t, response.Start)
		assert.Equal(t, test.TX1Bytes, response.Hash)
		assert.Equal(t, metamorph_api.Status_UNKNOWN, response.Status)
	})
}

func TestGetStatus(t *testing.T) {
	t.Run("GetStatus", func(t *testing.T) {
		response := NewProcessorResponse(test.TX1Bytes)
		assert.Equal(t, metamorph_api.Status_UNKNOWN, response.GetStatus())
	})
}

func TestSetErr(t *testing.T) {
	t.Run("SetErr", func(t *testing.T) {
		response := NewProcessorResponse(test.TX1Bytes)
		assert.Nil(t, response.Err)
		err := fmt.Errorf("test error")
		response.setErr(err, "test")
		assert.Equal(t, err, response.Err)
		assert.Equal(t, err, response.GetErr())
	})

	t.Run("SetErr channel", func(t *testing.T) {
		ch := make(chan StatusAndError)
		response := NewProcessorResponse(test.TX1Bytes)
		assert.Nil(t, response.Err)

		response.ch = ch
		err := fmt.Errorf("test error")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for status := range ch {
				assert.Equal(t, metamorph_api.Status_UNKNOWN, status.Status)
				assert.ErrorIs(t, err, status.Err)
				wg.Done()
			}
		}()

		response.setErr(err, "test")
		wg.Wait()

		assert.Equal(t, err, response.Err)
	})
}

func TestSetStatusAndError(t *testing.T) {
	t.Run("SetStatusAndError", func(t *testing.T) {
		response := NewProcessorResponse(test.TX1Bytes)
		assert.Nil(t, response.Err)
		assert.Equal(t, metamorph_api.Status_UNKNOWN, response.Status)

		err := fmt.Errorf("test error")
		response.setStatusAndError(metamorph_api.Status_SENT_TO_NETWORK, err, "test")
		assert.Equal(t, err, response.Err)
		assert.Equal(t, err, response.GetErr())
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, response.Status)
	})

	t.Run("SetErr channel", func(t *testing.T) {
		ch := make(chan StatusAndError)
		response := NewProcessorResponse(test.TX1Bytes)
		assert.Nil(t, response.Err)
		assert.Equal(t, metamorph_api.Status_UNKNOWN, response.Status)

		response.ch = ch
		err := fmt.Errorf("test error")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for status := range ch {
				assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, status.Status)
				assert.ErrorIs(t, err, status.Err)
				wg.Done()
			}
		}()

		response.setStatusAndError(metamorph_api.Status_SENT_TO_NETWORK, err, "test")
		wg.Wait()

		assert.Equal(t, err, response.Err)
	})
}
