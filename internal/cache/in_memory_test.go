package cache

import (
	"testing"
	"time"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/stretchr/testify/require"
)

func TestInMemoryCache(t *testing.T) {
	testutils.RunParallel(t, true, "in-memory TTL", func(t *testing.T) {
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

func TestInMemoryCacheLifeCycle(t *testing.T) {
	testutils.RunParallel(t, true, "in-memory TTL", func(t *testing.T) {
		// given
		cStore := NewMemoryStore()

		key1 := "long-ttl"
		key2 := "short-ttl"

		ttl1 := 1 * time.Second
		ttl2 := 50 * time.Millisecond

		// when keys are set then no errors are expected
		err := cStore.Set(key1, []byte("1"), ttl1)
		require.NoError(t, err)

		err = cStore.Set(key2, []byte("1"), ttl2)
		require.NoError(t, err)

		// when keys are deleted then ErrCacheNotFound is expected
		err = cStore.Del(key2)
		require.NoError(t, err)
		val, err := cStore.Get(key2)
		require.Nil(t, val)
		require.ErrorIs(t, err, ErrCacheNotFound)

		// when ttl expires then ErrCacheNotFound is expected
		// when ttl is not expired, then a value is expected
		err = cStore.Set(key2, []byte("1"), ttl2)
		require.NoError(t, err)

		time.Sleep(60 * time.Millisecond)

		val, err = cStore.Get(key1)
		require.NoError(t, err)
		require.NotNil(t, val)

		val, err = cStore.Get(key2)
		require.Nil(t, val)
		require.ErrorIs(t, err, ErrCacheNotFound)

		//when a MapSet key/value is stored then no errors are expected
		err = cStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)

		//when a MapSet value is deleted then ErrCacheNotFound is expected for that key
		err = cStore.MapDel("NonExistingKey")
		require.ErrorIs(t, err, ErrCacheNotFound)
		err = cStore.MapDel("hash", "key1")
		require.NoError(t, err)

		val, err = cStore.MapGet("hash", "notExistingKey")
		require.Nil(t, val)
		require.ErrorIs(t, err, ErrCacheNotFound)

		//When a MapSet key/value is stored then no errors are expected to retrieve the existing value
		err = cStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)

		val, err = cStore.MapGet("hash", "key1")
		require.NotNil(t, val)
		require.NoError(t, err)

		val, err = cStore.MapGet("hash", "key2")
		require.Nil(t, val)
		require.ErrorIs(t, err, ErrCacheNotFound)

		//When a MapGetaAll is invoked to retrieve all the items in the map then the error will depend on the map existing or not
		mapVal, err := cStore.MapGetAll("hash")
		require.NotNil(t, mapVal)
		require.NoError(t, err)

		mapVal, err = cStore.MapGetAll("notExistingHash")
		require.Nil(t, mapVal)
		require.ErrorIs(t, err, ErrCacheNotFound)

		//When the map length is checked then a valid return is expected
		mapLen, err := cStore.MapLen("hash")
		require.Equal(t, mapLen, int64(1))
		require.NoError(t, err)

		//When the map length is checked against a non-existing map, then an error is expected
		mapLen, err = cStore.MapLen("notExistingHash")
		require.Equal(t, mapLen, int64(0))
		require.NoError(t, err)

		//When the map length is checked against an existing map, then a valid length is expected
		mapExtractAll, err := cStore.MapExtractAll("hash")
		require.NotNil(t, mapExtractAll)
		require.NoError(t, err)

		//When the map length is checked against a non-existing map, then an error is expected
		mapExtractAll, err = cStore.MapExtractAll("hash2")
		require.Nil(t, mapExtractAll)
		require.ErrorIs(t, err, ErrCacheNotFound)
	})
}
