package badgerhold

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/timshannon/badgerhold/v3"

	"github.com/ordishs/gocore"
)

type BadgerHold struct {
	store *badgerhold.Store
	mu    sync.RWMutex
}

type loggerWrapper struct {
	*gocore.Logger
}

func (l loggerWrapper) Warningf(format string, args ...interface{}) {
	l.Warnf(format, args...)
}

var logLevel, _ = gocore.Config().Get("logLevel")
var logger = loggerWrapper{gocore.Log("bdgrh", gocore.NewLogLevelFromString(logLevel))}

func New(dir string) (*BadgerHold, error) {
	options := badgerhold.DefaultOptions

	options.Dir = dir
	options.ValueDir = dir
	if options.Dir == "" {
		folder, _ := gocore.Config().Get("dataFolder", "data")

		f, err := filepath.Abs(path.Join(folder, "metamorph"))
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path: %w", err)
		}

		options.Dir = f
		options.ValueDir = f
	}
	options.Logger = logger

	s, err := badgerhold.Open(options)
	if err != nil {
		return nil, err
	}

	return &BadgerHold{
		store: s,
	}, nil
}

func (s *BadgerHold) Close(_ context.Context) error {
	return s.store.Close()
}

func (s *BadgerHold) Set(_ context.Context, key []byte, value *store.StoreData) error {
	if value.StoredAt.IsZero() {
		value.StoredAt = time.Now()
	}

	if err := s.store.Insert(key, value); err != nil {
		if errors.Is(err, badgerhold.ErrKeyExists) {
			return nil
		}
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}

func (s *BadgerHold) Get(_ context.Context, hash []byte) (*store.StoreData, error) {
	result := &store.StoreData{}

	if err := s.store.Get(hash, result); err != nil {
		if errors.Is(err, badgerhold.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get data: %w", err)
	}

	return result, nil
}

// UpdateStatus attempts to update the status of a transaction
func (s *BadgerHold) UpdateStatus(_ context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error {
	// we need a lock here since we are doing 2 operations that need to be atomic
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.Get(context.Background(), hash)
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
		if err = s.store.Update(hash, tx); err != nil {
			return fmt.Errorf("failed to update data: %w", err)
		}
	}

	return nil
}

// UpdateMined updates the transaction to mined
func (s *BadgerHold) UpdateMined(_ context.Context, hash []byte, blockHash []byte, blockHeight int32) error {
	// we need a lock here since we are doing 2 operations that need to be atomic
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.Get(context.Background(), hash)
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
	if err = s.store.Update(hash, tx); err != nil {
		return fmt.Errorf("failed to update data: %w", err)
	}

	return nil
}

// GetUnseen returns all transactions that have not been seen on the network
func (s *BadgerHold) GetUnseen(_ context.Context, callback func(s *store.StoreData)) error {
	return s.store.ForEach(badgerhold.Where("Status").MatchFunc(func(ra *badgerhold.RecordAccess) (bool, error) {
		field, ok := ra.Field().(metamorph_api.Status)
		if ok {
			return field < metamorph_api.Status_SEEN_ON_NETWORK, nil
		}
		return false, nil
	}), func(s *store.StoreData) error {
		callback(s)
		return nil
	})
}

func (s *BadgerHold) Del(_ context.Context, hash []byte) error {
	result := &store.StoreData{}

	return s.store.Delete(hash, result)
}
