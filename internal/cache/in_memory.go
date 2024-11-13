package cache

import (
	"sync"
	"time"
)

type MemoryStore struct {
	data map[string][]byte
	mu   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

// Get retrieves a value by key. It returns an error if the key does not exist.
func (s *MemoryStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, found := s.data[key]
	if !found {
		return nil, ErrCacheNotFound
	}
	return value, nil
}

// Set stores a key-value pair, ignoring the ttl parameter.
func (s *MemoryStore) Set(key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return nil
}

// Del removes a key from the store.
func (s *MemoryStore) Del(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.data[key]; !found {
		return ErrCacheNotFound
	}
	delete(s.data, key)
	return nil
}
