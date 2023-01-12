package badger

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/dgraph-io/badger/v3"
	"github.com/ordishs/gocore"
)

type Badger struct {
	store *badger.DB
	mu    sync.RWMutex
}

type loggerWrapper struct {
	*gocore.Logger
}

func (l loggerWrapper) Warningf(format string, args ...interface{}) {
	l.Warnf(format, args...)
}

var logger = loggerWrapper{gocore.Log("bdgr")}

func New(dir string) (*Badger, error) {
	opts := badger.DefaultOptions(dir).
		WithLogger(logger).
		WithLoggingLevel(badger.ERROR).WithNumMemtables(32)
	s, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Badger{
		store: s,
	}, nil
}

func (s *Badger) Close(_ context.Context) error {
	return s.store.Close()
}

func (s *Badger) Set(_ context.Context, key []byte, value *store.StoreData) error {
	if value.StoredAt.IsZero() {
		value.StoredAt = time.Now()
	}

	var data bytes.Buffer
	enc := gob.NewEncoder(&data)
	err := enc.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	if err = s.store.Update(func(tx *badger.Txn) error {
		return tx.Set(key, data.Bytes())
	}); err != nil {
		return fmt.Errorf("failed to set data: %w", err)
	}

	return nil
}

func (s *Badger) Get(_ context.Context, hash []byte) (*store.StoreData, error) {
	var result *store.StoreData

	err := s.store.View(func(tx *badger.Txn) error {
		data, err := tx.Get(hash)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return store.ErrNotFound
			}
			return err
		}

		if err = data.Value(func(val []byte) error {
			dec := gob.NewDecoder(bytes.NewReader(val))
			return dec.Decode(&result)
		}); err != nil {
			return fmt.Errorf("failed to decode data: %w", err)
		}

		return nil
	})

	return result, err
}

// UpdateStatus attempts to update the status of a transaction
func (s *Badger) UpdateStatus(ctx context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error {
	// we need a lock here since we are doing 2 operations that need to be atomic
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.Get(ctx, hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// no need to update status if we don't have the transaction
			// we also shouldn't need to return an error here
			return nil
		}
		return err
	}

	// only update the status to later in the life-cycle
	// it is possible to get a SEEN_ON_NETWORK status again, when a block is mined
	if status > tx.Status || rejectReason != "" {
		tx.Status = status
		tx.RejectReason = rejectReason
		if err = s.Set(ctx, hash, tx); err != nil {
			return fmt.Errorf("failed to update data: %w", err)
		}
	}

	return nil
}

// UpdateMined updates the transaction to mined
func (s *Badger) UpdateMined(ctx context.Context, hash []byte, blockHash []byte, blockHeight int32) error {
	// we need a lock here since we are doing 2 operations that need to be atomic
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.Get(ctx, hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// no need to update status if we don't have the transaction
			// we also shouldn't need to return an error here
			return nil
		}
		return err
	}

	tx.Status = metamorph_api.Status_MINED
	tx.BlockHash = blockHash
	tx.BlockHeight = blockHeight
	if err = s.Set(ctx, hash, tx); err != nil {
		return fmt.Errorf("failed to update data: %w", err)
	}

	return nil
}

// GetUnseen returns all transactions that have not been seen on the network
func (s *Badger) GetUnseen(_ context.Context, callback func(s *store.StoreData)) error {
	return s.store.View(func(tx *badger.Txn) error {
		iter := tx.NewIterator(badger.DefaultIteratorOptions)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			if item.IsDeletedOrExpired() {
				continue
			}

			var result *store.StoreData
			if err := item.Value(func(val []byte) error {
				dec := gob.NewDecoder(bytes.NewReader(val))
				return dec.Decode(&result)
			}); err != nil {
				return fmt.Errorf("failed to decode data: %w", err)
			}

			if result.Status < metamorph_api.Status_SEEN_ON_NETWORK {
				callback(result)
			}
		}

		return nil
	})
}

func (s *Badger) Del(_ context.Context, hash []byte) error {
	return s.store.Update(func(tx *badger.Txn) error {
		return tx.Delete(hash)
	})
}
