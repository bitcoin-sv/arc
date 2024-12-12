// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"sync"
)

// Ensure, that WatcherMock does implement blocktx.Watcher.
// If this is not the case, regenerate this file with moq.
var _ blocktx.Watcher = &WatcherMock{}

// WatcherMock is a mock implementation of blocktx.Watcher.
//
//	func TestSomethingThatUsesWatcher(t *testing.T) {
//
//		// make and configure a mocked blocktx.Watcher
//		mockedWatcher := &WatcherMock{
//			ClearBlockTransactionsMapFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
//				panic("mock out the ClearBlockTransactionsMap method")
//			},
//			ClearBlocksFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
//				panic("mock out the ClearBlocks method")
//			},
//			ClearTransactionsFunc: func(ctx context.Context, retentionDays int32) (int64, error) {
//				panic("mock out the ClearTransactions method")
//			},
//			DelUnfinishedBlockProcessingFunc: func(ctx context.Context, processedBy string) (int64, error) {
//				panic("mock out the DelUnfinishedBlockProcessing method")
//			},
//			HealthFunc: func(ctx context.Context) error {
//				panic("mock out the Health method")
//			},
//		}
//
//		// use mockedWatcher in code that requires blocktx.Watcher
//		// and then make assertions.
//
//	}
type WatcherMock struct {
	// ClearBlockTransactionsMapFunc mocks the ClearBlockTransactionsMap method.
	ClearBlockTransactionsMapFunc func(ctx context.Context, retentionDays int32) (int64, error)

	// ClearBlocksFunc mocks the ClearBlocks method.
	ClearBlocksFunc func(ctx context.Context, retentionDays int32) (int64, error)

	// ClearTransactionsFunc mocks the ClearTransactions method.
	ClearTransactionsFunc func(ctx context.Context, retentionDays int32) (int64, error)

	// DelUnfinishedBlockProcessingFunc mocks the DelUnfinishedBlockProcessing method.
	DelUnfinishedBlockProcessingFunc func(ctx context.Context, processedBy string) (int64, error)

	// HealthFunc mocks the Health method.
	HealthFunc func(ctx context.Context) error

	// calls tracks calls to the methods.
	calls struct {
		// ClearBlockTransactionsMap holds details about calls to the ClearBlockTransactionsMap method.
		ClearBlockTransactionsMap []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RetentionDays is the retentionDays argument value.
			RetentionDays int32
		}
		// ClearBlocks holds details about calls to the ClearBlocks method.
		ClearBlocks []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RetentionDays is the retentionDays argument value.
			RetentionDays int32
		}
		// ClearTransactions holds details about calls to the ClearTransactions method.
		ClearTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// RetentionDays is the retentionDays argument value.
			RetentionDays int32
		}
		// DelUnfinishedBlockProcessing holds details about calls to the DelUnfinishedBlockProcessing method.
		DelUnfinishedBlockProcessing []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ProcessedBy is the processedBy argument value.
			ProcessedBy string
		}
		// Health holds details about calls to the Health method.
		Health []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
	}
	lockClearBlockTransactionsMap    sync.RWMutex
	lockClearBlocks                  sync.RWMutex
	lockClearTransactions            sync.RWMutex
	lockDelUnfinishedBlockProcessing sync.RWMutex
	lockHealth                       sync.RWMutex
}

// ClearBlockTransactionsMap calls ClearBlockTransactionsMapFunc.
func (mock *WatcherMock) ClearBlockTransactionsMap(ctx context.Context, retentionDays int32) (int64, error) {
	if mock.ClearBlockTransactionsMapFunc == nil {
		panic("WatcherMock.ClearBlockTransactionsMapFunc: method is nil but Watcher.ClearBlockTransactionsMap was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		RetentionDays int32
	}{
		Ctx:           ctx,
		RetentionDays: retentionDays,
	}
	mock.lockClearBlockTransactionsMap.Lock()
	mock.calls.ClearBlockTransactionsMap = append(mock.calls.ClearBlockTransactionsMap, callInfo)
	mock.lockClearBlockTransactionsMap.Unlock()
	return mock.ClearBlockTransactionsMapFunc(ctx, retentionDays)
}

// ClearBlockTransactionsMapCalls gets all the calls that were made to ClearBlockTransactionsMap.
// Check the length with:
//
//	len(mockedWatcher.ClearBlockTransactionsMapCalls())
func (mock *WatcherMock) ClearBlockTransactionsMapCalls() []struct {
	Ctx           context.Context
	RetentionDays int32
} {
	var calls []struct {
		Ctx           context.Context
		RetentionDays int32
	}
	mock.lockClearBlockTransactionsMap.RLock()
	calls = mock.calls.ClearBlockTransactionsMap
	mock.lockClearBlockTransactionsMap.RUnlock()
	return calls
}

// ClearBlocks calls ClearBlocksFunc.
func (mock *WatcherMock) ClearBlocks(ctx context.Context, retentionDays int32) (int64, error) {
	if mock.ClearBlocksFunc == nil {
		panic("WatcherMock.ClearBlocksFunc: method is nil but Watcher.ClearBlocks was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		RetentionDays int32
	}{
		Ctx:           ctx,
		RetentionDays: retentionDays,
	}
	mock.lockClearBlocks.Lock()
	mock.calls.ClearBlocks = append(mock.calls.ClearBlocks, callInfo)
	mock.lockClearBlocks.Unlock()
	return mock.ClearBlocksFunc(ctx, retentionDays)
}

// ClearBlocksCalls gets all the calls that were made to ClearBlocks.
// Check the length with:
//
//	len(mockedWatcher.ClearBlocksCalls())
func (mock *WatcherMock) ClearBlocksCalls() []struct {
	Ctx           context.Context
	RetentionDays int32
} {
	var calls []struct {
		Ctx           context.Context
		RetentionDays int32
	}
	mock.lockClearBlocks.RLock()
	calls = mock.calls.ClearBlocks
	mock.lockClearBlocks.RUnlock()
	return calls
}

// ClearTransactions calls ClearTransactionsFunc.
func (mock *WatcherMock) ClearTransactions(ctx context.Context, retentionDays int32) (int64, error) {
	if mock.ClearTransactionsFunc == nil {
		panic("WatcherMock.ClearTransactionsFunc: method is nil but Watcher.ClearTransactions was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		RetentionDays int32
	}{
		Ctx:           ctx,
		RetentionDays: retentionDays,
	}
	mock.lockClearTransactions.Lock()
	mock.calls.ClearTransactions = append(mock.calls.ClearTransactions, callInfo)
	mock.lockClearTransactions.Unlock()
	return mock.ClearTransactionsFunc(ctx, retentionDays)
}

// ClearTransactionsCalls gets all the calls that were made to ClearTransactions.
// Check the length with:
//
//	len(mockedWatcher.ClearTransactionsCalls())
func (mock *WatcherMock) ClearTransactionsCalls() []struct {
	Ctx           context.Context
	RetentionDays int32
} {
	var calls []struct {
		Ctx           context.Context
		RetentionDays int32
	}
	mock.lockClearTransactions.RLock()
	calls = mock.calls.ClearTransactions
	mock.lockClearTransactions.RUnlock()
	return calls
}

// DelUnfinishedBlockProcessing calls DelUnfinishedBlockProcessingFunc.
func (mock *WatcherMock) DelUnfinishedBlockProcessing(ctx context.Context, processedBy string) (int64, error) {
	if mock.DelUnfinishedBlockProcessingFunc == nil {
		panic("WatcherMock.DelUnfinishedBlockProcessingFunc: method is nil but Watcher.DelUnfinishedBlockProcessing was just called")
	}
	callInfo := struct {
		Ctx         context.Context
		ProcessedBy string
	}{
		Ctx:         ctx,
		ProcessedBy: processedBy,
	}
	mock.lockDelUnfinishedBlockProcessing.Lock()
	mock.calls.DelUnfinishedBlockProcessing = append(mock.calls.DelUnfinishedBlockProcessing, callInfo)
	mock.lockDelUnfinishedBlockProcessing.Unlock()
	return mock.DelUnfinishedBlockProcessingFunc(ctx, processedBy)
}

// DelUnfinishedBlockProcessingCalls gets all the calls that were made to DelUnfinishedBlockProcessing.
// Check the length with:
//
//	len(mockedWatcher.DelUnfinishedBlockProcessingCalls())
func (mock *WatcherMock) DelUnfinishedBlockProcessingCalls() []struct {
	Ctx         context.Context
	ProcessedBy string
} {
	var calls []struct {
		Ctx         context.Context
		ProcessedBy string
	}
	mock.lockDelUnfinishedBlockProcessing.RLock()
	calls = mock.calls.DelUnfinishedBlockProcessing
	mock.lockDelUnfinishedBlockProcessing.RUnlock()
	return calls
}

// Health calls HealthFunc.
func (mock *WatcherMock) Health(ctx context.Context) error {
	if mock.HealthFunc == nil {
		panic("WatcherMock.HealthFunc: method is nil but Watcher.Health was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockHealth.Lock()
	mock.calls.Health = append(mock.calls.Health, callInfo)
	mock.lockHealth.Unlock()
	return mock.HealthFunc(ctx)
}

// HealthCalls gets all the calls that were made to Health.
// Check the length with:
//
//	len(mockedWatcher.HealthCalls())
func (mock *WatcherMock) HealthCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockHealth.RLock()
	calls = mock.calls.Health
	mock.lockHealth.RUnlock()
	return calls
}