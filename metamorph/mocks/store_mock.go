// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
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
//			GetFunc: func(ctx context.Context, key []byte) (*store.StoreData, error) {
//				panic("mock out the Get method")
//			},
//			GetBlockProcessedFunc: func(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
//				panic("mock out the GetBlockProcessed method")
//			},
//			GetUnminedFunc: func(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
//				panic("mock out the GetUnmined method")
//			},
//			PingFunc: func(ctx context.Context) error {
//				panic("mock out the Ping method")
//			},
//			RemoveCallbackerFunc: func(ctx context.Context, hash *chainhash.Hash) error {
//				panic("mock out the RemoveCallbacker method")
//			},
//			SetFunc: func(ctx context.Context, key []byte, value *store.StoreData) error {
//				panic("mock out the Set method")
//			},
//			SetBlockProcessedFunc: func(ctx context.Context, blockHash *chainhash.Hash) error {
//				panic("mock out the SetBlockProcessed method")
//			},
//			SetUnlockedFunc: func(ctx context.Context, hashes []*chainhash.Hash) error {
//				panic("mock out the SetUnlocked method")
//			},
//			SetUnlockedByNameFunc: func(ctx context.Context, lockedBy string) (int64, error) {
//				panic("mock out the SetUnlockedByName method")
//			},
//			UpdateMinedFunc: func(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
//				panic("mock out the UpdateMined method")
//			},
//			UpdateStatusFunc: func(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
//				panic("mock out the UpdateStatus method")
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
	GetFunc func(ctx context.Context, key []byte) (*store.StoreData, error)

	// GetBlockProcessedFunc mocks the GetBlockProcessed method.
	GetBlockProcessedFunc func(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error)

	// GetUnminedFunc mocks the GetUnmined method.
	GetUnminedFunc func(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error)

	// PingFunc mocks the Ping method.
	PingFunc func(ctx context.Context) error

	// RemoveCallbackerFunc mocks the RemoveCallbacker method.
	RemoveCallbackerFunc func(ctx context.Context, hash *chainhash.Hash) error

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, key []byte, value *store.StoreData) error

	// SetBlockProcessedFunc mocks the SetBlockProcessed method.
	SetBlockProcessedFunc func(ctx context.Context, blockHash *chainhash.Hash) error

	// SetUnlockedFunc mocks the SetUnlocked method.
	SetUnlockedFunc func(ctx context.Context, hashes []*chainhash.Hash) error

	// SetUnlockedByNameFunc mocks the SetUnlockedByName method.
	SetUnlockedByNameFunc func(ctx context.Context, lockedBy string) (int64, error)

	// UpdateMinedFunc mocks the UpdateMined method.
	UpdateMinedFunc func(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error

	// UpdateStatusFunc mocks the UpdateStatus method.
	UpdateStatusFunc func(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error

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
		// GetBlockProcessed holds details about calls to the GetBlockProcessed method.
		GetBlockProcessed []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockHash is the blockHash argument value.
			BlockHash *chainhash.Hash
		}
		// GetUnmined holds details about calls to the GetUnmined method.
		GetUnmined []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Since is the since argument value.
			Since time.Time
			// Limit is the limit argument value.
			Limit int64
		}
		// Ping holds details about calls to the Ping method.
		Ping []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// RemoveCallbacker holds details about calls to the RemoveCallbacker method.
		RemoveCallbacker []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key []byte
			// Value is the value argument value.
			Value *store.StoreData
		}
		// SetBlockProcessed holds details about calls to the SetBlockProcessed method.
		SetBlockProcessed []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// BlockHash is the blockHash argument value.
			BlockHash *chainhash.Hash
		}
		// SetUnlocked holds details about calls to the SetUnlocked method.
		SetUnlocked []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hashes is the hashes argument value.
			Hashes []*chainhash.Hash
		}
		// SetUnlockedByName holds details about calls to the SetUnlockedByName method.
		SetUnlockedByName []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// LockedBy is the lockedBy argument value.
			LockedBy string
		}
		// UpdateMined holds details about calls to the UpdateMined method.
		UpdateMined []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// BlockHash is the blockHash argument value.
			BlockHash *chainhash.Hash
			// BlockHeight is the blockHeight argument value.
			BlockHeight uint64
		}
		// UpdateStatus holds details about calls to the UpdateStatus method.
		UpdateStatus []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// Status is the status argument value.
			Status metamorph_api.Status
			// RejectReason is the rejectReason argument value.
			RejectReason string
		}
	}
	lockClearData         sync.RWMutex
	lockClose             sync.RWMutex
	lockDel               sync.RWMutex
	lockGet               sync.RWMutex
	lockGetBlockProcessed sync.RWMutex
	lockGetUnmined        sync.RWMutex
	lockPing              sync.RWMutex
	lockRemoveCallbacker  sync.RWMutex
	lockSet               sync.RWMutex
	lockSetBlockProcessed sync.RWMutex
	lockSetUnlocked       sync.RWMutex
	lockSetUnlockedByName sync.RWMutex
	lockUpdateMined       sync.RWMutex
	lockUpdateStatus      sync.RWMutex
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
func (mock *MetamorphStoreMock) Get(ctx context.Context, key []byte) (*store.StoreData, error) {
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

// GetBlockProcessed calls GetBlockProcessedFunc.
func (mock *MetamorphStoreMock) GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error) {
	if mock.GetBlockProcessedFunc == nil {
		panic("MetamorphStoreMock.GetBlockProcessedFunc: method is nil but MetamorphStore.GetBlockProcessed was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		BlockHash *chainhash.Hash
	}{
		Ctx:       ctx,
		BlockHash: blockHash,
	}
	mock.lockGetBlockProcessed.Lock()
	mock.calls.GetBlockProcessed = append(mock.calls.GetBlockProcessed, callInfo)
	mock.lockGetBlockProcessed.Unlock()
	return mock.GetBlockProcessedFunc(ctx, blockHash)
}

// GetBlockProcessedCalls gets all the calls that were made to GetBlockProcessed.
// Check the length with:
//
//	len(mockedMetamorphStore.GetBlockProcessedCalls())
func (mock *MetamorphStoreMock) GetBlockProcessedCalls() []struct {
	Ctx       context.Context
	BlockHash *chainhash.Hash
} {
	var calls []struct {
		Ctx       context.Context
		BlockHash *chainhash.Hash
	}
	mock.lockGetBlockProcessed.RLock()
	calls = mock.calls.GetBlockProcessed
	mock.lockGetBlockProcessed.RUnlock()
	return calls
}

// GetUnmined calls GetUnminedFunc.
func (mock *MetamorphStoreMock) GetUnmined(ctx context.Context, since time.Time, limit int64) ([]*store.StoreData, error) {
	if mock.GetUnminedFunc == nil {
		panic("MetamorphStoreMock.GetUnminedFunc: method is nil but MetamorphStore.GetUnmined was just called")
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
	mock.lockGetUnmined.Lock()
	mock.calls.GetUnmined = append(mock.calls.GetUnmined, callInfo)
	mock.lockGetUnmined.Unlock()
	return mock.GetUnminedFunc(ctx, since, limit)
}

// GetUnminedCalls gets all the calls that were made to GetUnmined.
// Check the length with:
//
//	len(mockedMetamorphStore.GetUnminedCalls())
func (mock *MetamorphStoreMock) GetUnminedCalls() []struct {
	Ctx   context.Context
	Since time.Time
	Limit int64
} {
	var calls []struct {
		Ctx   context.Context
		Since time.Time
		Limit int64
	}
	mock.lockGetUnmined.RLock()
	calls = mock.calls.GetUnmined
	mock.lockGetUnmined.RUnlock()
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

// RemoveCallbacker calls RemoveCallbackerFunc.
func (mock *MetamorphStoreMock) RemoveCallbacker(ctx context.Context, hash *chainhash.Hash) error {
	if mock.RemoveCallbackerFunc == nil {
		panic("MetamorphStoreMock.RemoveCallbackerFunc: method is nil but MetamorphStore.RemoveCallbacker was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockRemoveCallbacker.Lock()
	mock.calls.RemoveCallbacker = append(mock.calls.RemoveCallbacker, callInfo)
	mock.lockRemoveCallbacker.Unlock()
	return mock.RemoveCallbackerFunc(ctx, hash)
}

// RemoveCallbackerCalls gets all the calls that were made to RemoveCallbacker.
// Check the length with:
//
//	len(mockedMetamorphStore.RemoveCallbackerCalls())
func (mock *MetamorphStoreMock) RemoveCallbackerCalls() []struct {
	Ctx  context.Context
	Hash *chainhash.Hash
} {
	var calls []struct {
		Ctx  context.Context
		Hash *chainhash.Hash
	}
	mock.lockRemoveCallbacker.RLock()
	calls = mock.calls.RemoveCallbacker
	mock.lockRemoveCallbacker.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *MetamorphStoreMock) Set(ctx context.Context, key []byte, value *store.StoreData) error {
	if mock.SetFunc == nil {
		panic("MetamorphStoreMock.SetFunc: method is nil but MetamorphStore.Set was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Key   []byte
		Value *store.StoreData
	}{
		Ctx:   ctx,
		Key:   key,
		Value: value,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(ctx, key, value)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedMetamorphStore.SetCalls())
func (mock *MetamorphStoreMock) SetCalls() []struct {
	Ctx   context.Context
	Key   []byte
	Value *store.StoreData
} {
	var calls []struct {
		Ctx   context.Context
		Key   []byte
		Value *store.StoreData
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}

// SetBlockProcessed calls SetBlockProcessedFunc.
func (mock *MetamorphStoreMock) SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error {
	if mock.SetBlockProcessedFunc == nil {
		panic("MetamorphStoreMock.SetBlockProcessedFunc: method is nil but MetamorphStore.SetBlockProcessed was just called")
	}
	callInfo := struct {
		Ctx       context.Context
		BlockHash *chainhash.Hash
	}{
		Ctx:       ctx,
		BlockHash: blockHash,
	}
	mock.lockSetBlockProcessed.Lock()
	mock.calls.SetBlockProcessed = append(mock.calls.SetBlockProcessed, callInfo)
	mock.lockSetBlockProcessed.Unlock()
	return mock.SetBlockProcessedFunc(ctx, blockHash)
}

// SetBlockProcessedCalls gets all the calls that were made to SetBlockProcessed.
// Check the length with:
//
//	len(mockedMetamorphStore.SetBlockProcessedCalls())
func (mock *MetamorphStoreMock) SetBlockProcessedCalls() []struct {
	Ctx       context.Context
	BlockHash *chainhash.Hash
} {
	var calls []struct {
		Ctx       context.Context
		BlockHash *chainhash.Hash
	}
	mock.lockSetBlockProcessed.RLock()
	calls = mock.calls.SetBlockProcessed
	mock.lockSetBlockProcessed.RUnlock()
	return calls
}

// SetUnlocked calls SetUnlockedFunc.
func (mock *MetamorphStoreMock) SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error {
	if mock.SetUnlockedFunc == nil {
		panic("MetamorphStoreMock.SetUnlockedFunc: method is nil but MetamorphStore.SetUnlocked was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Hashes []*chainhash.Hash
	}{
		Ctx:    ctx,
		Hashes: hashes,
	}
	mock.lockSetUnlocked.Lock()
	mock.calls.SetUnlocked = append(mock.calls.SetUnlocked, callInfo)
	mock.lockSetUnlocked.Unlock()
	return mock.SetUnlockedFunc(ctx, hashes)
}

// SetUnlockedCalls gets all the calls that were made to SetUnlocked.
// Check the length with:
//
//	len(mockedMetamorphStore.SetUnlockedCalls())
func (mock *MetamorphStoreMock) SetUnlockedCalls() []struct {
	Ctx    context.Context
	Hashes []*chainhash.Hash
} {
	var calls []struct {
		Ctx    context.Context
		Hashes []*chainhash.Hash
	}
	mock.lockSetUnlocked.RLock()
	calls = mock.calls.SetUnlocked
	mock.lockSetUnlocked.RUnlock()
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

// UpdateMined calls UpdateMinedFunc.
func (mock *MetamorphStoreMock) UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error {
	if mock.UpdateMinedFunc == nil {
		panic("MetamorphStoreMock.UpdateMinedFunc: method is nil but MetamorphStore.UpdateMined was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		BlockHash   *chainhash.Hash
		BlockHeight uint64
	}{
		Ctx:         ctx,
		Hash:        hash,
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
	}
	mock.lockUpdateMined.Lock()
	mock.calls.UpdateMined = append(mock.calls.UpdateMined, callInfo)
	mock.lockUpdateMined.Unlock()
	return mock.UpdateMinedFunc(ctx, hash, blockHash, blockHeight)
}

// UpdateMinedCalls gets all the calls that were made to UpdateMined.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateMinedCalls())
func (mock *MetamorphStoreMock) UpdateMinedCalls() []struct {
	Ctx         context.Context
	Hash        *chainhash.Hash
	BlockHash   *chainhash.Hash
	BlockHeight uint64
} {
	var calls []struct {
		Ctx         context.Context
		Hash        *chainhash.Hash
		BlockHash   *chainhash.Hash
		BlockHeight uint64
	}
	mock.lockUpdateMined.RLock()
	calls = mock.calls.UpdateMined
	mock.lockUpdateMined.RUnlock()
	return calls
}

// UpdateStatus calls UpdateStatusFunc.
func (mock *MetamorphStoreMock) UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error {
	if mock.UpdateStatusFunc == nil {
		panic("MetamorphStoreMock.UpdateStatusFunc: method is nil but MetamorphStore.UpdateStatus was just called")
	}
	callInfo := struct {
		Ctx          context.Context
		Hash         *chainhash.Hash
		Status       metamorph_api.Status
		RejectReason string
	}{
		Ctx:          ctx,
		Hash:         hash,
		Status:       status,
		RejectReason: rejectReason,
	}
	mock.lockUpdateStatus.Lock()
	mock.calls.UpdateStatus = append(mock.calls.UpdateStatus, callInfo)
	mock.lockUpdateStatus.Unlock()
	return mock.UpdateStatusFunc(ctx, hash, status, rejectReason)
}

// UpdateStatusCalls gets all the calls that were made to UpdateStatus.
// Check the length with:
//
//	len(mockedMetamorphStore.UpdateStatusCalls())
func (mock *MetamorphStoreMock) UpdateStatusCalls() []struct {
	Ctx          context.Context
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	RejectReason string
} {
	var calls []struct {
		Ctx          context.Context
		Hash         *chainhash.Hash
		Status       metamorph_api.Status
		RejectReason string
	}
	mock.lockUpdateStatus.RLock()
	calls = mock.calls.UpdateStatus
	mock.lockUpdateStatus.RUnlock()
	return calls
}
