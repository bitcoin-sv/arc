// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"sync"
)

// Ensure, that RateBroadcasterMock does implement broadcaster.RateBroadcaster.
// If this is not the case, regenerate this file with moq.
var _ broadcaster.RateBroadcaster = &RateBroadcasterMock{}

// RateBroadcasterMock is a mock implementation of broadcaster.RateBroadcaster.
//
//	func TestSomethingThatUsesRateBroadcaster(t *testing.T) {
//
//		// make and configure a mocked broadcaster.RateBroadcaster
//		mockedRateBroadcaster := &RateBroadcasterMock{
//			GetConnectionCountFunc: func() int64 {
//				panic("mock out the GetConnectionCount method")
//			},
//			GetLimitFunc: func() int64 {
//				panic("mock out the GetLimit method")
//			},
//			GetTxCountFunc: func() int64 {
//				panic("mock out the GetTxCount method")
//			},
//			GetUtxoSetLenFunc: func() int {
//				panic("mock out the GetUtxoSetLen method")
//			},
//			ShutdownFunc: func()  {
//				panic("mock out the Shutdown method")
//			},
//			StartFunc: func() error {
//				panic("mock out the Start method")
//			},
//			WaitFunc: func()  {
//				panic("mock out the Wait method")
//			},
//		}
//
//		// use mockedRateBroadcaster in code that requires broadcaster.RateBroadcaster
//		// and then make assertions.
//
//	}
type RateBroadcasterMock struct {
	// GetConnectionCountFunc mocks the GetConnectionCount method.
	GetConnectionCountFunc func() int64

	// GetLimitFunc mocks the GetLimit method.
	GetLimitFunc func() int64

	// GetTxCountFunc mocks the GetTxCount method.
	GetTxCountFunc func() int64

	// GetUtxoSetLenFunc mocks the GetUtxoSetLen method.
	GetUtxoSetLenFunc func() int

	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// StartFunc mocks the Start method.
	StartFunc func() error

	// WaitFunc mocks the Wait method.
	WaitFunc func()

	// calls tracks calls to the methods.
	calls struct {
		// GetConnectionCount holds details about calls to the GetConnectionCount method.
		GetConnectionCount []struct {
		}
		// GetLimit holds details about calls to the GetLimit method.
		GetLimit []struct {
		}
		// GetTxCount holds details about calls to the GetTxCount method.
		GetTxCount []struct {
		}
		// GetUtxoSetLen holds details about calls to the GetUtxoSetLen method.
		GetUtxoSetLen []struct {
		}
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
		// Start holds details about calls to the Start method.
		Start []struct {
		}
		// Wait holds details about calls to the Wait method.
		Wait []struct {
		}
	}
	lockGetConnectionCount sync.RWMutex
	lockGetLimit           sync.RWMutex
	lockGetTxCount         sync.RWMutex
	lockGetUtxoSetLen      sync.RWMutex
	lockShutdown           sync.RWMutex
	lockStart              sync.RWMutex
	lockWait               sync.RWMutex
}

// GetConnectionCount calls GetConnectionCountFunc.
func (mock *RateBroadcasterMock) GetConnectionCount() int64 {
	if mock.GetConnectionCountFunc == nil {
		panic("RateBroadcasterMock.GetConnectionCountFunc: method is nil but RateBroadcaster.GetConnectionCount was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetConnectionCount.Lock()
	mock.calls.GetConnectionCount = append(mock.calls.GetConnectionCount, callInfo)
	mock.lockGetConnectionCount.Unlock()
	return mock.GetConnectionCountFunc()
}

// GetConnectionCountCalls gets all the calls that were made to GetConnectionCount.
// Check the length with:
//
//	len(mockedRateBroadcaster.GetConnectionCountCalls())
func (mock *RateBroadcasterMock) GetConnectionCountCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetConnectionCount.RLock()
	calls = mock.calls.GetConnectionCount
	mock.lockGetConnectionCount.RUnlock()
	return calls
}

// GetLimit calls GetLimitFunc.
func (mock *RateBroadcasterMock) GetLimit() int64 {
	if mock.GetLimitFunc == nil {
		panic("RateBroadcasterMock.GetLimitFunc: method is nil but RateBroadcaster.GetLimit was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetLimit.Lock()
	mock.calls.GetLimit = append(mock.calls.GetLimit, callInfo)
	mock.lockGetLimit.Unlock()
	return mock.GetLimitFunc()
}

// GetLimitCalls gets all the calls that were made to GetLimit.
// Check the length with:
//
//	len(mockedRateBroadcaster.GetLimitCalls())
func (mock *RateBroadcasterMock) GetLimitCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetLimit.RLock()
	calls = mock.calls.GetLimit
	mock.lockGetLimit.RUnlock()
	return calls
}

// GetTxCount calls GetTxCountFunc.
func (mock *RateBroadcasterMock) GetTxCount() int64 {
	if mock.GetTxCountFunc == nil {
		panic("RateBroadcasterMock.GetTxCountFunc: method is nil but RateBroadcaster.GetTxCount was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetTxCount.Lock()
	mock.calls.GetTxCount = append(mock.calls.GetTxCount, callInfo)
	mock.lockGetTxCount.Unlock()
	return mock.GetTxCountFunc()
}

// GetTxCountCalls gets all the calls that were made to GetTxCount.
// Check the length with:
//
//	len(mockedRateBroadcaster.GetTxCountCalls())
func (mock *RateBroadcasterMock) GetTxCountCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetTxCount.RLock()
	calls = mock.calls.GetTxCount
	mock.lockGetTxCount.RUnlock()
	return calls
}

// GetUtxoSetLen calls GetUtxoSetLenFunc.
func (mock *RateBroadcasterMock) GetUtxoSetLen() int {
	if mock.GetUtxoSetLenFunc == nil {
		panic("RateBroadcasterMock.GetUtxoSetLenFunc: method is nil but RateBroadcaster.GetUtxoSetLen was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetUtxoSetLen.Lock()
	mock.calls.GetUtxoSetLen = append(mock.calls.GetUtxoSetLen, callInfo)
	mock.lockGetUtxoSetLen.Unlock()
	return mock.GetUtxoSetLenFunc()
}

// GetUtxoSetLenCalls gets all the calls that were made to GetUtxoSetLen.
// Check the length with:
//
//	len(mockedRateBroadcaster.GetUtxoSetLenCalls())
func (mock *RateBroadcasterMock) GetUtxoSetLenCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetUtxoSetLen.RLock()
	calls = mock.calls.GetUtxoSetLen
	mock.lockGetUtxoSetLen.RUnlock()
	return calls
}

// Shutdown calls ShutdownFunc.
func (mock *RateBroadcasterMock) Shutdown() {
	if mock.ShutdownFunc == nil {
		panic("RateBroadcasterMock.ShutdownFunc: method is nil but RateBroadcaster.Shutdown was just called")
	}
	callInfo := struct {
	}{}
	mock.lockShutdown.Lock()
	mock.calls.Shutdown = append(mock.calls.Shutdown, callInfo)
	mock.lockShutdown.Unlock()
	mock.ShutdownFunc()
}

// ShutdownCalls gets all the calls that were made to Shutdown.
// Check the length with:
//
//	len(mockedRateBroadcaster.ShutdownCalls())
func (mock *RateBroadcasterMock) ShutdownCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockShutdown.RLock()
	calls = mock.calls.Shutdown
	mock.lockShutdown.RUnlock()
	return calls
}

// Start calls StartFunc.
func (mock *RateBroadcasterMock) Start() error {
	if mock.StartFunc == nil {
		panic("RateBroadcasterMock.StartFunc: method is nil but RateBroadcaster.Start was just called")
	}
	callInfo := struct {
	}{}
	mock.lockStart.Lock()
	mock.calls.Start = append(mock.calls.Start, callInfo)
	mock.lockStart.Unlock()
	return mock.StartFunc()
}

// StartCalls gets all the calls that were made to Start.
// Check the length with:
//
//	len(mockedRateBroadcaster.StartCalls())
func (mock *RateBroadcasterMock) StartCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockStart.RLock()
	calls = mock.calls.Start
	mock.lockStart.RUnlock()
	return calls
}

// Wait calls WaitFunc.
func (mock *RateBroadcasterMock) Wait() {
	if mock.WaitFunc == nil {
		panic("RateBroadcasterMock.WaitFunc: method is nil but RateBroadcaster.Wait was just called")
	}
	callInfo := struct {
	}{}
	mock.lockWait.Lock()
	mock.calls.Wait = append(mock.calls.Wait, callInfo)
	mock.lockWait.Unlock()
	mock.WaitFunc()
}

// WaitCalls gets all the calls that were made to Wait.
// Check the length with:
//
//	len(mockedRateBroadcaster.WaitCalls())
func (mock *RateBroadcasterMock) WaitCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockWait.RLock()
	calls = mock.calls.Wait
	mock.lockWait.RUnlock()
	return calls
}
