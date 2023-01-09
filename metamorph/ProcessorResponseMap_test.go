package metamorph

import (
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessorResponseMap(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		tests := []struct {
			name   string
			expiry time.Duration
			want   *ProcessorResponseMap
		}{
			{
				name:   "TestNewProcessorResponseMap",
				expiry: 10 * time.Second,
				want: &ProcessorResponseMap{
					expiry: 10 * time.Second,
					items:  make(map[string]*ProcessorResponse),
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equalf(t, tt.want, NewProcessorResponseMap(tt.expiry), "NewProcessorResponseMap(%v)", tt.expiry)
			})
		}
	})

	t.Run("go routine", func(t *testing.T) {
		proc := NewProcessorResponseMap(50 * time.Millisecond)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		item, ok := proc.Get(tx1)
		assert.True(t, ok)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())

		time.Sleep(100 * time.Millisecond)

		// should be cleaned up
		item, ok = proc.Get(tx1)
		assert.False(t, ok)
		assert.Nil(t, item)
	})
}

func TestProcessorResponseMap_Clear(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(50 * time.Millisecond)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		item, ok := proc.Get(tx1)
		assert.True(t, ok)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())

		proc.Clear()

		item, ok = proc.Get(tx1)
		assert.False(t, ok)
		assert.Nil(t, item)
	})
}

func TestProcessorResponseMap_Delete(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		proc.Set(tx2, NewProcessorResponseWithStatus(tx2Bytes, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

		item, ok := proc.Get(tx1)
		require.True(t, ok)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())

		item2, ok2 := proc.Get(tx2)
		require.True(t, ok2)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, item2.GetStatus())

		proc.Delete(tx1)

		item, ok = proc.Get(tx1)
		require.False(t, ok)
		assert.Nil(t, item)

		item2, ok2 = proc.Get(tx2)
		require.True(t, ok2)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, item2.GetStatus())
	})
}

func TestProcessorResponseMap_Get(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		_, ok := proc.Get(tx1)
		require.False(t, ok)
	})

	t.Run("returned", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))

		item, ok := proc.Get(tx1)
		require.True(t, ok)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
	})

	t.Run("status returned", func(t *testing.T) {
		tests := []struct {
			status metamorph_api.Status
			ok     bool
		}{
			{status: metamorph_api.Status_UNKNOWN, ok: true},
			{status: metamorph_api.Status_QUEUED, ok: true},
			{status: metamorph_api.Status_RECEIVED, ok: true},
			{status: metamorph_api.Status_STORED, ok: true},
			{status: metamorph_api.Status_ANNOUNCED_TO_NETWORK, ok: true},
			{status: metamorph_api.Status_SENT_TO_NETWORK, ok: true},
			{status: metamorph_api.Status_SEEN_ON_NETWORK, ok: true},
			{status: metamorph_api.Status_MINED, ok: true},
			{status: metamorph_api.Status_CONFIRMED, ok: true},
			{status: metamorph_api.Status_REJECTED, ok: true},
		}
		for _, tt := range tests {
			proc := NewProcessorResponseMap(2 * time.Second)
			proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, tt.status))

			item, ok := proc.Get(tx1)
			require.Equal(t, ok, tt.ok)
			if tt.ok {
				assert.Equal(t, tt.status, item.GetStatus())
			}
		}
	})
}

func TestProcessorResponseMap_Items(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		assert.Len(t, proc.Items(), 0)
	})

	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		proc.Set(tx2, NewProcessorResponseWithStatus(tx2Bytes, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

		items := proc.Items()
		assert.Equal(t, 2, len(items))
		assert.Equal(t, tx1Bytes, items[tx1].Hash)
		assert.Equal(t, tx2Bytes, items[tx2].Hash)
	})
}

func TestProcessorResponseMap_Len(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		assert.Equal(t, 0, proc.Len())
	})

	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		proc.Set(tx2, NewProcessorResponseWithStatus(tx2Bytes, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

		assert.Equal(t, 2, proc.Len())
	})
}

func TestProcessorResponseMap_Set(t *testing.T) {
	t.Run("empty key", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set("", NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		assert.Len(t, proc.items, 1)

		item := proc.items[""]
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
	})

	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		assert.Len(t, proc.items, 0)

		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))

		assert.Len(t, proc.items, 1)

		item := proc.items[tx1]
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
	})
}

func TestProcessorResponseMap_clean(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		proc := NewProcessorResponseMap(2 * time.Second)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		proc.Set(tx2, NewProcessorResponseWithStatus(tx2Bytes, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

		proc.clean()

		assert.Equal(t, 2, proc.Len())
	})

	t.Run("expiry", func(t *testing.T) {
		proc := NewProcessorResponseMap(50 * time.Millisecond)
		proc.Set(tx1, NewProcessorResponseWithStatus(tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK))
		proc.Set(tx2, NewProcessorResponseWithStatus(tx2Bytes, metamorph_api.Status_ANNOUNCED_TO_NETWORK))

		time.Sleep(100 * time.Millisecond)

		proc.clean()

		assert.Equal(t, 0, proc.Len())
	})
}
