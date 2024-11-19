package cache

import (
	"errors"
	"sync"
	"time"
)

type MemoryStore struct {
	data sync.Map
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Get retrieves a value by key and hash (if given)
func (s *MemoryStore) Get(hash *string, key string) ([]byte, error) {
	if hash != nil {
		hashValue, found := s.data.Load(hash)
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

	value, found := s.data.Load(key)
	if !found {
		return nil, ErrCacheNotFound
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	return bytes, nil
}

// Set stores a key-value pair, ignoring the ttl parameter.
func (s *MemoryStore) Set(hash *string, key string, value []byte, _ time.Duration) error {
	if hash != nil {
		raw, _ := s.data.LoadOrStore(*hash, make(map[string][]byte))

		hashMap, ok := raw.(map[string][]byte)
		if !ok {
			return ErrCacheFailedToSet
		}

		hashMap[key] = value

		s.data.Store(*hash, hashMap)
		return nil
	}

	s.data.Store(key, value)
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

// GetAllForHash retrieves all key-value pairs for a specific hash.
func (s *MemoryStore) GetAllForHash(hash string) (map[string][]byte, error) {
	hashValue, found := s.data.Load(hash)
	if !found {
		return nil, ErrCacheNotFound
	}

	hashMap, ok := hashValue.(map[string][]byte)
	if !ok {
		return nil, ErrCacheFailedToGet
	}

	return hashMap, nil
}

// GetAllForHashAndDelete retrieves all key-value pairs for a specific hash and deletes the hash.
func (s *MemoryStore) GetAllForHashAndDelete(hash string) (map[string][]byte, error) {
	hashMap, err := s.GetAllForHash(hash)
	if err != nil {
		return nil, err
	}

	err = s.Del(nil, hash)
	if err != nil {
		return nil, err
	}

	return hashMap, nil
}

// CountElementsForHash returns the number of elements in a hash in memory.
func (s *MemoryStore) CountElementsForHash(hash string) (int64, error) {
	hashMap, err := s.GetAllForHash(hash)
	if err != nil {
		if errors.Is(err, ErrCacheNotFound) {
			return 0, nil
		}
		return 0, err
	}

	return int64(len(hashMap)), nil
}
