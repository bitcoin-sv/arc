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
func NewRedisStore(addr, password string, db int) *RedisStore {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisStore{
		client: client,
		ctx:    context.Background(),
	}
}

// Get retrieves a value by key.
func (r *RedisStore) Get(key string) ([]byte, error) {
	result, err := r.client.Get(r.ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheNotFound
	} else if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

// Set stores a value with a TTL.
func (r *RedisStore) Set(key string, value []byte, ttl time.Duration) error {
	err := r.client.Set(r.ctx, key, value, ttl).Err()
	if err != nil {
		return err
	}
	return nil
}

// Del removes a value by key.
func (r *RedisStore) Del(key string) error {
	result, err := r.client.Del(r.ctx, key).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrCacheNotFound
	}
	return nil
}

// SetClient sets the Redis client for the store.
func (r *RedisStore) SetClient(client redis.UniversalClient) {
	r.client = client
}

// SetContext sets the context for the store.
func (r *RedisStore) SetContext(ctx context.Context) {
	r.ctx = ctx
}
