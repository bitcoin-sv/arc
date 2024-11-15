package cache

import (
	"errors"
	"sync"
	"time"
)

type MemoryStore struct {
	data sync.Map
}

type cacheItem struct {
	expiration time.Time
	value      []byte
}

const clearCacheInterval = 10 * time.Second

func NewMemoryStore() *MemoryStore {
	mStore := &MemoryStore{}

	mStore.startClearCache(clearCacheInterval)

	return mStore
}

// Get retrieves a value by key.
func (s *MemoryStore) Get(key string) ([]byte, error) {
	value, found := s.data.Load(key)
	if !found {
		return nil, ErrCacheNotFound
	}

	cacheItem, ok := value.(cacheItem)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	if time.Now().After(cacheItem.expiration) {
		s.data.Delete(key)
		return nil, ErrCacheNotFound
	}

	return cacheItem.value, nil
}

// Set stores a key-value pair, ignoring the ttl parameter.
func (s *MemoryStore) Set(key string, value []byte, ttl time.Duration) error {
	expiration := time.Now().Add(ttl)
	s.data.Store(key, cacheItem{
		expiration: expiration,
		value:      value,
	})
	return nil
}

// Del removes a key from the store.
func (s *MemoryStore) Del(hash *string, keys ...string) error {
	if hash != nil {
		hashValue, found := s.data.Load(*hash)
		if !found {
			return ErrCacheNotFound
		}

		hashMap, ok := hashValue.(map[string][]byte)
		if !ok {
			return errors.Join(ErrCacheFailedToDel, ErrCacheFailedToGet)
		}

		for _, k := range keys {
			delete(hashMap, k)
		}

		s.data.Store(*hash, hashMap)
		return nil
	}

	for _, k := range keys {
		s.data.Delete(k)
	}
	return nil
}

func (s *MemoryStore) startClearCache(interval time.Duration) {
	go func() {
		for now := range time.Tick(interval) {
			s.data.Range(func(key, value any) bool {
				cacheItem, ok := value.(cacheItem)
				if !ok {
					return true // continue iteration
				}
				if now.After(cacheItem.expiration) {
					s.data.Delete(key.(string))
				}
				return true // continue iteration
			})
		}
	}()
}

// MapGet retrieves a value by key and hashsetKey. Return err if hashsetKey or key not found.
func (s *MemoryStore) MapGet(hashsetKey string, key string) ([]byte, error) {
	hashValue, found := s.data.Load(hashsetKey)
	if !found {
		return nil, ErrCacheNotFound
	}

	hashMap, ok := hashValue.(map[string][]byte)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	fieldValue, exists := hashMap[key]
	if !exists {
		return nil, ErrCacheNotFound
	}

	return fieldValue, nil
}

// MapSet stores a key-value pair for specific hashsetKey.
func (s *MemoryStore) MapSet(hashsetKey string, key string, value []byte) error {
	raw, _ := s.data.LoadOrStore(hashsetKey, make(map[string][]byte))

	hashMap, ok := raw.(map[string][]byte)
	if !ok {
		return ErrCacheFailedToSet
	}

	hashMap[key] = value

	s.data.Store(hashsetKey, hashMap)
	return nil
}

// MapDel removes a value by key in specific hashsetKey.
func (s *MemoryStore) MapDel(hashsetKey string, keys ...string) error {
	hashValue, found := s.data.Load(hashsetKey)
	if !found {
		return ErrCacheNotFound
	}

	hashMap, ok := hashValue.(map[string][]byte)
	if !ok {
		return errors.Join(ErrCacheFailedToDel, ErrCacheFailedToGet)
	}

	for _, k := range keys {
		delete(hashMap, k)
	}

	s.data.Store(hashsetKey, hashMap)
	return nil
}

// MapGetAll retrieves all key-value pairs for a specific hashsetKey. Return err if hashsetKey not found.
func (s *MemoryStore) MapGetAll(hashsetKey string) (map[string][]byte, error) {
	hashValue, found := s.data.Load(hashsetKey)
	if !found {
		return nil, ErrCacheNotFound
	}

	hashMap, ok := hashValue.(map[string][]byte)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	return hashMap, nil
}

// MapExtractAll retrieves all key-value pairs for a specific hashsetKey and deletes the hashsetKey. Return err if hashsetKey not found.
func (s *MemoryStore) MapExtractAll(hashsetKey string) (map[string][]byte, error) {
	hashValue, found := s.data.LoadAndDelete(hashsetKey)
	if !found {
		return nil, ErrCacheNotFound
	}

	hashMap, ok := hashValue.(map[string][]byte)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	return hashMap, nil
}

// MapLen returns the number of elements in a hashsetKey in memory.
func (s *MemoryStore) MapLen(hashsetKey string) (int64, error) {
	hashMap, err := s.MapGetAll(hashsetKey)
	if err != nil {
		if errors.Is(err, ErrCacheNotFound) {
			return 0, nil
		}
		return 0, err
	}

	return int64(len(hashMap)), nil
}
