package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemoryCache(t *testing.T) {
	t.Run("in-memory TTL", func(t *testing.T) {
		// given
		cStore := NewMemoryStore()

		key1 := "long-ttl"
		key2 := "short-ttl"

		ttl1 := 1 * time.Second
		ttl2 := 50 * time.Millisecond

		// when
		err := cStore.Set(key1, []byte("1"), ttl1)
		require.NoError(t, err)

		err = cStore.Set(key2, []byte("1"), ttl2)
		require.NoError(t, err)

		time.Sleep(60 * time.Millisecond)

		// then
		val, err := cStore.Get(key1)
		require.NoError(t, err)
		require.NotNil(t, val)

		val, err = cStore.Get(key2)
		require.Nil(t, val)
		require.ErrorIs(t, err, ErrCacheNotFound)
	})
}
