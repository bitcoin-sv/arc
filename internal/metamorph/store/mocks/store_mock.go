// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"sync"
	"time"
)

// Ensure, that MetamorphStoreMock does implement store.MetamorphStore.
// If this is not the case, regenerate this file with moq.
var _ store.MetamorphStore = &MetamorphStoreMock{}

// MetamorphStoreMock is a mock implementation of store.MetamorphStore.
//
//	func TestSomethingThatUsesMetamorphStore(t *testing.T) {
//
//		// make and configure a mocked store.MetamorphStore
//		mockedMetamorphStore := &MetamorphStoreMock{
//			ClearDataFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
//				panic("mock out the ClearData method")
//			},
//			CloseFunc: func(ctx context.Context) error {
//				panic("mock out the Close method")
//			},
//			DelFunc: func(ctx context.Context, key []byte) error {
//				panic("mock out the Del method")
//			},
//			GetFunc: func(ctx context.Context, key []byte) (*store.Data, error) {
//				panic("mock out the Get method")
//			},
//			GetManyFunc: func(ctx context.Context, keys [][]byte) ([]*store.Data, error) {
//				panic("mock out the GetMany method")
//			},
//			GetRawTxsFunc: func(ctx context.Context, hashes [][]byte) ([][]byte, error) {
//				panic("mock out the GetRawTxs method")
//			},
//			GetSeenOnNetworkFunc: func(ctx context.Context, since time.Time, until time.Time, limit int64, offset int64) ([]*store.Data, error) {
//				panic("mock out the GetSeenOnNetwork method")
//			},
//			GetStatsFunc: func(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*store.Stats, error) {
//				panic("mock out the GetStats method")
//			},
//			GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.Data, error) {
//				panic("mock out the GetUnmined method")
//			},
//			IncrementRetriesFunc: func(ctx context.Context, hash *chainhash.Hash) error {
//				panic("mock out the IncrementRetries method")
//			},
//			PingFunc: func(ctx context.Context) error {
//				panic("mock out the Ping method")
//			},
//			SetFunc: func(ctx context.Context, value *store.Data) error {
//				panic("mock out the Set method")
//			},
//			SetBulkFunc: func(ctx context.Context, data []*store.Data) error {
//				panic("mock out the SetBulk method")
//			},
//			SetLockedFunc: func(ctx context.Context, since time.Time, limit int64) error {
//				panic("mock out the SetLocked method")
//			},
//			SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) {
//				panic("mock out the SetUnlockedByName method")
//			},
//			SetUnlockedByNameExceptFunc: func(ctx context.Context, except []string) (int64, error) {
//				panic("mock out the SetUnlockedByNameExcept method")
//			},
//			UpdateDoubleSpendFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
//				panic("mock out the UpdateDoubleSpend method")
//			},
//			UpdateMinedFunc: func(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*store.Data, error) {
//				panic("mock out the UpdateMined method")
//			},
//			UpdateStatusBulkFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
//				panic("mock out the UpdateStatusBulk method")
//			},
//			UpdateStatusHistoryBulkFunc: func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
//				panic("mock out the UpdateStatusHistoryBulk method")
//			},
//		}
//
//		// use mockedMetamorphStore in code that requires store.MetamorphStore
//		// and then make assertions.
//
//	}
type MetamorphStoreMock struct {
	// ClearDataFunc mocks the ClearData method.
	ClearDataFunc func(ctx context.Context, retentionDays int32) (int64, error)

	// CloseFunc mocks the Close method.
	CloseFunc func(ctx context.Context) error

	// DelFunc mocks the Del method.
	DelFunc func(ctx context.Context, key []byte) error

	// GetFunc mocks the Get method.
	GetFunc func(ctx context.Context, key []byte) (*store.Data, error)

	// GetManyFunc mocks the GetMany method.
	GetManyFunc func(ctx context.Context, keys [][]byte) ([]*store.Data, error)

	// GetRawTxsFunc mocks the GetRawTxs method.
	GetRawTxsFunc func(ctx context.Context, hashes [][]byte) ([][]byte, error)

	// GetSeenOnNetworkFunc mocks the GetSeenOnNetwork method.
	GetSeenOnNetworkFunc func(ctx context.Context, since time.Time, until time.Time, limit int64, offset int64) ([]*store.Data, error)

	// GetStatsFunc mocks the GetStats method.
	GetStatsFunc func(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*store.Stats, error)

	// GetUnminedFunc mocks the GetUnmined method.
	GetUnminedFunc func(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.Data, error)

	// IncrementRetriesFunc mocks the IncrementRetries method.
	IncrementRetriesFunc func(ctx context.Context, hash *chainhash.Hash) error

	// PingFunc mocks the Ping method.
	PingFunc func(ctx context.Context) error

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, value *store.Data) error

	// SetBulkFunc mocks the SetBulk method.
	SetBulkFunc func(ctx context.Context, data []*store.Data) error

	// SetLockedFunc mocks the SetLocked method.
	SetLockedFunc func(ctx context.Context, since time.Time, limit int64) error

	// SetUnlockedByNameFunc mocks the SetUnlockedByName method.
	SetUnlockedByNameFunc func(ctx context.Context, lockedBy string) (int64, error)

	// SetUnlockedByNameExceptFunc mocks the SetUnlockedByNameExcept method.
	SetUnlockedByNameExceptFunc func(ctx context.Context, except []string) (int64, error)

	// UpdateDoubleSpendFunc mocks the UpdateDoubleSpend method.
	UpdateDoubleSpendFunc func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error)

	// UpdateMinedFunc mocks the UpdateMined method.
	UpdateMinedFunc func(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*store.Data, error)

	// UpdateStatusBulkFunc mocks the UpdateStatusBulk method.
	UpdateStatusBulkFunc func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error)

	// UpdateStatusHistoryBulkFunc mocks the UpdateStatusHistoryBulk method.
	UpdateStatusHistoryBulkFunc func(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error)

	// calls tracks calls to the methods.
	calls struct {
		// ClearData holds details about calls to the ClearData method.
		ClearData []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RetentionDays is the retentionDays argument value.
			RetentionDays int32
		}
		// Close holds details about calls to the Close method.
		Close []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// Del holds details about calls to the Del method.
		Del []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key []byte
		}
		// Get holds details about calls to the Get method.
		Get []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key []byte
		}
		// GetMany holds details about calls to the GetMany method.
		GetMany []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Keys is the keys argument value.
			Keys [][]byte
		}
		// GetRawTxs holds details about calls to the GetRawTxs method.
		GetRawTxs []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hashes is the hashes argument value.
			Hashes [][]byte
		}
		// GetSeenOnNetwork holds details about calls to the GetSeenOnNetwork method.
		GetSeenOnNetwork []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Since is the since argument value.
			Since time.Time
			// Until is the until argument value.
			Until time.Time
			// Limit is the limit argument value.
			Limit int64
			// Offset is the offset argument value.
			Offset int64
		}
		// GetStats holds details about calls to the GetStats method.
		GetStats []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Since is the since argument value.
			Since time.Time
			// NotSeenLimit is the notSeenLimit argument value.
			NotSeenLimit time.Duration
			// NotMinedLimit is the notMinedLimit argument value.
			NotMinedLimit time.Duration
		}
		// GetUnmined holds details about calls to the GetUnmined method.
		GetUnmined []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Since is the since argument value.
			Since time.Time
			// Limit is the limit argument value.
			Limit int64
			// Offset is the offset argument value.
			Offset int64
		}
		// IncrementRetries holds details about calls to the IncrementRetries method.
		IncrementRetries []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
		}
		// Ping holds details about calls to the Ping method.
		Ping []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Value is the value argument value.
			Value *store.Data
		}
		// SetBulk holds details about calls to the SetBulk method.
		SetBulk []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Data is the data argument value.
			Data []*store.Data
		}
		// SetLocked holds details about calls to the SetLocked method.
		SetLocked []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Since is the since argument value.
			Since time.Time
			// Limit is the limit argument value.
			Limit int64
		}
		// SetUnlockedByName holds details about calls to the SetUnlockedByName method.
		SetUnlockedByName []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// LockedBy is the lockedBy argument value.
			LockedBy string
		}
		// SetUnlockedByNameExcept holds details about calls to the SetUnlockedByNameExcept method.
		SetUnlockedByNameExcept []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Except is the except argument value.
			Except []string
		}
		// UpdateDoubleSpend holds details about calls to the UpdateDoubleSpend method.
		UpdateDoubleSpend []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Updates is the updates argument value.
			Updates []store.UpdateStatus
		}
		// UpdateMined holds details about calls to the UpdateMined method.
		UpdateMined []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// TxsBlocks is the txsBlocks argument value.
			TxsBlocks []*blocktx_api.TransactionBlock
		}
		// UpdateStatusBulk holds details about calls to the UpdateStatusBulk method.
		UpdateStatusBulk []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Updates is the updates argument value.
			Updates []store.UpdateStatus
		}
		// UpdateStatusHistoryBulk holds details about calls to the UpdateStatusHistoryBulk method.
		UpdateStatusHistoryBulk []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Updates is the updates argument value.
			Updates []store.UpdateStatus
		}
	}
	lockClearData               sync.RWMutex
	lockClose                   sync.RWMutex
	lockDel                     sync.RWMutex
	lockGet                     sync.RWMutex
	lockGetMany                 sync.RWMutex
	lockGetRawTxs               sync.RWMutex
	lockGetSeenOnNetwork        sync.RWMutex
	lockGetStats                sync.RWMutex
	lockGetUnmined              sync.RWMutex
	lockIncrementRetries        sync.RWMutex
	lockPing                    sync.RWMutex
	lockSet                     sync.RWMutex
	lockSetBulk                 sync.RWMutex
	lockSetLocked               sync.RWMutex
	lockSetUnlockedByName       sync.RWMutex
	lockSetUnlockedByNameExcept sync.RWMutex
	lockUpdateDoubleSpend       sync.RWMutex
	lockUpdateMined             sync.RWMutex
	lockUpdateStatusBulk        sync.RWMutex
	lockUpdateStatusHistoryBulk sync.RWMutex
}

// ClearData calls ClearDataFunc.
func (mock *MetamorphStoreMock) ClearData(ctx context.Context, retentionDays int32) (int64, error) {
	if mock.ClearDataFunc == nil {
		panic("MetamorphStoreMock.ClearDataFunc: method is nil but MetamorphStore.ClearData was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		RetentionDays int32
	}{
		Ctx:           ctx,
		RetentionDays: retentionDays,
	}
	mock.lockClearData.Lock()
	mock.calls.ClearData = append(mock.calls.ClearData, callInfo)
	mock.lockClearData.Unlock()
	return mock.ClearDataFunc(ctx, retentionDays)
}

// ClearDataCalls gets all the calls that were made to ClearData.
// Check the length with:
//
//	len(mockedMetamorphStore.ClearDataCalls())
func (mock *MetamorphStoreMock) ClearDataCalls() []struct {
	Ctx           context.Context
	RetentionDays int32
} {
	var calls []struct {
		Ctx           context.Context
		RetentionDays int32
	}
	mock.lockClearData.RLock()
	calls = mock.calls.ClearData
	mock.lockClearData.RUnlock()
	return calls
}

// Close calls CloseFunc.
func (mock *MetamorphStoreMock) Close(ctx context.Context) error {
	if mock.CloseFunc == nil {
		panic("MetamorphStoreMock.CloseFunc: method is nil but MetamorphStore.Close was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	return mock.CloseFunc(ctx)
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedMetamorphStore.CloseCalls())
func (mock *MetamorphStoreMock) CloseCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// Del calls DelFunc.
func (mock *MetamorphStoreMock) Del(ctx context.Context, key []byte) error {
	if mock.DelFunc == nil {
		panic("MetamorphStoreMock.DelFunc: method is nil but MetamorphStore.Del was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Key []byte
	}{
		Ctx: ctx,
		Key: key,
	}
	mock.lockDel.Lock()
	mock.calls.Del = append(mock.calls.Del, callInfo)
	mock.lockDel.Unlock()
	return mock.DelFunc(ctx, key)
}

// DelCalls gets all the calls that were made to Del.
// Check the length with:
//
//	len(mockedMetamorphStore.DelCalls())
func (mock *MetamorphStoreMock) DelCalls() []struct {
	Ctx context.Context
	Key []byte
} {
	var calls []struct {
		Ctx context.Context
		Key []byte
	}
	mock.lockDel.RLock()
	calls = mock.calls.Del
	mock.lockDel.RUnlock()
	return calls
}

// Get calls GetFunc.
func (mock *MetamorphStoreMock) Get(ctx context.Context, key []byte) (*store.Data, error) {
	if mock.GetFunc == nil {
		panic("MetamorphStoreMock.GetFunc: method is nil but MetamorphStore.Get was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Key []byte
	}{
		Ctx: ctx,
		Key: key,
	}
	mock.lockGet.Lock()
	mock.calls.Get = append(mock.calls.Get, callInfo)
	mock.lockGet.Unlock()
	return mock.GetFunc(ctx, key)
}

// GetCalls gets all the calls that were made to Get.
// Check the length with:
//
//	len(mockedMetamorphStore.GetCalls())
func (mock *MetamorphStoreMock) GetCalls() []struct {
	Ctx context.Context
	Key []byte
} {
	var calls []struct {
		Ctx context.Context
		Key []byte
	}
	mock.lockGet.RLock()
	calls = mock.calls.Get
	mock.lockGet.RUnlock()
	return calls
}

// GetMany calls GetManyFunc.
func (mock *MetamorphStoreMock) GetMany(ctx context.Context, keys [][]byte) ([]*store.Data, error) {
	if mock.GetManyFunc == nil {
		panic("MetamorphStoreMock.GetManyFunc: method is nil but MetamorphStore.GetMany was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Keys [][]byte
	}{
		Ctx:  ctx,
		Keys: keys,
	}
	mock.lockGetMany.Lock()
	mock.calls.GetMany = append(mock.calls.GetMany, callInfo)
	mock.lockGetMany.Unlock()
	return mock.GetManyFunc(ctx, keys)
}

// GetManyCalls gets all the calls that were made to GetMany.
// Check the length with:
//
//	len(mockedMetamorphStore.GetManyCalls())
func (mock *MetamorphStoreMock) GetManyCalls() []struct {
	Ctx  context.Context
	Keys [][]byte
} {
	var calls []struct {
		Ctx  context.Context
		Keys [][]byte
	}
	mock.lockGetMany.RLock()
	calls = mock.calls.GetMany
	mock.lockGetMany.RUnlock()
	return calls
}

// GetRawTxs calls GetRawTxsFunc.
func (mock *MetamorphStoreMock) GetRawTxs(ctx context.Context, hashes [][]byte) ([][]byte, error) {
	if mock.GetRawTxsFunc == nil {
		panic("MetamorphStoreMock.GetRawTxsFunc: method is nil but MetamorphStore.GetRawTxs was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Hashes [][]byte
	}{
		Ctx:    ctx,
		Hashes: hashes,
	}
	mock.lockGetRawTxs.Lock()
	mock.calls.GetRawTxs = append(mock.calls.GetRawTxs, callInfo)
	mock.lockGetRawTxs.Unlock()
	return mock.GetRawTxsFunc(ctx, hashes)
}

// GetRawTxsCalls gets all the calls that were made to GetRawTxs.
// Check the length with:
//
//	len(mockedMetamorphStore.GetRawTxsCalls())
func (mock *MetamorphStoreMock) GetRawTxsCalls() []struct {
	Ctx    context.Context
	Hashes [][]byte
} {
	var calls []struct {
		Ctx    context.Context
		Hashes [][]byte
	}
	mock.lockGetRawTxs.RLock()
	calls = mock.calls.GetRawTxs
	mock.lockGetRawTxs.RUnlock()
	return calls
}

// GetSeenOnNetwork calls GetSeenOnNetworkFunc.
func (mock *MetamorphStoreMock) GetSeenOnNetwork(ctx context.Context, since time.Time, until time.Time, limit int64, offset int64) ([]*store.Data, error) {
	if mock.GetSeenOnNetworkFunc == nil {
		panic("MetamorphStoreMock.GetSeenOnNetworkFunc: method is nil but MetamorphStore.GetSeenOnNetwork was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Since  time.Time
		Until  time.Time
		Limit  int64
		Offset int64
	}{
		Ctx:    ctx,
		Since:  since,
		Until:  until,
		Limit:  limit,
		Offset: offset,
	}
	mock.lockGetSeenOnNetwork.Lock()
	mock.calls.GetSeenOnNetwork = append(mock.calls.GetSeenOnNetwork, callInfo)
	mock.lockGetSeenOnNetwork.Unlock()
	return mock.GetSeenOnNetworkFunc(ctx, since, until, limit, offset)
}

// GetSeenOnNetworkCalls gets all the calls that were made to GetSeenOnNetwork.
// Check the length with:
//
//	len(mockedMetamorphStore.GetSeenOnNetworkCalls())
func (mock *MetamorphStoreMock) GetSeenOnNetworkCalls() []struct {
	Ctx    context.Context
	Since  time.Time
	Until  time.Time
	Limit  int64
	Offset int64
} {
	var calls []struct {
		Ctx    context.Context
		Since  time.Time
		Until  time.Time
		Limit  int64
		Offset int64
	}
	mock.lockGetSeenOnNetwork.RLock()
	calls = mock.calls.GetSeenOnNetwork
	mock.lockGetSeenOnNetwork.RUnlock()
	return calls
}

// GetStats calls GetStatsFunc.
func (mock *MetamorphStoreMock) GetStats(ctx context.Context, since time.Time, notSeenLimit time.Duration, notMinedLimit time.Duration) (*store.Stats, error) {
	if mock.GetStatsFunc == nil {
		panic("MetamorphStoreMock.GetStatsFunc: method is nil but MetamorphStore.GetStats was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		Since         time.Time
		NotSeenLimit  time.Duration
		NotMinedLimit time.Duration
	}{
		Ctx:           ctx,
		Since:         since,
		NotSeenLimit:  notSeenLimit,
		NotMinedLimit: notMinedLimit,
	}
	mock.lockGetStats.Lock()
	mock.calls.GetStats = append(mock.calls.GetStats, callInfo)
	mock.lockGetStats.Unlock()
	return mock.GetStatsFunc(ctx, since, notSeenLimit, notMinedLimit)
}

// GetStatsCalls gets all the calls that were made to GetStats.
// Check the length with:
//
//	len(mockedMetamorphStore.GetStatsCalls())
func (mock *MetamorphStoreMock) GetStatsCalls() []struct {
	Ctx           context.Context
	Since         time.Time
	NotSeenLimit  time.Duration
	NotMinedLimit time.Duration
} {
	var calls []struct {
		Ctx           context.Context
		Since         time.Time
		NotSeenLimit  time.Duration
		NotMinedLimit time.Duration
	}
	mock.lockGetStats.RLock()
	calls = mock.calls.GetStats
	mock.lockGetStats.RUnlock()
	return calls
}

// GetUnmined calls GetUnminedFunc.
func (mock *MetamorphStoreMock) GetUnmined(ctx context.Context, since time.Time, limit int64, offset int64) ([]*store.Data, error) {
	if mock.GetUnminedFunc == nil {
		panic("MetamorphStoreMock.GetUnminedFunc: method is nil but MetamorphStore.GetUnmined was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Since  time.Time
		Limit  int64
		Offset int64
	}{
		Ctx:    ctx,
		Since:  since,
		Limit:  limit,
		Offset: offset,
	}
	mock.lockGetUnmined.Lock()
	mock.calls.GetUnmined = append(mock.calls.GetUnmined, callInfo)
	mock.lockGetUnmined.Unlock()
	return mock.GetUnminedFunc(ctx, since, limit, offset)
}

// GetUnminedCalls gets all the calls that were made to GetUnmined.
// Check the length with:
//
//	len(mockedMetamorphStore.GetUnminedCalls())
func (mock *MetamorphStoreMock) GetUnminedCalls() []struct {
	Ctx    context.Context
	Since  time.Time
	Limit  int64
	Offset int64
} {
	var calls []struct {
		Ctx    context.Context
		Since  time.Time
		Limit  int64
		Offset int64
	}
	mock.lockGetUnmined.RLock()
	calls = mock.calls.GetUnmined
	mock.lockGetUnmined.RUnlock()
	return calls
}

// IncrementRetries calls IncrementRetriesFunc.
func (mock *MetamorphStoreMock) IncrementRetries(ctx context.Context, hash *chainhash.Hash) error {
	if mock.IncrementRetriesFunc == nil {
		panic("MetamorphStoreMock.IncrementRetriesFunc: method is nil but MetamorphStore.IncrementRetries was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockIncrementRetries.Lock()
	mock.calls.IncrementRetries = append(mock.calls.IncrementRetries, callInfo)
	mock.lockIncrementRetries.Unlock()
	return mock.IncrementRetriesFunc(ctx, hash)
}

// IncrementRetriesCalls gets all the calls that were made to IncrementRetries.
// Check the length with:
//
//	len(mockedMetamorphStore.IncrementRetriesCalls())
func (mock *MetamorphStoreMock) IncrementRetriesCalls() []struct {
	Ctx  context.Context
	Hash *chainhash.Hash
} {
	var calls []struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}
	mock.lockIncrementRetries.RLock()
	calls = mock.calls.IncrementRetries
	mock.lockIncrementRetries.RUnlock()
	return calls
}

// Ping calls PingFunc.
func (mock *MetamorphStoreMock) Ping(ctx context.Context) error {
	if mock.PingFunc == nil {
		panic("MetamorphStoreMock.PingFunc: method is nil but MetamorphStore.Ping was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockPing.Lock()
	mock.calls.Ping = append(mock.calls.Ping, callInfo)
	mock.lockPing.Unlock()
	return mock.PingFunc(ctx)
}

// PingCalls gets all the calls that were made to Ping.
// Check the length with:
//
//	len(mockedMetamorphStore.PingCalls())
func (mock *MetamorphStoreMock) PingCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockPing.RLock()
	calls = mock.calls.Ping
	mock.lockPing.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *MetamorphStoreMock) Set(ctx context.Context, value *store.Data) error {
	if mock.SetFunc == nil {
		panic("MetamorphStoreMock.SetFunc: method is nil but MetamorphStore.Set was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Value *store.Data
	}{
		Ctx:   ctx,
		Value: value,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(ctx, value)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedMetamorphStore.SetCalls())
func (mock *MetamorphStoreMock) SetCalls() []struct {
	Ctx   context.Context
	Value *store.Data
} {
	var calls []struct {
		Ctx   context.Context
		Value *store.Data
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}

// SetBulk calls SetBulkFunc.
func (mock *MetamorphStoreMock) SetBulk(ctx context.Context, data []*store.Data) error {
	if mock.SetBulkFunc == nil {
		panic("MetamorphStoreMock.SetBulkFunc: method is nil but MetamorphStore.SetBulk was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Data []*store.Data
	}{
		Ctx:  ctx,
		Data: data,
	}
	mock.lockSetBulk.Lock()
	mock.calls.SetBulk = append(mock.calls.SetBulk, callInfo)
	mock.lockSetBulk.Unlock()
	return mock.SetBulkFunc(ctx, data)
}

// SetBulkCalls gets all the calls that were made to SetBulk.
// Check the length with:
//
//	len(mockedMetamorphStore.SetBulkCalls())
func (mock *MetamorphStoreMock) SetBulkCalls() []struct {
	Ctx  context.Context
	Data []*store.Data
} {
	var calls []struct {
		Ctx  context.Context
		Data []*store.Data
	}
	mock.lockSetBulk.RLock()
	calls = mock.calls.SetBulk
	mock.lockSetBulk.RUnlock()
	return calls
}

// SetLocked calls SetLockedFunc.
func (mock *MetamorphStoreMock) SetLocked(ctx context.Context, since time.Time, limit int64) error {
	if mock.SetLockedFunc == nil {
		panic("MetamorphStoreMock.SetLockedFunc: method is nil but MetamorphStore.SetLocked was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Since time.Time
		Limit int64
	}{
		Ctx:   ctx,
		Since: since,
		Limit: limit,
	}
	mock.lockSetLocked.Lock()
	mock.calls.SetLocked = append(mock.calls.SetLocked, callInfo)
	mock.lockSetLocked.Unlock()
	return mock.SetLockedFunc(ctx, since, limit)
}

// SetLockedCalls gets all the calls that were made to SetLocked.
// Check the length with:
//
//	len(mockedMetamorphStore.SetLockedCalls())
func (mock *MetamorphStoreMock) SetLockedCalls() []struct {
	Ctx   context.Context
	Since time.Time
	Limit int64
} {
	var calls []struct {
		Ctx   context.Context
		Since time.Time
		Limit int64
	}
	mock.lockSetLocked.RLock()
	calls = mock.calls.SetLocked
	mock.lockSetLocked.RUnlock()
	return calls
}

// SetUnlockedByName calls SetUnlockedByNameFunc.
func (mock *MetamorphStoreMock) SetUnlockedByName(ctx context.Context, lockedBy string) (int64, error) {
	if mock.SetUnlockedByNameFunc == nil {
		panic("MetamorphStoreMock.SetUnlockedByNameFunc: method is nil but MetamorphStore.SetUnlockedByName was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		LockedBy string
	}{
		Ctx:      ctx,
		LockedBy: lockedBy,
	}
	mock.lockSetUnlockedByName.Lock()
	mock.calls.SetUnlockedByName = append(mock.calls.SetUnlockedByName, callInfo)
	mock.lockSetUnlockedByName.Unlock()
	return mock.SetUnlockedByNameFunc(ctx, lockedBy)
}

// SetUnlockedByNameCalls gets all the calls that were made to SetUnlockedByName.
// Check the length with:
//
//	len(mockedMetamorphStore.SetUnlockedByNameCalls())
func (mock *MetamorphStoreMock) SetUnlockedByNameCalls() []struct {
	Ctx      context.Context
	LockedBy string
} {
	var calls []struct {
		Ctx      context.Context
		LockedBy string
	}
	mock.lockSetUnlockedByName.RLock()
	calls = mock.calls.SetUnlockedByName
	mock.lockSetUnlockedByName.RUnlock()
	return calls
}

// SetUnlockedByNameExcept calls SetUnlockedByNameExceptFunc.
func (mock *MetamorphStoreMock) SetUnlockedByNameExcept(ctx context.Context, except []string) (int64, error) {
	if mock.SetUnlockedByNameExceptFunc == nil {
		panic("MetamorphStoreMock.SetUnlockedByNameExceptFunc: method is nil but MetamorphStore.SetUnlockedByNameExcept was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Except []string
	}{
		Ctx:    ctx,
		Except: except,
	}
	mock.lockSetUnlockedByNameExcept.Lock()
	mock.calls.SetUnlockedByNameExcept = append(mock.calls.SetUnlockedByNameExcept, callInfo)
	mock.lockSetUnlockedByNameExcept.Unlock()
	return mock.SetUnlockedByNameExceptFunc(ctx, except)
}

// SetUnlockedByNameExceptCalls gets all the calls that were made to SetUnlockedByNameExcept.
// Check the length with:
//
//	len(mockedMetamorphStore.SetUnlockedByNameExceptCalls())
func (mock *MetamorphStoreMock) SetUnlockedByNameExceptCalls() []struct {
	Ctx    context.Context
	Except []string
} {
	var calls []struct {
		Ctx    context.Context
		Except []string
	}
	mock.lockSetUnlockedByNameExcept.RLock()
	calls = mock.calls.SetUnlockedByNameExcept
	mock.lockSetUnlockedByNameExcept.RUnlock()
	return calls
}

// UpdateDoubleSpend calls UpdateDoubleSpendFunc.
func (mock *MetamorphStoreMock) UpdateDoubleSpend(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
	if mock.UpdateDoubleSpendFunc == nil {
		panic("MetamorphStoreMock.UpdateDoubleSpendFunc: method is nil but MetamorphStore.UpdateDoubleSpend was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}{
		Ctx:     ctx,
		Updates: updates,
	}
	mock.lockUpdateDoubleSpend.Lock()
	mock.calls.UpdateDoubleSpend = append(mock.calls.UpdateDoubleSpend, callInfo)
	mock.lockUpdateDoubleSpend.Unlock()
	return mock.UpdateDoubleSpendFunc(ctx, updates)
}

// UpdateDoubleSpendCalls gets all the calls that were made to UpdateDoubleSpend.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateDoubleSpendCalls())
func (mock *MetamorphStoreMock) UpdateDoubleSpendCalls() []struct {
	Ctx     context.Context
	Updates []store.UpdateStatus
} {
	var calls []struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}
	mock.lockUpdateDoubleSpend.RLock()
	calls = mock.calls.UpdateDoubleSpend
	mock.lockUpdateDoubleSpend.RUnlock()
	return calls
}

// UpdateMined calls UpdateMinedFunc.
func (mock *MetamorphStoreMock) UpdateMined(ctx context.Context, txsBlocks []*blocktx_api.TransactionBlock) ([]*store.Data, error) {
	if mock.UpdateMinedFunc == nil {
		panic("MetamorphStoreMock.UpdateMinedFunc: method is nil but MetamorphStore.UpdateMined was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		TxsBlocks []*blocktx_api.TransactionBlock
	}{
		Ctx:       ctx,
		TxsBlocks: txsBlocks,
	}
	mock.lockUpdateMined.Lock()
	mock.calls.UpdateMined = append(mock.calls.UpdateMined, callInfo)
	mock.lockUpdateMined.Unlock()
	return mock.UpdateMinedFunc(ctx, txsBlocks)
}

// UpdateMinedCalls gets all the calls that were made to UpdateMined.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateMinedCalls())
func (mock *MetamorphStoreMock) UpdateMinedCalls() []struct {
	Ctx       context.Context
	TxsBlocks []*blocktx_api.TransactionBlock
} {
	var calls []struct {
		Ctx       context.Context
		TxsBlocks []*blocktx_api.TransactionBlock
	}
	mock.lockUpdateMined.RLock()
	calls = mock.calls.UpdateMined
	mock.lockUpdateMined.RUnlock()
	return calls
}

// UpdateStatusBulk calls UpdateStatusBulkFunc.
func (mock *MetamorphStoreMock) UpdateStatusBulk(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
	if mock.UpdateStatusBulkFunc == nil {
		panic("MetamorphStoreMock.UpdateStatusBulkFunc: method is nil but MetamorphStore.UpdateStatusBulk was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}{
		Ctx:     ctx,
		Updates: updates,
	}
	mock.lockUpdateStatusBulk.Lock()
	mock.calls.UpdateStatusBulk = append(mock.calls.UpdateStatusBulk, callInfo)
	mock.lockUpdateStatusBulk.Unlock()
	return mock.UpdateStatusBulkFunc(ctx, updates)
}

// UpdateStatusBulkCalls gets all the calls that were made to UpdateStatusBulk.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateStatusBulkCalls())
func (mock *MetamorphStoreMock) UpdateStatusBulkCalls() []struct {
	Ctx     context.Context
	Updates []store.UpdateStatus
} {
	var calls []struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}
	mock.lockUpdateStatusBulk.RLock()
	calls = mock.calls.UpdateStatusBulk
	mock.lockUpdateStatusBulk.RUnlock()
	return calls
}

// UpdateStatusHistoryBulk calls UpdateStatusHistoryBulkFunc.
func (mock *MetamorphStoreMock) UpdateStatusHistoryBulk(ctx context.Context, updates []store.UpdateStatus) ([]*store.Data, error) {
	if mock.UpdateStatusHistoryBulkFunc == nil {
		panic("MetamorphStoreMock.UpdateStatusHistoryBulkFunc: method is nil but MetamorphStore.UpdateStatusHistoryBulk was just called")
	}
	callInfo := struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}{
		Ctx:     ctx,
		Updates: updates,
	}
	mock.lockUpdateStatusHistoryBulk.Lock()
	mock.calls.UpdateStatusHistoryBulk = append(mock.calls.UpdateStatusHistoryBulk, callInfo)
	mock.lockUpdateStatusHistoryBulk.Unlock()
	return mock.UpdateStatusHistoryBulkFunc(ctx, updates)
}

// UpdateStatusHistoryBulkCalls gets all the calls that were made to UpdateStatusHistoryBulk.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateStatusHistoryBulkCalls())
func (mock *MetamorphStoreMock) UpdateStatusHistoryBulkCalls() []struct {
	Ctx     context.Context
	Updates []store.UpdateStatus
} {
	var calls []struct {
		Ctx     context.Context
		Updates []store.UpdateStatus
	}
	mock.lockUpdateStatusHistoryBulk.RLock()
	calls = mock.calls.UpdateStatusHistoryBulk
	mock.lockUpdateStatusHistoryBulk.RUnlock()
	return calls
}
