package cache_test

import (
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/stretchr/testify/require"
)

func TestFreecacheStore(t *testing.T) {
	size := 100 * 1024 * 1024 // 100MB
	store := cache.NewFreecacheStore(size)

	key := "example"
	value := []byte("hello world")
	ttl := 5 * time.Second

	// Test Set
	err := store.Set(key, value, ttl)
	require.NoError(t, err, "expected no error on Set")

	// Test Get
	retrievedValue, err := store.Get(key)
	require.NoError(t, err, "expected no error on Get")
	require.Equal(t, value, retrievedValue, "expected retrieved value to match set value")

	// Test Get after TTL expiry
	time.Sleep(ttl + 1*time.Second)
	retrievedValue, err = store.Get(key)
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Get after TTL expiry")
	require.Nil(t, retrievedValue, "expected nil value on Get after TTL expiry")

	// Test Delete
	err = store.Set(key, value, ttl)
	require.NoError(t, err, "expected no error on Set before Delete")

	err = store.Del(key)
	require.NoError(t, err, "expected no error on Delete")

	retrievedValue, err = store.Get(key)
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Get after Delete")
	require.Nil(t, retrievedValue, "expected nil value on Get after Delete")

	// Test Delete non-existent key
	err = store.Del("nonexistent")
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Delete for non-existent key")
}
