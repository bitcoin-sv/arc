package mock

import (
	"context"
	"sync"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
	"github.com/labstack/gommon/random"
)

type Store struct {
	data map[string]*callbacker_api.Callback
	mu   sync.Mutex
}

func New() (*Store, error) {
	return &Store{
		data: make(map[string]*callbacker_api.Callback),
	}, nil
}

func (s *Store) Get(_ context.Context, key string) (*callbacker_api.Callback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	callback, ok := s.data[key]
	if !ok {
		return nil, store.ErrNotFound
	}

	return callback, nil
}

func (s *Store) GetExpired(_ context.Context) (map[string]*callbacker_api.Callback, error) {
	return nil, nil
}

func (s *Store) Set(_ context.Context, callback *callbacker_api.Callback) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := random.String(32)
	s.data[key] = callback
	return key, nil
}

func (s *Store) UpdateExpiry(_ context.Context, key string) error {
	return nil
}

func (s *Store) Del(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

func (s *Store) Close(_ context.Context) error {
	return nil
}
