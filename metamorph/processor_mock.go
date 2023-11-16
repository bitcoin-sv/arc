// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package metamorph

import (
	"context"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"sync"
)

// Ensure, that ProcessorIMock does implement ProcessorI.
// If this is not the case, regenerate this file with moq.
var _ ProcessorI = &ProcessorIMock{}

// ProcessorIMock is a mock implementation of ProcessorI.
//
//	func TestSomethingThatUsesProcessorI(t *testing.T) {
//
//		// make and configure a mocked ProcessorI
//		mockedProcessorI := &ProcessorIMock{
//			GetPeersFunc: func() ([]string, []string) {
//				panic("mock out the GetPeers method")
//			},
//			GetStatsFunc: func(debugItems bool) *ProcessorStats {
//				panic("mock out the GetStats method")
//			},
//			LoadUnminedFunc: func()  {
//				panic("mock out the LoadUnmined method")
//			},
//			ProcessTransactionFunc: func(ctx context.Context, req *ProcessorRequest)  {
//				panic("mock out the ProcessTransaction method")
//			},
//			SendStatusForTransactionFunc: func(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error) {
//				panic("mock out the SendStatusForTransaction method")
//			},
//			SendStatusMinedForTransactionFunc: func(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error) {
//				panic("mock out the SendStatusMinedForTransaction method")
//			},
//			SetFunc: func(ctx context.Context, req *ProcessorRequest) error {
//				panic("mock out the Set method")
//			},
//			ShutdownFunc: func()  {
//				panic("mock out the Shutdown method")
//			},
//		}
//
//		// use mockedProcessorI in code that requires ProcessorI
//		// and then make assertions.
//
//	}
type ProcessorIMock struct {
	// GetPeersFunc mocks the GetPeers method.
	GetPeersFunc func() ([]string, []string)

	// GetStatsFunc mocks the GetStats method.
	GetStatsFunc func(debugItems bool) *ProcessorStats

	// LoadUnminedFunc mocks the LoadUnmined method.
	LoadUnminedFunc func()

	// ProcessTransactionFunc mocks the ProcessTransaction method.
	ProcessTransactionFunc func(ctx context.Context, req *ProcessorRequest)

	// SendStatusForTransactionFunc mocks the SendStatusForTransaction method.
	SendStatusForTransactionFunc func(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error)

	// SendStatusMinedForTransactionFunc mocks the SendStatusMinedForTransaction method.
	SendStatusMinedForTransactionFunc func(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error)

	// SetFunc mocks the Set method.
	SetFunc func(ctx context.Context, req *ProcessorRequest) error

	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// calls tracks calls to the methods.
	calls struct {
		// GetPeers holds details about calls to the GetPeers method.
		GetPeers []struct {
		}
		// GetStats holds details about calls to the GetStats method.
		GetStats []struct {
			// DebugItems is the debugItems argument value.
			DebugItems bool
		}
		// LoadUnmined holds details about calls to the LoadUnmined method.
		LoadUnmined []struct {
		}
		// ProcessTransaction holds details about calls to the ProcessTransaction method.
		ProcessTransaction []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Req is the req argument value.
			Req *ProcessorRequest
		}
		// SendStatusForTransaction holds details about calls to the SendStatusForTransaction method.
		SendStatusForTransaction []struct {
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// Status is the status argument value.
			Status metamorph_api.Status
			// ID is the id argument value.
			ID string
			// Err is the err argument value.
			Err error
		}
		// SendStatusMinedForTransaction holds details about calls to the SendStatusMinedForTransaction method.
		SendStatusMinedForTransaction []struct {
			// Hash is the hash argument value.
			Hash *chainhash.Hash
			// BlockHash is the blockHash argument value.
			BlockHash *chainhash.Hash
			// BlockHeight is the blockHeight argument value.
			BlockHeight uint64
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Req is the req argument value.
			Req *ProcessorRequest
		}
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
	}
	lockGetPeers                      sync.RWMutex
	lockGetStats                      sync.RWMutex
	lockLoadUnmined                   sync.RWMutex
	lockProcessTransaction            sync.RWMutex
	lockSendStatusForTransaction      sync.RWMutex
	lockSendStatusMinedForTransaction sync.RWMutex
	lockSet                           sync.RWMutex
	lockShutdown                      sync.RWMutex
}

// GetPeers calls GetPeersFunc.
func (mock *ProcessorIMock) GetPeers() ([]string, []string) {
	if mock.GetPeersFunc == nil {
		panic("ProcessorIMock.GetPeersFunc: method is nil but ProcessorI.GetPeers was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetPeers.Lock()
	mock.calls.GetPeers = append(mock.calls.GetPeers, callInfo)
	mock.lockGetPeers.Unlock()
	return mock.GetPeersFunc()
}

// GetPeersCalls gets all the calls that were made to GetPeers.
// Check the length with:
//
//	len(mockedProcessorI.GetPeersCalls())
func (mock *ProcessorIMock) GetPeersCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetPeers.RLock()
	calls = mock.calls.GetPeers
	mock.lockGetPeers.RUnlock()
	return calls
}

// GetStats calls GetStatsFunc.
func (mock *ProcessorIMock) GetStats(debugItems bool) *ProcessorStats {
	if mock.GetStatsFunc == nil {
		panic("ProcessorIMock.GetStatsFunc: method is nil but ProcessorI.GetStats was just called")
	}
	callInfo := struct {
		DebugItems bool
	}{
		DebugItems: debugItems,
	}
	mock.lockGetStats.Lock()
	mock.calls.GetStats = append(mock.calls.GetStats, callInfo)
	mock.lockGetStats.Unlock()
	return mock.GetStatsFunc(debugItems)
}

// GetStatsCalls gets all the calls that were made to GetStats.
// Check the length with:
//
//	len(mockedProcessorI.GetStatsCalls())
func (mock *ProcessorIMock) GetStatsCalls() []struct {
	DebugItems bool
} {
	var calls []struct {
		DebugItems bool
	}
	mock.lockGetStats.RLock()
	calls = mock.calls.GetStats
	mock.lockGetStats.RUnlock()
	return calls
}

// LoadUnmined calls LoadUnminedFunc.
func (mock *ProcessorIMock) LoadUnmined() {
	if mock.LoadUnminedFunc == nil {
		panic("ProcessorIMock.LoadUnminedFunc: method is nil but ProcessorI.LoadUnmined was just called")
	}
	callInfo := struct {
	}{}
	mock.lockLoadUnmined.Lock()
	mock.calls.LoadUnmined = append(mock.calls.LoadUnmined, callInfo)
	mock.lockLoadUnmined.Unlock()
	mock.LoadUnminedFunc()
}

// LoadUnminedCalls gets all the calls that were made to LoadUnmined.
// Check the length with:
//
//	len(mockedProcessorI.LoadUnminedCalls())
func (mock *ProcessorIMock) LoadUnminedCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockLoadUnmined.RLock()
	calls = mock.calls.LoadUnmined
	mock.lockLoadUnmined.RUnlock()
	return calls
}

// ProcessTransaction calls ProcessTransactionFunc.
func (mock *ProcessorIMock) ProcessTransaction(ctx context.Context, req *ProcessorRequest) {
	if mock.ProcessTransactionFunc == nil {
		panic("ProcessorIMock.ProcessTransactionFunc: method is nil but ProcessorI.ProcessTransaction was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Req *ProcessorRequest
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
	Req *ProcessorRequest
} {
	var calls []struct {
		Ctx context.Context
		Req *ProcessorRequest
	}
	mock.lockProcessTransaction.RLock()
	calls = mock.calls.ProcessTransaction
	mock.lockProcessTransaction.RUnlock()
	return calls
}

// SendStatusForTransaction calls SendStatusForTransactionFunc.
func (mock *ProcessorIMock) SendStatusForTransaction(hash *chainhash.Hash, status metamorph_api.Status, id string, err error) (bool, error) {
	if mock.SendStatusForTransactionFunc == nil {
		panic("ProcessorIMock.SendStatusForTransactionFunc: method is nil but ProcessorI.SendStatusForTransaction was just called")
	}
	callInfo := struct {
		Hash   *chainhash.Hash
		Status metamorph_api.Status
		ID     string
		Err    error
	}{
		Hash:   hash,
		Status: status,
		ID:     id,
		Err:    err,
	}
	mock.lockSendStatusForTransaction.Lock()
	mock.calls.SendStatusForTransaction = append(mock.calls.SendStatusForTransaction, callInfo)
	mock.lockSendStatusForTransaction.Unlock()
	return mock.SendStatusForTransactionFunc(hash, status, id, err)
}

// SendStatusForTransactionCalls gets all the calls that were made to SendStatusForTransaction.
// Check the length with:
//
//	len(mockedProcessorI.SendStatusForTransactionCalls())
func (mock *ProcessorIMock) SendStatusForTransactionCalls() []struct {
	Hash   *chainhash.Hash
	Status metamorph_api.Status
	ID     string
	Err    error
} {
	var calls []struct {
		Hash   *chainhash.Hash
		Status metamorph_api.Status
		ID     string
		Err    error
	}
	mock.lockSendStatusForTransaction.RLock()
	calls = mock.calls.SendStatusForTransaction
	mock.lockSendStatusForTransaction.RUnlock()
	return calls
}

// SendStatusMinedForTransaction calls SendStatusMinedForTransactionFunc.
func (mock *ProcessorIMock) SendStatusMinedForTransaction(hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) (bool, error) {
	if mock.SendStatusMinedForTransactionFunc == nil {
		panic("ProcessorIMock.SendStatusMinedForTransactionFunc: method is nil but ProcessorI.SendStatusMinedForTransaction was just called")
	}
	callInfo := struct {
		Hash        *chainhash.Hash
		BlockHash   *chainhash.Hash
		BlockHeight uint64
	}{
		Hash:        hash,
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
	}
	mock.lockSendStatusMinedForTransaction.Lock()
	mock.calls.SendStatusMinedForTransaction = append(mock.calls.SendStatusMinedForTransaction, callInfo)
	mock.lockSendStatusMinedForTransaction.Unlock()
	return mock.SendStatusMinedForTransactionFunc(hash, blockHash, blockHeight)
}

// SendStatusMinedForTransactionCalls gets all the calls that were made to SendStatusMinedForTransaction.
// Check the length with:
//
//	len(mockedProcessorI.SendStatusMinedForTransactionCalls())
func (mock *ProcessorIMock) SendStatusMinedForTransactionCalls() []struct {
	Hash        *chainhash.Hash
	BlockHash   *chainhash.Hash
	BlockHeight uint64
} {
	var calls []struct {
		Hash        *chainhash.Hash
		BlockHash   *chainhash.Hash
		BlockHeight uint64
	}
	mock.lockSendStatusMinedForTransaction.RLock()
	calls = mock.calls.SendStatusMinedForTransaction
	mock.lockSendStatusMinedForTransaction.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *ProcessorIMock) Set(ctx context.Context, req *ProcessorRequest) error {
	if mock.SetFunc == nil {
		panic("ProcessorIMock.SetFunc: method is nil but ProcessorI.Set was just called")
	}
	callInfo := struct {
		Ctx context.Context
		Req *ProcessorRequest
	}{
		Ctx: ctx,
		Req: req,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(ctx, req)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedProcessorI.SetCalls())
func (mock *ProcessorIMock) SetCalls() []struct {
	Ctx context.Context
	Req *ProcessorRequest
} {
	var calls []struct {
		Ctx context.Context
		Req *ProcessorRequest
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}

// Shutdown calls ShutdownFunc.
func (mock *ProcessorIMock) Shutdown() {
	if mock.ShutdownFunc == nil {
		panic("ProcessorIMock.ShutdownFunc: method is nil but ProcessorI.Shutdown was just called")
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
//	len(mockedProcessorI.ShutdownCalls())
func (mock *ProcessorIMock) ShutdownCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockShutdown.RLock()
	calls = mock.calls.Shutdown
	mock.lockShutdown.RUnlock()
	return calls
}
