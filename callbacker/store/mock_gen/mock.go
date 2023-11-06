// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mock_gen

import (
	"context"
	"github.com/bitcoin-sv/arc/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/callbacker/store"
	"sync"
)

// Ensure, that StoreMock does implement store.Store.
// If this is not the case, regenerate this file with moq.
var _ store.Store = &StoreMock{}

// StoreMock is a mock implementation of store.Store.
//
//	func TestSomethingThatUsesStore(t *testing.T) {
//
//		// make and configure a mocked store.Store
//		mockedStore := &StoreMock{
//			CloseFunc: func(contextMoqParam context.Context) error {
//				panic("mock out the Close method")
//			},
//			DelFunc: func(ctx context.Context, key string) error {
//				panic("mock out the Del method")
//			},
//			GetFunc: func(ctx context.Context, key string) (*callbacker_api.Callback, error) {
//				panic("mock out the Get method")
//			},
//			GetExpiredFunc: func(contextMoqParam context.Context) (map[string]*callbacker_api.Callback, error) {
//				panic("mock out the GetExpired method")
//			},
//			SetFunc: func(ctx context.Context, callback *callbacker_api.Callback) (string, error) {
//				panic("mock out the Set method")
//			},
//			UpdateExpiryFunc: func(ctx context.Context, key string) error {
//				panic("mock out the UpdateExpiry method")
//			},
//		}
//
//		// use mockedStore in code that requires store.Store
//		// and then make assertions.
//
//	}
type StoreMock struct {
	// CloseFunc mocks the Close method.
	CloseFunc func(contextMoqParam context.Context) error

	// DelFunc mocks the Del method.
	DelFunc func(ctx context.Context, key string) error

	// GetFunc mocks the Get method.
	GetFunc func(ctx context.Context, key string) (*callbacker_api.Callback, error)

	// GetExpiredFunc mocks the GetExpired method.
	GetExpiredFunc func(contextMoqParam context.Context) (map[string]*callbacker_api.Callback, error)

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, callback *callbacker_api.Callback) (string, error)

	// UpdateExpiryFunc mocks the UpdateExpiry method.
	UpdateExpiryFunc func(ctx context.Context, key string) error

	// calls tracks calls to the methods.
	calls struct {
		// Close holds details about calls to the Close method.
		Close []struct {
			// ContextMoqParam is the contextMoqParam argument value.
			ContextMoqParam context.Context
		}
		// Del holds details about calls to the Del method.
		Del []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key string
		}
		// Get holds details about calls to the Get method.
		Get []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key string
		}
		// GetExpired holds details about calls to the GetExpired method.
		GetExpired []struct {
			// ContextMoqParam is the contextMoqParam argument value.
			ContextMoqParam context.Context
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Callback is the callback argument value.
			Callback *callbacker_api.Callback
		}
		// UpdateExpiry holds details about calls to the UpdateExpiry method.
		UpdateExpiry []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Key is the key argument value.
			Key string
		}
	}
	lockClose        sync.RWMutex
	lockDel          sync.RWMutex
	lockGet          sync.RWMutex
	lockGetExpired   sync.RWMutex
	lockSet          sync.RWMutex
	lockUpdateExpiry sync.RWMutex
}

// Close calls CloseFunc.
func (mock *StoreMock) Close(contextMoqParam context.Context) error {
	if mock.CloseFunc == nil {
		panic("StoreMock.CloseFunc: method is nil but Store.Close was just called")
	}
	callInfo := struct {
		ContextMoqParam context.Context
	}{
		ContextMoqParam: contextMoqParam,
	}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	return mock.CloseFunc(contextMoqParam)
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedStore.CloseCalls())
func (mock *StoreMock) CloseCalls() []struct {
	ContextMoqParam context.Context
} {
	var calls []struct {
		ContextMoqParam context.Context
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// Del calls DelFunc.
func (mock *StoreMock) Del(ctx context.Context, key string) error {
	if mock.DelFunc == nil {
		panic("StoreMock.DelFunc: method is nil but Store.Del was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Key string
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
//	len(mockedStore.DelCalls())
func (mock *StoreMock) DelCalls() []struct {
	Ctx context.Context
	Key string
} {
	var calls []struct {
		Ctx context.Context
		Key string
	}
	mock.lockDel.RLock()
	calls = mock.calls.Del
	mock.lockDel.RUnlock()
	return calls
}

// Get calls GetFunc.
func (mock *StoreMock) Get(ctx context.Context, key string) (*callbacker_api.Callback, error) {
	if mock.GetFunc == nil {
		panic("StoreMock.GetFunc: method is nil but Store.Get was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Key string
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
//	len(mockedStore.GetCalls())
func (mock *StoreMock) GetCalls() []struct {
	Ctx context.Context
	Key string
} {
	var calls []struct {
		Ctx context.Context
		Key string
	}
	mock.lockGet.RLock()
	calls = mock.calls.Get
	mock.lockGet.RUnlock()
	return calls
}

// GetExpired calls GetExpiredFunc.
func (mock *StoreMock) GetExpired(contextMoqParam context.Context) (map[string]*callbacker_api.Callback, error) {
	if mock.GetExpiredFunc == nil {
		panic("StoreMock.GetExpiredFunc: method is nil but Store.GetExpired was just called")
	}
	callInfo := struct {
		ContextMoqParam context.Context
	}{
		ContextMoqParam: contextMoqParam,
	}
	mock.lockGetExpired.Lock()
	mock.calls.GetExpired = append(mock.calls.GetExpired, callInfo)
	mock.lockGetExpired.Unlock()
	return mock.GetExpiredFunc(contextMoqParam)
}

// GetExpiredCalls gets all the calls that were made to GetExpired.
// Check the length with:
//
//	len(mockedStore.GetExpiredCalls())
func (mock *StoreMock) GetExpiredCalls() []struct {
	ContextMoqParam context.Context
} {
	var calls []struct {
		ContextMoqParam context.Context
	}
	mock.lockGetExpired.RLock()
	calls = mock.calls.GetExpired
	mock.lockGetExpired.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *StoreMock) Set(ctx context.Context, callback *callbacker_api.Callback) (string, error) {
	if mock.SetFunc == nil {
		panic("StoreMock.SetFunc: method is nil but Store.Set was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		Callback *callbacker_api.Callback
	}{
		Ctx:      ctx,
		Callback: callback,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(ctx, callback)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedStore.SetCalls())
func (mock *StoreMock) SetCalls() []struct {
	Ctx      context.Context
	Callback *callbacker_api.Callback
} {
	var calls []struct {
		Ctx      context.Context
		Callback *callbacker_api.Callback
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}

// UpdateExpiry calls UpdateExpiryFunc.
func (mock *StoreMock) UpdateExpiry(ctx context.Context, key string) error {
	if mock.UpdateExpiryFunc == nil {
		panic("StoreMock.UpdateExpiryFunc: method is nil but Store.UpdateExpiry was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Key string
	}{
		Ctx: ctx,
		Key: key,
	}
	mock.lockUpdateExpiry.Lock()
	mock.calls.UpdateExpiry = append(mock.calls.UpdateExpiry, callInfo)
	mock.lockUpdateExpiry.Unlock()
	return mock.UpdateExpiryFunc(ctx, key)
}

// UpdateExpiryCalls gets all the calls that were made to UpdateExpiry.
// Check the length with:
//
//	len(mockedStore.UpdateExpiryCalls())
func (mock *StoreMock) UpdateExpiryCalls() []struct {
	Ctx context.Context
	Key string
} {
	var calls []struct {
		Ctx context.Context
		Key string
	}
	mock.lockUpdateExpiry.RLock()
	calls = mock.calls.UpdateExpiry
	mock.lockUpdateExpiry.RUnlock()
	return calls
}
