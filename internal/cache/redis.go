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
func (r *RedisStore) Del(keys ...string) error {
	result, err := r.client.Del(r.ctx, keys...).Result()
	if err != nil {
		return errors.Join(ErrCacheFailedToDel, err)
	}
	if result == 0 {
		return ErrCacheNotFound
	}
	return nil
}

// GetAllWithPrefix retrieves all key-value pairs that match a specific prefix.
func (r *RedisStore) GetAllWithPrefix(prefix string) (map[string][]byte, error) {
	var cursor uint64
	results := make(map[string][]byte)

	for {
		keys, newCursor, err := r.client.Scan(r.ctx, cursor, prefix+"*", 10).Result()
		if err != nil {
			return nil, errors.Join(ErrCacheFailedToScan, err)
		}

		for _, key := range keys {
			value, err := r.client.Get(r.ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				// Key has been removed between SCAN and GET, skip it
				continue
			} else if err != nil {
				return nil, errors.Join(ErrCacheFailedToGet, err)
			}
			results[key] = []byte(value)
		}

		cursor = newCursor
		if cursor == 0 {
			break
		}
	}
	return results, nil
}
