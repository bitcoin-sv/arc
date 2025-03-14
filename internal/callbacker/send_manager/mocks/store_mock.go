// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/callbacker/send_manager"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"sync"
)

// Ensure, that SendManagerStoreMock does implement send_manager.SendManagerStore.
// If this is not the case, regenerate this file with moq.
var _ send_manager.SendManagerStore = &SendManagerStoreMock{}

// SendManagerStoreMock is a mock implementation of send_manager.SendManagerStore.
//
//	func TestSomethingThatUsesSendManagerStore(t *testing.T) {
//
//		// make and configure a mocked send_manager.SendManagerStore
//		mockedSendManagerStore := &SendManagerStoreMock{
//			GetAndDeleteFunc: func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
//				panic("mock out the GetAndDelete method")
//			},
//			SetFunc: func(ctx context.Context, dto *store.CallbackData) error {
//				panic("mock out the Set method")
//			},
//			SetManyFunc: func(ctx context.Context, data []*store.CallbackData) error {
//				panic("mock out the SetMany method")
//			},
//		}
//
//		// use mockedSendManagerStore in code that requires send_manager.SendManagerStore
//		// and then make assertions.
//
//	}
type SendManagerStoreMock struct {
	// GetAndDeleteFunc mocks the GetAndDelete method.
	GetAndDeleteFunc func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error)

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, dto *store.CallbackData) error

	// SetManyFunc mocks the SetMany method.
	SetManyFunc func(ctx context.Context, data []*store.CallbackData) error

	// calls tracks calls to the methods.
	calls struct {
		// GetAndDelete holds details about calls to the GetAndDelete method.
		GetAndDelete []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// URL is the url argument value.
			URL string
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
	lockGetAndDelete sync.RWMutex
	lockSet          sync.RWMutex
	lockSetMany      sync.RWMutex
}

// GetAndDelete calls GetAndDeleteFunc.
func (mock *SendManagerStoreMock) GetAndDelete(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
	if mock.GetAndDeleteFunc == nil {
		panic("SendManagerStoreMock.GetAndDeleteFunc: method is nil but SendManagerStore.GetAndDelete was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		URL   string
		Limit int
	}{
		Ctx:   ctx,
		URL:   url,
		Limit: limit,
	}
	mock.lockGetAndDelete.Lock()
	mock.calls.GetAndDelete = append(mock.calls.GetAndDelete, callInfo)
	mock.lockGetAndDelete.Unlock()
	return mock.GetAndDeleteFunc(ctx, url, limit)
}

// GetAndDeleteCalls gets all the calls that were made to GetAndDelete.
// Check the length with:
//
//	len(mockedSendManagerStore.GetAndDeleteCalls())
func (mock *SendManagerStoreMock) GetAndDeleteCalls() []struct {
	Ctx   context.Context
	URL   string
	Limit int
} {
	var calls []struct {
		Ctx   context.Context
		URL   string
		Limit int
	}
	mock.lockGetAndDelete.RLock()
	calls = mock.calls.GetAndDelete
	mock.lockGetAndDelete.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *SendManagerStoreMock) Set(ctx context.Context, dto *store.CallbackData) error {
	if mock.SetFunc == nil {
		panic("SendManagerStoreMock.SetFunc: method is nil but SendManagerStore.Set was just called")
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
//	len(mockedSendManagerStore.SetCalls())
func (mock *SendManagerStoreMock) SetCalls() []struct {
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
func (mock *SendManagerStoreMock) SetMany(ctx context.Context, data []*store.CallbackData) error {
	if mock.SetManyFunc == nil {
		panic("SendManagerStoreMock.SetManyFunc: method is nil but SendManagerStore.SetMany was just called")
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
//	len(mockedSendManagerStore.SetManyCalls())
func (mock *SendManagerStoreMock) SetManyCalls() []struct {
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
