package memorystore

import (
	"context"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type MemoryStore struct {
	mu sync.RWMutex
	// the memory store is mainly used for testing, so we don't need to worry about this being public
	Store map[string]*store.StoreData
}

// New returns a new initialized MemoryStore database implementing the DB
// interface. If the database cannot be initialized, an error will be returned.
func New() (*MemoryStore, error) {
	return &MemoryStore{
		Store: make(map[string]*store.StoreData),
	}, nil
}

// Get implements the Store interface. It attempts to get a value for a given key.
// If the key does not exist an error is returned, otherwise the retrieved value.
func (m *MemoryStore) Get(_ context.Context, key []byte) (*store.StoreData, error) {
	hash := store.HashString(key)

	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.Store[hash]
	if !ok {
		return nil, store.ErrNotFound
	}
	return value, nil
}

// GetUnmined returns all transactions that have not been seen on the network returns all transactions that have not been seen on the network
func (m *MemoryStore) GetUnmined(_ context.Context, callback func(s *store.StoreData)) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.Store {
		if v.Status < metamorph_api.Status_SEEN_ON_NETWORK {
			callback(v)
		}
	}

	return nil
}

// UpdateStatus attempts to update the status of a transaction
func (m *MemoryStore) UpdateStatus(_ context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := store.HashString(hash[:])

	tx, ok := m.Store[hashKey]
	if !ok {
		// no need to return an error if not found when updating status
		return nil
	}
	// only update the status to later in the life-cycle
	// it is possible to get a SEEN_ON_NETWORK status again, when a block is mined
	if status > tx.Status || rejectReason != "" {
		tx.Status = status
		tx.RejectReason = rejectReason
	}

	return nil
}

func (m *MemoryStore) UpdateMined(_ context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := store.HashString(hash[:])

	tx, ok := m.Store[hashKey]
	if !ok {
		// no need to return an error if not found when updating status
		return nil
	}

	tx.Status = metamorph_api.Status_MINED
	tx.BlockHash = blockHash
	tx.BlockHeight = blockHeight

	return nil
}

// Set implements the Store interface. It attempts to Store a value for a given key
// and namespace. If the key/value pair cannot be saved, an error is returned.
func (m *MemoryStore) Set(_ context.Context, key []byte, value *store.StoreData) error {
	hash := store.HashString(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Store[hash] = value
	return nil
}

func (m *MemoryStore) Del(_ context.Context, key []byte) (err error) {
	hash := store.HashString(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Store[hash] = nil
	return nil
}

func (m *MemoryStore) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemoryStore) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	//TODO implement me
	panic("implement me")
}

// Close implements the Store interface. It closes the connection to the underlying
// MemoryStore database as well as invoking the context's cancel function.
func (m *MemoryStore) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Store = make(map[string]*store.StoreData)
	return nil
}
