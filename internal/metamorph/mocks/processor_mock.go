// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"sync"
)

// Ensure, that ProcessorIMock does implement metamorph.ProcessorI.
// If this is not the case, regenerate this file with moq.
var _ metamorph.ProcessorI = &ProcessorIMock{}

// ProcessorIMock is a mock implementation of metamorph.ProcessorI.
//
//	func TestSomethingThatUsesProcessorI(t *testing.T) {
//
//		// make and configure a mocked metamorph.ProcessorI
//		mockedProcessorI := &ProcessorIMock{
//			GetPeersNmberFunc: func() uint {
//				panic("mock out the GetPeersNmber method")
//			},
//			GetProcessorMapSizeFunc: func() int {
//				panic("mock out the GetProcessorMapSize method")
//			},
//			HealthFunc: func() error {
//				panic("mock out the Health method")
//			},
//			ProcessTransactionFunc: func(ctx context.Context, req *metamorph.ProcessorRequest)  {
//				panic("mock out the ProcessTransaction method")
//			},
//		}
//
//		// use mockedProcessorI in code that requires metamorph.ProcessorI
//		// and then make assertions.
//
//	}
type ProcessorIMock struct {
	// GetPeersNmberFunc mocks the GetPeersNmber method.
	GetPeersNmberFunc func() uint

	// GetProcessorMapSizeFunc mocks the GetProcessorMapSize method.
	GetProcessorMapSizeFunc func() int

	// HealthFunc mocks the Health method.
	HealthFunc func() error

	// ProcessTransactionFunc mocks the ProcessTransaction method.
	ProcessTransactionFunc func(ctx context.Context, req *metamorph.ProcessorRequest)

	// calls tracks calls to the methods.
	calls struct {
		// GetPeersNmber holds details about calls to the GetPeersNmber method.
		GetPeersNmber []struct {
		}
		// GetProcessorMapSize holds details about calls to the GetProcessorMapSize method.
		GetProcessorMapSize []struct {
		}
		// Health holds details about calls to the Health method.
		Health []struct {
		}
		// ProcessTransaction holds details about calls to the ProcessTransaction method.
		ProcessTransaction []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Req is the req argument value.
			Req *metamorph.ProcessorRequest
		}
	}
	lockGetPeersNmber       sync.RWMutex
	lockGetProcessorMapSize sync.RWMutex
	lockHealth              sync.RWMutex
	lockProcessTransaction  sync.RWMutex
}

// GetPeersNmber calls GetPeersNmberFunc.
func (mock *ProcessorIMock) GetPeersNmber() uint {
	if mock.GetPeersNmberFunc == nil {
		panic("ProcessorIMock.GetPeersNmberFunc: method is nil but ProcessorI.GetPeersNmber was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetPeersNmber.Lock()
	mock.calls.GetPeersNmber = append(mock.calls.GetPeersNmber, callInfo)
	mock.lockGetPeersNmber.Unlock()
	return mock.GetPeersNmberFunc()
}

// GetPeersNmberCalls gets all the calls that were made to GetPeersNmber.
// Check the length with:
//
//	len(mockedProcessorI.GetPeersNmberCalls())
func (mock *ProcessorIMock) GetPeersNmberCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetPeersNmber.RLock()
	calls = mock.calls.GetPeersNmber
	mock.lockGetPeersNmber.RUnlock()
	return calls
}

// GetProcessorMapSize calls GetProcessorMapSizeFunc.
func (mock *ProcessorIMock) GetProcessorMapSize() int {
	if mock.GetProcessorMapSizeFunc == nil {
		panic("ProcessorIMock.GetProcessorMapSizeFunc: method is nil but ProcessorI.GetProcessorMapSize was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetProcessorMapSize.Lock()
	mock.calls.GetProcessorMapSize = append(mock.calls.GetProcessorMapSize, callInfo)
	mock.lockGetProcessorMapSize.Unlock()
	return mock.GetProcessorMapSizeFunc()
}

// GetProcessorMapSizeCalls gets all the calls that were made to GetProcessorMapSize.
// Check the length with:
//
//	len(mockedProcessorI.GetProcessorMapSizeCalls())
func (mock *ProcessorIMock) GetProcessorMapSizeCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetProcessorMapSize.RLock()
	calls = mock.calls.GetProcessorMapSize
	mock.lockGetProcessorMapSize.RUnlock()
	return calls
}

// Health calls HealthFunc.
func (mock *ProcessorIMock) Health() error {
	if mock.HealthFunc == nil {
		panic("ProcessorIMock.HealthFunc: method is nil but ProcessorI.Health was just called")
	}
	callInfo := struct {
	}{}
	mock.lockHealth.Lock()
	mock.calls.Health = append(mock.calls.Health, callInfo)
	mock.lockHealth.Unlock()
	return mock.HealthFunc()
}

// HealthCalls gets all the calls that were made to Health.
// Check the length with:
//
//	len(mockedProcessorI.HealthCalls())
func (mock *ProcessorIMock) HealthCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockHealth.RLock()
	calls = mock.calls.Health
	mock.lockHealth.RUnlock()
	return calls
}

// ProcessTransaction calls ProcessTransactionFunc.
func (mock *ProcessorIMock) ProcessTransaction(ctx context.Context, req *metamorph.ProcessorRequest) {
	if mock.ProcessTransactionFunc == nil {
		panic("ProcessorIMock.ProcessTransactionFunc: method is nil but ProcessorI.ProcessTransaction was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Req *metamorph.ProcessorRequest
	}{
		Ctx: ctx,
		Req: req,
	}
	mock.lockProcessTransaction.Lock()
	mock.calls.ProcessTransaction = append(mock.calls.ProcessTransaction, callInfo)
	mock.lockProcessTransaction.Unlock()
	mock.ProcessTransactionFunc(ctx, req)
}

// ProcessTransactionCalls gets all the calls that were made to ProcessTransaction.
// Check the length with:
//
//	len(mockedProcessorI.ProcessTransactionCalls())
func (mock *ProcessorIMock) ProcessTransactionCalls() []struct {
	Ctx context.Context
	Req *metamorph.ProcessorRequest
} {
	var calls []struct {
		Ctx context.Context
		Req *metamorph.ProcessorRequest
	}
	mock.lockProcessTransaction.RLock()
	calls = mock.calls.ProcessTransaction
	mock.lockProcessTransaction.RUnlock()
	return calls
}
