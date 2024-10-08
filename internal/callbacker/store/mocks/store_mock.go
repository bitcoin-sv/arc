// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"sync"
	"time"
)

// Ensure, that CallbackerStoreMock does implement store.CallbackerStore.
// If this is not the case, regenerate this file with moq.
var _ store.CallbackerStore = &CallbackerStoreMock{}

// CallbackerStoreMock is a mock implementation of store.CallbackerStore.
//
//	func TestSomethingThatUsesCallbackerStore(t *testing.T) {
//
//		// make and configure a mocked store.CallbackerStore
//		mockedCallbackerStore := &CallbackerStoreMock{
//			CloseFunc: func() error {
//				panic("mock out the Close method")
//			},
//			DeleteFailedOlderThanFunc: func(ctx context.Context, t time.Time) error {
//				panic("mock out the DeleteFailedOlderThan method")
//			},
//			PopFailedManyFunc: func(ctx context.Context, t time.Time, limit int) ([]*store.CallbackData, error) {
//				panic("mock out the PopFailedMany method")
//			},
//			PopManyFunc: func(ctx context.Context, limit int) ([]*store.CallbackData, error) {
//				panic("mock out the PopMany method")
//			},
//			SetFunc: func(ctx context.Context, dto *store.CallbackData) error {
//				panic("mock out the Set method")
//			},
//			SetManyFunc: func(ctx context.Context, data []*store.CallbackData) error {
//				panic("mock out the SetMany method")
//			},
//		}
//
//		// use mockedCallbackerStore in code that requires store.CallbackerStore
//		// and then make assertions.
//
//	}
type CallbackerStoreMock struct {
	// CloseFunc mocks the Close method.
	CloseFunc func() error

	// DeleteFailedOlderThanFunc mocks the DeleteFailedOlderThan method.
	DeleteFailedOlderThanFunc func(ctx context.Context, t time.Time) error

	// PopFailedManyFunc mocks the PopFailedMany method.
	PopFailedManyFunc func(ctx context.Context, t time.Time, limit int) ([]*store.CallbackData, error)

	// PopManyFunc mocks the PopMany method.
	PopManyFunc func(ctx context.Context, limit int) ([]*store.CallbackData, error)

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, dto *store.CallbackData) error

	// SetManyFunc mocks the SetMany method.
	SetManyFunc func(ctx context.Context, data []*store.CallbackData) error

	// calls tracks calls to the methods.
	calls struct {
		// Close holds details about calls to the Close method.
		Close []struct {
		}
		// DeleteFailedOlderThan holds details about calls to the DeleteFailedOlderThan method.
		DeleteFailedOlderThan []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// T is the t argument value.
			T time.Time
		}
		// PopFailedMany holds details about calls to the PopFailedMany method.
		PopFailedMany []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// T is the t argument value.
			T time.Time
			// Limit is the limit argument value.
			Limit int
		}
		// PopMany holds details about calls to the PopMany method.
		PopMany []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Limit is the limit argument value.
			Limit int
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Dto is the dto argument value.
			Dto *store.CallbackData
		}
		// SetMany holds details about calls to the SetMany method.
		SetMany []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Data is the data argument value.
			Data []*store.CallbackData
		}
	}
	lockClose                 sync.RWMutex
	lockDeleteFailedOlderThan sync.RWMutex
	lockPopFailedMany         sync.RWMutex
	lockPopMany               sync.RWMutex
	lockSet                   sync.RWMutex
	lockSetMany               sync.RWMutex
}

// Close calls CloseFunc.
func (mock *CallbackerStoreMock) Close() error {
	if mock.CloseFunc == nil {
		panic("CallbackerStoreMock.CloseFunc: method is nil but CallbackerStore.Close was just called")
	}
	callInfo := struct {
	}{}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	return mock.CloseFunc()
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedCallbackerStore.CloseCalls())
func (mock *CallbackerStoreMock) CloseCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// DeleteFailedOlderThan calls DeleteFailedOlderThanFunc.
func (mock *CallbackerStoreMock) DeleteFailedOlderThan(ctx context.Context, t time.Time) error {
	if mock.DeleteFailedOlderThanFunc == nil {
		panic("CallbackerStoreMock.DeleteFailedOlderThanFunc: method is nil but CallbackerStore.DeleteFailedOlderThan was just called")
	}
	callInfo := struct {
		Ctx context.Context
		T   time.Time
	}{
		Ctx: ctx,
		T:   t,
	}
	mock.lockDeleteFailedOlderThan.Lock()
	mock.calls.DeleteFailedOlderThan = append(mock.calls.DeleteFailedOlderThan, callInfo)
	mock.lockDeleteFailedOlderThan.Unlock()
	return mock.DeleteFailedOlderThanFunc(ctx, t)
}

// DeleteFailedOlderThanCalls gets all the calls that were made to DeleteFailedOlderThan.
// Check the length with:
//
//	len(mockedCallbackerStore.DeleteFailedOlderThanCalls())
func (mock *CallbackerStoreMock) DeleteFailedOlderThanCalls() []struct {
	Ctx context.Context
	T   time.Time
} {
	var calls []struct {
		Ctx context.Context
		T   time.Time
	}
	mock.lockDeleteFailedOlderThan.RLock()
	calls = mock.calls.DeleteFailedOlderThan
	mock.lockDeleteFailedOlderThan.RUnlock()
	return calls
}

// PopFailedMany calls PopFailedManyFunc.
func (mock *CallbackerStoreMock) PopFailedMany(ctx context.Context, t time.Time, limit int) ([]*store.CallbackData, error) {
	if mock.PopFailedManyFunc == nil {
		panic("CallbackerStoreMock.PopFailedManyFunc: method is nil but CallbackerStore.PopFailedMany was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		T     time.Time
		Limit int
	}{
		Ctx:   ctx,
		T:     t,
		Limit: limit,
	}
	mock.lockPopFailedMany.Lock()
	mock.calls.PopFailedMany = append(mock.calls.PopFailedMany, callInfo)
	mock.lockPopFailedMany.Unlock()
	return mock.PopFailedManyFunc(ctx, t, limit)
}

// PopFailedManyCalls gets all the calls that were made to PopFailedMany.
// Check the length with:
//
//	len(mockedCallbackerStore.PopFailedManyCalls())
func (mock *CallbackerStoreMock) PopFailedManyCalls() []struct {
	Ctx   context.Context
	T     time.Time
	Limit int
} {
	var calls []struct {
		Ctx   context.Context
		T     time.Time
		Limit int
	}
	mock.lockPopFailedMany.RLock()
	calls = mock.calls.PopFailedMany
	mock.lockPopFailedMany.RUnlock()
	return calls
}

// PopMany calls PopManyFunc.
func (mock *CallbackerStoreMock) PopMany(ctx context.Context, limit int) ([]*store.CallbackData, error) {
	if mock.PopManyFunc == nil {
		panic("CallbackerStoreMock.PopManyFunc: method is nil but CallbackerStore.PopMany was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Limit int
	}{
		Ctx:   ctx,
		Limit: limit,
	}
	mock.lockPopMany.Lock()
	mock.calls.PopMany = append(mock.calls.PopMany, callInfo)
	mock.lockPopMany.Unlock()
	return mock.PopManyFunc(ctx, limit)
}

// PopManyCalls gets all the calls that were made to PopMany.
// Check the length with:
//
//	len(mockedCallbackerStore.PopManyCalls())
func (mock *CallbackerStoreMock) PopManyCalls() []struct {
	Ctx   context.Context
	Limit int
} {
	var calls []struct {
		Ctx   context.Context
		Limit int
	}
	mock.lockPopMany.RLock()
	calls = mock.calls.PopMany
	mock.lockPopMany.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *CallbackerStoreMock) Set(ctx context.Context, dto *store.CallbackData) error {
	if mock.SetFunc == nil {
		panic("CallbackerStoreMock.SetFunc: method is nil but CallbackerStore.Set was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Dto *store.CallbackData
	}{
		Ctx: ctx,
		Dto: dto,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(ctx, dto)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedCallbackerStore.SetCalls())
func (mock *CallbackerStoreMock) SetCalls() []struct {
	Ctx context.Context
	Dto *store.CallbackData
} {
	var calls []struct {
		Ctx context.Context
		Dto *store.CallbackData
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}

// SetMany calls SetManyFunc.
func (mock *CallbackerStoreMock) SetMany(ctx context.Context, data []*store.CallbackData) error {
	if mock.SetManyFunc == nil {
		panic("CallbackerStoreMock.SetManyFunc: method is nil but CallbackerStore.SetMany was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Data []*store.CallbackData
	}{
		Ctx:  ctx,
		Data: data,
	}
	mock.lockSetMany.Lock()
	mock.calls.SetMany = append(mock.calls.SetMany, callInfo)
	mock.lockSetMany.Unlock()
	return mock.SetManyFunc(ctx, data)
}

// SetManyCalls gets all the calls that were made to SetMany.
// Check the length with:
//
//	len(mockedCallbackerStore.SetManyCalls())
func (mock *CallbackerStoreMock) SetManyCalls() []struct {
	Ctx  context.Context
	Data []*store.CallbackData
} {
	var calls []struct {
		Ctx  context.Context
		Data []*store.CallbackData
	}
	mock.lockSetMany.RLock()
	calls = mock.calls.SetMany
	mock.lockSetMany.RUnlock()
	return calls
}
