package metamorph_test

import (
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/metamorph"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/processor_response"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessorResponseMapGet(t *testing.T) {
	tt := []struct {
		name string
		hash *chainhash.Hash

		expectedFound bool
	}{
		{
			name: "found",
			hash: testdata.TX1Hash,

			expectedFound: true,
		},
		{
			name: "not found",
			hash: testdata.TX2Hash,

			expectedFound: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			proc := metamorph.NewProcessorResponseMap(50 * time.Millisecond)
			proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))

			item, ok := proc.Get(tc.hash)
			require.Equal(t, tc.expectedFound, ok)

			if tc.expectedFound {
				require.Equal(t, metamorph_api.Status_RECEIVED, item.GetStatus())
			}
		})
	}
}

func TestProcessorResponseMap_Delete(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))
		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponse(testdata.TX2Hash))

		item, ok := proc.Get(testdata.TX1Hash)
		require.True(t, ok)
		assert.Equal(t, metamorph_api.Status_RECEIVED, item.GetStatus())

		item2, ok2 := proc.Get(testdata.TX2Hash)
		require.True(t, ok2)
		assert.Equal(t, metamorph_api.Status_RECEIVED, item2.GetStatus())

		proc.Delete(testdata.TX1Hash)

		item, ok = proc.Get(testdata.TX1Hash)
		require.False(t, ok)
		assert.Nil(t, item)

		item2, ok2 = proc.Get(testdata.TX2Hash)
		require.True(t, ok2)
		assert.Equal(t, metamorph_api.Status_RECEIVED, item2.GetStatus())
	})
}

func TestProcessorResponseMap_Get(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		_, ok := proc.Get(testdata.TX1Hash)
		require.False(t, ok)
	})

	t.Run("returned", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))

		item, ok := proc.Get(testdata.TX1Hash)
		require.True(t, ok)
		assert.Equal(t, metamorph_api.Status_RECEIVED, item.GetStatus())
	})
}

func TestProcessorResponseMap_Items(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		assert.Len(t, proc.Items(), 0)
	})

	t.Run("default", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))
		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponse(testdata.TX2Hash))

		items := proc.Items()
		assert.Equal(t, 2, len(items))
		assert.Equal(t, testdata.TX1Hash, items[*testdata.TX1Hash].Hash)
		assert.Equal(t, testdata.TX2Hash, items[*testdata.TX2Hash].Hash)
	})
}

func TestProcessorResponseMap_Len(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		assert.Equal(t, 0, proc.Len())
	})

	t.Run("default", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))
		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponse(testdata.TX2Hash))

		assert.Equal(t, 2, proc.Len())
	})
}

func TestProcessorResponseMap_Set(t *testing.T) {
	t.Run("empty key", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		proc.Set(&chainhash.Hash{}, processor_response.NewProcessorResponse(testdata.TX1Hash))
		assert.Len(t, proc.ResponseItems, 1)

		item := proc.ResponseItems[chainhash.Hash{}]
		assert.Equal(t, metamorph_api.Status_RECEIVED, item.GetStatus())
	})

	t.Run("default", func(t *testing.T) {
		proc := metamorph.NewProcessorResponseMap(2 * time.Second)
		assert.Len(t, proc.ResponseItems, 0)

		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponse(testdata.TX1Hash))

		assert.Len(t, proc.ResponseItems, 1)

		item := proc.ResponseItems[*testdata.TX1Hash]
		assert.Equal(t, metamorph_api.Status_RECEIVED, item.GetStatus())
	})
}
