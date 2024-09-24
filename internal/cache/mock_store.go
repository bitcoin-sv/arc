package cache

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockCacheStore is a mock type for CacheStore.
type MockCacheStore struct {
	mock.Mock
}

// Get returns value by key.
func (m *MockCacheStore) Get(key string) ([]byte, error) {
	args := m.Called(key)
	return args.Get(0).([]byte), args.Error(1)
}

// Set sets value by key.
func (m *MockCacheStore) Set(key string, value []byte, ttl time.Duration) error {
	args := m.Called(key, value, ttl)
	return args.Error(0)
}

// Del deletes value by key.
func (m *MockCacheStore) Del(key string) error {
	args := m.Called(key)
	return args.Error(0)
}
