// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/callbacker/store"
	"sync"
	"time"
)

// Ensure, that ProcessorStoreMock does implement store.ProcessorStore.
// If this is not the case, regenerate this file with moq.
var _ store.ProcessorStore = &ProcessorStoreMock{}

// ProcessorStoreMock is a mock implementation of store.ProcessorStore.
//
//	func TestSomethingThatUsesProcessorStore(t *testing.T) {
//
//		// make and configure a mocked store.ProcessorStore
//		mockedProcessorStore := &ProcessorStoreMock{
//			DeleteOlderThanFunc: func(ctx context.Context, t time.Time) error {
//				panic("mock out the DeleteOlderThan method")
//			},
//			DeleteURLMappingFunc: func(ctx context.Context, instance string) (int64, error) {
//				panic("mock out the DeleteURLMapping method")
//			},
//			GetAndDeleteFunc: func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
//				panic("mock out the GetAndDelete method")
//			},
//			GetURLMappingsFunc: func(ctx context.Context) (map[string]string, error) {
//				panic("mock out the GetURLMappings method")
//			},
//			GetUnmappedURLFunc: func(ctx context.Context) (string, error) {
//				panic("mock out the GetUnmappedURL method")
//			},
//			SetURLMappingFunc: func(ctx context.Context, m store.URLMapping) error {
//				panic("mock out the SetURLMapping method")
//			},
//		}
//
//		// use mockedProcessorStore in code that requires store.ProcessorStore
//		// and then make assertions.
//
//	}
type ProcessorStoreMock struct {
	// DeleteOlderThanFunc mocks the DeleteOlderThan method.
	DeleteOlderThanFunc func(ctx context.Context, t time.Time) error

	// DeleteURLMappingFunc mocks the DeleteURLMapping method.
	DeleteURLMappingFunc func(ctx context.Context, instance string) (int64, error)

	// GetAndDeleteFunc mocks the GetAndDelete method.
	GetAndDeleteFunc func(ctx context.Context, url string, limit int) ([]*store.CallbackData, error)

	// GetURLMappingsFunc mocks the GetURLMappings method.
	GetURLMappingsFunc func(ctx context.Context) (map[string]string, error)

	// GetUnmappedURLFunc mocks the GetUnmappedURL method.
	GetUnmappedURLFunc func(ctx context.Context) (string, error)

	// SetURLMappingFunc mocks the SetURLMapping method.
	SetURLMappingFunc func(ctx context.Context, m store.URLMapping) error

	// calls tracks calls to the methods.
	calls struct {
		// DeleteOlderThan holds details about calls to the DeleteOlderThan method.
		DeleteOlderThan []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// T is the t argument value.
			T time.Time
		}
		// DeleteURLMapping holds details about calls to the DeleteURLMapping method.
		DeleteURLMapping []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Instance is the instance argument value.
			Instance string
		}
		// GetAndDelete holds details about calls to the GetAndDelete method.
		GetAndDelete []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// URL is the url argument value.
			URL string
			// Limit is the limit argument value.
			Limit int
		}
		// GetURLMappings holds details about calls to the GetURLMappings method.
		GetURLMappings []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// GetUnmappedURL holds details about calls to the GetUnmappedURL method.
		GetUnmappedURL []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// SetURLMapping holds details about calls to the SetURLMapping method.
		SetURLMapping []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// M is the m argument value.
			M store.URLMapping
		}
	}
	lockDeleteOlderThan  sync.RWMutex
	lockDeleteURLMapping sync.RWMutex
	lockGetAndDelete     sync.RWMutex
	lockGetURLMappings   sync.RWMutex
	lockGetUnmappedURL   sync.RWMutex
	lockSetURLMapping    sync.RWMutex
}

// DeleteOlderThan calls DeleteOlderThanFunc.
func (mock *ProcessorStoreMock) DeleteOlderThan(ctx context.Context, t time.Time) error {
	if mock.DeleteOlderThanFunc == nil {
		panic("ProcessorStoreMock.DeleteOlderThanFunc: method is nil but ProcessorStore.DeleteOlderThan was just called")
	}
	callInfo := struct {
		Ctx context.Context
		T   time.Time
	}{
		Ctx: ctx,
		T:   t,
	}
	mock.lockDeleteOlderThan.Lock()
	mock.calls.DeleteOlderThan = append(mock.calls.DeleteOlderThan, callInfo)
	mock.lockDeleteOlderThan.Unlock()
	return mock.DeleteOlderThanFunc(ctx, t)
}

// DeleteOlderThanCalls gets all the calls that were made to DeleteOlderThan.
// Check the length with:
//
//	len(mockedProcessorStore.DeleteOlderThanCalls())
func (mock *ProcessorStoreMock) DeleteOlderThanCalls() []struct {
	Ctx context.Context
	T   time.Time
} {
	var calls []struct {
		Ctx context.Context
		T   time.Time
	}
	mock.lockDeleteOlderThan.RLock()
	calls = mock.calls.DeleteOlderThan
	mock.lockDeleteOlderThan.RUnlock()
	return calls
}

// DeleteURLMapping calls DeleteURLMappingFunc.
func (mock *ProcessorStoreMock) DeleteURLMapping(ctx context.Context, instance string) (int64, error) {
	if mock.DeleteURLMappingFunc == nil {
		panic("ProcessorStoreMock.DeleteURLMappingFunc: method is nil but ProcessorStore.DeleteURLMapping was just called")
	}
	callInfo := struct {
		Ctx      context.Context
		Instance string
	}{
		Ctx:      ctx,
		Instance: instance,
	}
	mock.lockDeleteURLMapping.Lock()
	mock.calls.DeleteURLMapping = append(mock.calls.DeleteURLMapping, callInfo)
	mock.lockDeleteURLMapping.Unlock()
	return mock.DeleteURLMappingFunc(ctx, instance)
}

// DeleteURLMappingCalls gets all the calls that were made to DeleteURLMapping.
// Check the length with:
//
//	len(mockedProcessorStore.DeleteURLMappingCalls())
func (mock *ProcessorStoreMock) DeleteURLMappingCalls() []struct {
	Ctx      context.Context
	Instance string
} {
	var calls []struct {
		Ctx      context.Context
		Instance string
	}
	mock.lockDeleteURLMapping.RLock()
	calls = mock.calls.DeleteURLMapping
	mock.lockDeleteURLMapping.RUnlock()
	return calls
}

// GetAndDelete calls GetAndDeleteFunc.
func (mock *ProcessorStoreMock) GetAndDelete(ctx context.Context, url string, limit int) ([]*store.CallbackData, error) {
	if mock.GetAndDeleteFunc == nil {
		panic("ProcessorStoreMock.GetAndDeleteFunc: method is nil but ProcessorStore.GetAndDelete was just called")
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
//	len(mockedProcessorStore.GetAndDeleteCalls())
func (mock *ProcessorStoreMock) GetAndDeleteCalls() []struct {
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

// GetURLMappings calls GetURLMappingsFunc.
func (mock *ProcessorStoreMock) GetURLMappings(ctx context.Context) (map[string]string, error) {
	if mock.GetURLMappingsFunc == nil {
		panic("ProcessorStoreMock.GetURLMappingsFunc: method is nil but ProcessorStore.GetURLMappings was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetURLMappings.Lock()
	mock.calls.GetURLMappings = append(mock.calls.GetURLMappings, callInfo)
	mock.lockGetURLMappings.Unlock()
	return mock.GetURLMappingsFunc(ctx)
}

// GetURLMappingsCalls gets all the calls that were made to GetURLMappings.
// Check the length with:
//
//	len(mockedProcessorStore.GetURLMappingsCalls())
func (mock *ProcessorStoreMock) GetURLMappingsCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetURLMappings.RLock()
	calls = mock.calls.GetURLMappings
	mock.lockGetURLMappings.RUnlock()
	return calls
}

// GetUnmappedURL calls GetUnmappedURLFunc.
func (mock *ProcessorStoreMock) GetUnmappedURL(ctx context.Context) (string, error) {
	if mock.GetUnmappedURLFunc == nil {
		panic("ProcessorStoreMock.GetUnmappedURLFunc: method is nil but ProcessorStore.GetUnmappedURL was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockGetUnmappedURL.Lock()
	mock.calls.GetUnmappedURL = append(mock.calls.GetUnmappedURL, callInfo)
	mock.lockGetUnmappedURL.Unlock()
	return mock.GetUnmappedURLFunc(ctx)
}

// GetUnmappedURLCalls gets all the calls that were made to GetUnmappedURL.
// Check the length with:
//
//	len(mockedProcessorStore.GetUnmappedURLCalls())
func (mock *ProcessorStoreMock) GetUnmappedURLCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockGetUnmappedURL.RLock()
	calls = mock.calls.GetUnmappedURL
	mock.lockGetUnmappedURL.RUnlock()
	return calls
}

// SetURLMapping calls SetURLMappingFunc.
func (mock *ProcessorStoreMock) SetURLMapping(ctx context.Context, m store.URLMapping) error {
	if mock.SetURLMappingFunc == nil {
		panic("ProcessorStoreMock.SetURLMappingFunc: method is nil but ProcessorStore.SetURLMapping was just called")
	}
	callInfo := struct {
		Ctx context.Context
		M   store.URLMapping
	}{
		Ctx: ctx,
		M:   m,
	}
	mock.lockSetURLMapping.Lock()
	mock.calls.SetURLMapping = append(mock.calls.SetURLMapping, callInfo)
	mock.lockSetURLMapping.Unlock()
	return mock.SetURLMappingFunc(ctx, m)
}

// SetURLMappingCalls gets all the calls that were made to SetURLMapping.
// Check the length with:
//
//	len(mockedProcessorStore.SetURLMappingCalls())
func (mock *ProcessorStoreMock) SetURLMappingCalls() []struct {
	Ctx context.Context
	M   store.URLMapping
} {
	var calls []struct {
		Ctx context.Context
		M   store.URLMapping
	}
	mock.lockSetURLMapping.RLock()
	calls = mock.calls.SetURLMapping
	mock.lockSetURLMapping.RUnlock()
	return calls
}
