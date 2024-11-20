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

// Set stores a value with a TTL for key.
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

// MapGet retrieves a value by key and hash (if given). Return err if hash or key not found.
func (r *RedisStore) MapGet(hash string, key string) ([]byte, error) {
	result, err := r.client.HGet(r.ctx, hash, key).Result()

	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheNotFound
	} else if err != nil {
		return nil, errors.Join(ErrCacheFailedToGet, err)
	}

	return []byte(result), nil
}

// MapSet stores a value for a specific hash.
func (r *RedisStore) MapSet(hash string, key string, value []byte) error {
	err := r.client.HSet(r.ctx, hash, key, value).Err()

	if err != nil {
		return errors.Join(ErrCacheFailedToSet, err)
	}

	return nil
}

// MapDel removes a value by key in specific hash.
func (r *RedisStore) MapDel(hash string, keys ...string) error {
	result, err := r.client.HDel(r.ctx, hash, keys...).Result()

	if err != nil {
		return errors.Join(ErrCacheFailedToDel, err)
	}
	if result == 0 {
		return ErrCacheNotFound
	}

	return nil
}

// MapGetAll retrieves all key-value pairs for a specific hash. Return err if hash not found.
func (r *RedisStore) MapGetAll(hash string) (map[string][]byte, error) {
	values, err := r.client.HGetAll(r.ctx, hash).Result()
	if err != nil {
		return nil, errors.Join(ErrCacheFailedToGet, err)
	}

	result := make(map[string][]byte)
	for field, value := range values {
		result[field] = []byte(value)
	}
	return result, nil
}

// MapExtractAll retrieves all key-value pairs for a specific hash and remove them from cache, all in one transaction.
func (r *RedisStore) MapExtractAll(hash string) (map[string][]byte, error) {
	tx := r.client.TxPipeline()

	getAllCmd := tx.HGetAll(r.ctx, hash)
	tx.Del(r.ctx, hash)

	_, err := tx.Exec(r.ctx)
	if err != nil {
		return nil, errors.Join(ErrCacheFailedToExecuteTx, err)
	}

	getAllCmdResult := getAllCmd.Val()

	result := make(map[string][]byte)
	for field, value := range getAllCmdResult {
		result[field] = []byte(value)
	}
	return result, nil
}

// MapLen returns the number of elements in a hash.
func (r *RedisStore) MapLen(hash string) (int64, error) {
	count, err := r.client.HLen(r.ctx, hash).Result()
	if err != nil {
		return 0, errors.Join(ErrCacheFailedToGetCount, err)
	}
	return count, nil
}
