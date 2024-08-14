// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"sync"
)

// Ensure, that ConsolidatorMock does implement broadcaster.Consolidator.
// If this is not the case, regenerate this file with moq.
var _ broadcaster.Consolidator = &ConsolidatorMock{}

// ConsolidatorMock is a mock implementation of broadcaster.Consolidator.
//
//	func TestSomethingThatUsesConsolidator(t *testing.T) {
//
//		// make and configure a mocked broadcaster.Consolidator
//		mockedConsolidator := &ConsolidatorMock{
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
//		// use mockedConsolidator in code that requires broadcaster.Consolidator
//		// and then make assertions.
//
//	}
type ConsolidatorMock struct {
	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// StartFunc mocks the Start method.
	StartFunc func() error

	// WaitFunc mocks the Wait method.
	WaitFunc func()

	// calls tracks calls to the methods.
	calls struct {
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
	lockShutdown sync.RWMutex
	lockStart    sync.RWMutex
	lockWait     sync.RWMutex
}

// Shutdown calls ShutdownFunc.
func (mock *ConsolidatorMock) Shutdown() {
	if mock.ShutdownFunc == nil {
		panic("ConsolidatorMock.ShutdownFunc: method is nil but Consolidator.Shutdown was just called")
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
//	len(mockedConsolidator.ShutdownCalls())
func (mock *ConsolidatorMock) ShutdownCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockShutdown.RLock()
	calls = mock.calls.Shutdown
	mock.lockShutdown.RUnlock()
	return calls
}

// Start calls StartFunc.
func (mock *ConsolidatorMock) Start() error {
	if mock.StartFunc == nil {
		panic("ConsolidatorMock.StartFunc: method is nil but Consolidator.Start was just called")
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
//	len(mockedConsolidator.StartCalls())
func (mock *ConsolidatorMock) StartCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockStart.RLock()
	calls = mock.calls.Start
	mock.lockStart.RUnlock()
	return calls
}

// Wait calls WaitFunc.
func (mock *ConsolidatorMock) Wait() {
	if mock.WaitFunc == nil {
		panic("ConsolidatorMock.WaitFunc: method is nil but Consolidator.Wait was just called")
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
//	len(mockedConsolidator.WaitCalls())
func (mock *ConsolidatorMock) WaitCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockWait.RLock()
	calls = mock.calls.Wait
	mock.lockWait.RUnlock()
	return calls
}
