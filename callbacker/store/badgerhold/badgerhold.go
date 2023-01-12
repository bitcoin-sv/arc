package badgerhold

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/callbacker/callbacker_api"
	"github.com/TAAL-GmbH/arc/callbacker/store"
	"github.com/labstack/gommon/random"
	"github.com/timshannon/badgerhold/v3"

	"github.com/ordishs/gocore"
)

type BadgerData struct {
	Key           string
	CallbackAfter time.Time `badgerhold:"index"`
	CallbackCount int
	Hash          []byte
	Url           string
	Token         string
	Status        int32
}

type BadgerHold struct {
	store              *badgerhold.Store
	mu                 sync.RWMutex
	maxCallbackRetries int
	interval           time.Duration
}

type loggerWrapper struct {
	*gocore.Logger
}

func (l loggerWrapper) Warningf(format string, args ...interface{}) {
	l.Warnf(format, args...)
}

var logger = loggerWrapper{gocore.Log("cbdgr")}

func New(dir string, interval time.Duration) (*BadgerHold, error) {
	options := badgerhold.DefaultOptions

	options.Dir = dir
	options.ValueDir = dir
	if options.Dir == "" {
		options.Dir = "data"
		options.ValueDir = "data"
	}
	options.Logger = logger

	s, err := badgerhold.Open(options)
	if err != nil {
		return nil, err
	}

	return &BadgerHold{
		store:              s,
		maxCallbackRetries: 32,
		interval:           interval,
	}, nil
}

func (bh *BadgerHold) Get(_ context.Context, key string) (*callbacker_api.Callback, error) {
	result := &BadgerData{}

	if err := bh.store.Get(key, result); err != nil {
		if errors.Is(err, badgerhold.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get data: %w", err)
	}

	return &callbacker_api.Callback{
		Hash:   result.Hash,
		Url:    result.Url,
		Token:  result.Token,
		Status: result.Status,
	}, nil
}

func (bh *BadgerHold) GetExpired(_ context.Context) (map[string]*callbacker_api.Callback, error) {
	var result []*BadgerData

	if err := bh.store.Find(&result, badgerhold.Where("CallbackAfter").Lt(time.Now())); err != nil {
		return nil, fmt.Errorf("failed to get data: %w", err)
	}

	callbacks := make(map[string]*callbacker_api.Callback)
	for _, callback := range result {
		callbacks[callback.Key] = &callbacker_api.Callback{
			Hash:   callback.Hash,
			Url:    callback.Url,
			Token:  callback.Token,
			Status: callback.Status,
		}
	}

	return callbacks, nil
}

func (bh *BadgerHold) Set(_ context.Context, callback *callbacker_api.Callback) (string, error) {
	if callback == nil {
		return "", fmt.Errorf("callback is nil")
	}

	key := random.String(32)
	callbackAfter := time.Now().Add(bh.interval)
	value := BadgerData{
		Key:           key,
		CallbackAfter: callbackAfter,
		CallbackCount: 0,
		Hash:          callback.Hash,
		Url:           callback.Url,
		Token:         callback.Token,
		Status:        callback.Status,
	}
	if err := bh.store.Upsert(key, value); err != nil {
		return "", fmt.Errorf("failed to insert data: %w", err)
	}
	return key, nil
}

func (bh *BadgerHold) UpdateExpiry(_ context.Context, key string) error {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	data := &BadgerData{}
	if err := bh.store.Get(key, data); err != nil {
		if errors.Is(err, badgerhold.ErrNotFound) {
			return store.ErrNotFound
		}
		return fmt.Errorf("failed to get data: %w", err)
	}

	data.CallbackCount++
	if data.CallbackCount > bh.maxCallbackRetries {
		if err := bh.Del(context.Background(), key); err != nil {
			return fmt.Errorf("failed to delete data: %w", err)
		}
		return store.ErrMaxRetries
	}

	nextTry := bh.incrementInterval(bh.interval, data.CallbackCount)
	data.CallbackAfter = time.Now().Add(nextTry)

	if err := bh.store.Update(key, data); err != nil {
		return fmt.Errorf("failed to update data: %w", err)
	}

	return nil
}

func (bh *BadgerHold) Del(_ context.Context, key string) error {
	result := &BadgerData{}
	return bh.store.Delete(key, result)
}

func (bh *BadgerHold) Close(_ context.Context) error {
	return bh.store.Close()
}

func (bh *BadgerHold) incrementInterval(duration time.Duration, count int) time.Duration {
	return duration * time.Duration(math.Pow(2, float64(count)))
}
