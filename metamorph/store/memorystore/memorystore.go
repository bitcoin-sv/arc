package memorystore

import (
	"context"
	"fmt"
	"sync"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	store2 "github.com/TAAL-GmbH/arc/metamorph/store"
)

type MemoryStore struct {
	mu    sync.RWMutex
	store map[string]*store2.StoreData
}

// New returns a new initialized MemoryStore database implementing the DB
// interface. If the database cannot be initialized, an error will be returned.
func New() (store2.Store, error) {
	return &MemoryStore{
		store: make(map[string]*store2.StoreData),
	}, nil
}

// Get implements the Store interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (m *MemoryStore) Get(ctx context.Context, key []byte) (value *store2.StoreData, err error) {
	hash := store2.HashString(key)

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.store[hash]
	if !ok {
		return nil, store2.ErrNotFound
	}
	return value, nil
}

// GetUnseen returns all transactions that have not been seen on the network
func (m *MemoryStore) GetUnseen(_ context.Context, callback func(s *store2.StoreData)) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.store {
		if v.Status < metamorph_api.Status_SEEN_ON_NETWORK {
			callback(v)
		}
	}

	return nil
}

// UpdateStatus attempts to update the status of a transaction
func (m *MemoryStore) UpdateStatus(_ context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.store[store2.HashString(hash)]
	if !ok {
		return fmt.Errorf("transaction not found")
	}
	// only update the status to later in the life-cycle
	// it is possible to get a SEEN_ON_NETWORK status again, when a block is mined
	if status > tx.Status || rejectReason != "" {
		tx.Status = status
		tx.RejectReason = rejectReason
	}

	return nil
}

func (m *MemoryStore) UpdateMined(_ context.Context, hash []byte, blockHash []byte, blockHeight int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, ok := m.store[store2.HashString(hash)]
	if !ok {
		return fmt.Errorf("transaction not found")
	}

	tx.Status = metamorph_api.Status_MINED
	tx.BlockHash = blockHash
	tx.BlockHeight = blockHeight

	return nil
}

// Set implements the Store interface. It attempts to store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (m *MemoryStore) Set(ctx context.Context, key []byte, value *store2.StoreData) error {
	hash := store2.HashString(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[hash] = value
	return nil
}

func (m *MemoryStore) Del(ctx context.Context, key []byte) (err error) {
	hash := store2.HashString(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.store[hash] = nil
	return nil
}

// Close implements the Store interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (m *MemoryStore) Close(ctx context.Context) error {
	ctx.Done()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.store = make(map[string]*store2.StoreData)
	return nil
}
