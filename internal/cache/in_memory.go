package cache

import (
	"sync"
	"time"
)

type MemoryStore struct {
	data sync.Map
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Get retrieves a value by key. It returns an error if the key does not exist.
func (s *MemoryStore) Get(key string) ([]byte, error) {
	value, found := s.data.Load(key)
	if !found {
		return nil, ErrCacheNotFound
	}

	bytes := value.([]byte)

	return bytes, nil
}

// Set stores a key-value pair, ignoring the ttl parameter.
func (s *MemoryStore) Set(key string, value []byte, _ time.Duration) error {
	s.data.Store(key, value)
	return nil
}

// Del removes a key from the store.
func (s *MemoryStore) Del(key string) error {
	s.data.Delete(key)
	return nil
}
