package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/require"
)

func setupMockRedisStore() (*cache.RedisStore, redismock.ClientMock) {
	client, mock := redismock.NewClientMock()
	store := cache.NewRedisStore("", "", 0)
	store.SetClient(client)
	store.SetContext(context.Background())
	return store, mock
}

func TestRedisStore(t *testing.T) {
	store, mock := setupMockRedisStore()

	key := "example"
	value := []byte("hello world")
	ttl := 5 * time.Second

	// Test Set
	mock.ExpectSet(key, value, ttl).SetVal("OK")
	err := store.Set(key, value, ttl)
	require.NoError(t, err, "expected no error on Set")

	// Test Get
	mock.ExpectGet(key).SetVal(string(value))
	retrievedValue, err := store.Get(key)
	require.NoError(t, err, "expected no error on Get")
	require.Equal(t, value, retrievedValue, "expected retrieved value to match set value")

	// Test Get after TTL expiry
	mock.ExpectGet(key).RedisNil()
	retrievedValue, err = store.Get(key)
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Get after TTL expiry")
	require.Nil(t, retrievedValue, "expected nil value on Get after TTL expiry")

	// Test Delete
	mock.ExpectDel(key).SetVal(1)
	err = store.Del(key)
	require.NoError(t, err, "expected no error on Delete")

	mock.ExpectGet(key).RedisNil()
	retrievedValue, err = store.Get(key)
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Get after Delete")
	require.Nil(t, retrievedValue, "expected nil value on Get after Delete")

	// Test Delete non-existent key
	mock.ExpectDel("nonexistent").SetVal(0)
	err = store.Del("nonexistent")
	require.ErrorIs(t, err, cache.ErrCacheNotFound, "expected error on Delete for non-existent key")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("there were unfulfilled expectations: %s", err)
	}
}
