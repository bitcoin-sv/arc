package processor_response

import (
	"fmt"
	"sync"
	"testing"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.IsType(t, "string", response.String())
	})
}

func TestNewProcessorResponse(t *testing.T) {
	t.Run("NewProcessorResponse", func(t *testing.T) {
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.NotNil(t, response.Start)
		assert.Equal(t, testdata.TX1Hash, response.Hash)
		assert.Equal(t, metamorph_api.Status_RECEIVED, response.Status)
	})
}

func TestGetStatus(t *testing.T) {
	t.Run("GetStatus", func(t *testing.T) {
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.Equal(t, metamorph_api.Status_RECEIVED, response.GetStatus())
	})
}

func TestSetErr(t *testing.T) {
	t.Run("SetErr", func(t *testing.T) {
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.Nil(t, response.Err)
		err := fmt.Errorf("test error")
		response.setErr(err, "test")
		assert.Equal(t, err, response.Err)
		assert.Equal(t, err, response.GetErr())
	})

	t.Run("SetErr channel", func(t *testing.T) {
		ch := make(chan StatusAndError)
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.Nil(t, response.Err)

		response.callerCh = ch
		err := fmt.Errorf("test error")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for status := range ch {
				assert.Equal(t, metamorph_api.Status_RECEIVED, status.Status)
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
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.Nil(t, response.Err)
		assert.Equal(t, metamorph_api.Status_RECEIVED, response.Status)

		err := fmt.Errorf("test error")
		response.setStatusAndError(metamorph_api.Status_SENT_TO_NETWORK, err, "test")
		assert.Equal(t, err, response.Err)
		assert.Equal(t, err, response.GetErr())
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, response.Status)
	})

	t.Run("SetErr channel", func(t *testing.T) {
		ch := make(chan StatusAndError)
		response := NewProcessorResponse(testdata.TX1Hash)
		assert.Nil(t, response.Err)
		assert.Equal(t, metamorph_api.Status_RECEIVED, response.Status)

		response.callerCh = ch
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
