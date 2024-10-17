package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStore is an implementation of CacheStore using Redis.
type RedisStore struct {
	client redis.UniversalClient
	ctx    context.Context
}

// NewRedisStore initializes a RedisStore.
func NewRedisStore(ctx context.Context, c redis.UniversalClient) *RedisStore {
	return &RedisStore{
		client: c,
		ctx:    ctx,
	}
}

// Get retrieves a value by key.
func (r *RedisStore) Get(key string) ([]byte, error) {
	result, err := r.client.Get(r.ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheNotFound
	} else if err != nil {
		return nil, errors.Join(ErrCacheFailedToGet, err)
	}
	return []byte(result), nil
}

// Set stores a value with a TTL.
func (r *RedisStore) Set(key string, value []byte, ttl time.Duration) error {
	err := r.client.Set(r.ctx, key, value, ttl).Err()
	if err != nil {
		return errors.Join(ErrCacheFailedToSet, err)
	}
	return nil
}

// Del removes a value by key.
func (r *RedisStore) Del(key string) error {
	result, err := r.client.Del(r.ctx, key).Result()
	if err != nil {
		return errors.Join(ErrCacheFailedToDel, err)
	}
	if result == 0 {
		return ErrCacheNotFound
	}
	return nil
}
