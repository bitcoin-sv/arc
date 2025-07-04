// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"sync"
)

// Ensure, that ClientMock does implement blocktx.Client.
// If this is not the case, regenerate this file with moq.
var _ blocktx.Client = &ClientMock{}

// ClientMock is a mock implementation of blocktx.Client.
//
//	func TestSomethingThatUsesClient(t *testing.T) {
//
//		// make and configure a mocked blocktx.Client
//		mockedClient := &ClientMock{
//			AnyTransactionsMinedFunc: func(ctx context.Context, hash [][]byte) ([]*blocktx_api.IsMined, error) {
//				panic("mock out the AnyTransactionsMined method")
//			},
//			CurrentBlockHeightFunc: func(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error) {
//				panic("mock out the CurrentBlockHeight method")
//			},
//			RegisterTransactionFunc: func(ctx context.Context, hash []byte) error {
//				panic("mock out the RegisterTransaction method")
//			},
//			RegisterTransactionsFunc: func(ctx context.Context, hashes [][]byte) error {
//				panic("mock out the RegisterTransactions method")
//			},
//		}
//
//		// use mockedClient in code that requires blocktx.Client
//		// and then make assertions.
//
//	}
type ClientMock struct {
	// AnyTransactionsMinedFunc mocks the AnyTransactionsMined method.
	AnyTransactionsMinedFunc func(ctx context.Context, hash [][]byte) ([]*blocktx_api.IsMined, error)

	// CurrentBlockHeightFunc mocks the CurrentBlockHeight method.
	CurrentBlockHeightFunc func(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error)

	// RegisterTransactionFunc mocks the RegisterTransaction method.
	RegisterTransactionFunc func(ctx context.Context, hash []byte) error

	// RegisterTransactionsFunc mocks the RegisterTransactions method.
	RegisterTransactionsFunc func(ctx context.Context, hashes [][]byte) error

	// calls tracks calls to the methods.
	calls struct {
		// AnyTransactionsMined holds details about calls to the AnyTransactionsMined method.
		AnyTransactionsMined []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash [][]byte
		}
		// CurrentBlockHeight holds details about calls to the CurrentBlockHeight method.
		CurrentBlockHeight []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
		}
		// RegisterTransaction holds details about calls to the RegisterTransaction method.
		RegisterTransaction []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hash is the hash argument value.
			Hash []byte
		}
		// RegisterTransactions holds details about calls to the RegisterTransactions method.
		RegisterTransactions []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Hashes is the hashes argument value.
			Hashes [][]byte
		}
	}
	lockAnyTransactionsMined sync.RWMutex
	lockCurrentBlockHeight   sync.RWMutex
	lockRegisterTransaction  sync.RWMutex
	lockRegisterTransactions sync.RWMutex
}

// AnyTransactionsMined calls AnyTransactionsMinedFunc.
func (mock *ClientMock) AnyTransactionsMined(ctx context.Context, hash [][]byte) ([]*blocktx_api.IsMined, error) {
	if mock.AnyTransactionsMinedFunc == nil {
		panic("ClientMock.AnyTransactionsMinedFunc: method is nil but Client.AnyTransactionsMined was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash [][]byte
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockAnyTransactionsMined.Lock()
	mock.calls.AnyTransactionsMined = append(mock.calls.AnyTransactionsMined, callInfo)
	mock.lockAnyTransactionsMined.Unlock()
	return mock.AnyTransactionsMinedFunc(ctx, hash)
}

// AnyTransactionsMinedCalls gets all the calls that were made to AnyTransactionsMined.
// Check the length with:
//
//	len(mockedClient.AnyTransactionsMinedCalls())
func (mock *ClientMock) AnyTransactionsMinedCalls() []struct {
	Ctx  context.Context
	Hash [][]byte
} {
	var calls []struct {
		Ctx  context.Context
		Hash [][]byte
	}
	mock.lockAnyTransactionsMined.RLock()
	calls = mock.calls.AnyTransactionsMined
	mock.lockAnyTransactionsMined.RUnlock()
	return calls
}

// CurrentBlockHeight calls CurrentBlockHeightFunc.
func (mock *ClientMock) CurrentBlockHeight(ctx context.Context) (*blocktx_api.CurrentBlockHeightResponse, error) {
	if mock.CurrentBlockHeightFunc == nil {
		panic("ClientMock.CurrentBlockHeightFunc: method is nil but Client.CurrentBlockHeight was just called")
	}
	callInfo := struct {
		Ctx context.Context
	}{
		Ctx: ctx,
	}
	mock.lockCurrentBlockHeight.Lock()
	mock.calls.CurrentBlockHeight = append(mock.calls.CurrentBlockHeight, callInfo)
	mock.lockCurrentBlockHeight.Unlock()
	return mock.CurrentBlockHeightFunc(ctx)
}

// CurrentBlockHeightCalls gets all the calls that were made to CurrentBlockHeight.
// Check the length with:
//
//	len(mockedClient.CurrentBlockHeightCalls())
func (mock *ClientMock) CurrentBlockHeightCalls() []struct {
	Ctx context.Context
} {
	var calls []struct {
		Ctx context.Context
	}
	mock.lockCurrentBlockHeight.RLock()
	calls = mock.calls.CurrentBlockHeight
	mock.lockCurrentBlockHeight.RUnlock()
	return calls
}

// RegisterTransaction calls RegisterTransactionFunc.
func (mock *ClientMock) RegisterTransaction(ctx context.Context, hash []byte) error {
	if mock.RegisterTransactionFunc == nil {
		panic("ClientMock.RegisterTransactionFunc: method is nil but Client.RegisterTransaction was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		Hash []byte
	}{
		Ctx:  ctx,
		Hash: hash,
	}
	mock.lockRegisterTransaction.Lock()
	mock.calls.RegisterTransaction = append(mock.calls.RegisterTransaction, callInfo)
	mock.lockRegisterTransaction.Unlock()
	return mock.RegisterTransactionFunc(ctx, hash)
}

// RegisterTransactionCalls gets all the calls that were made to RegisterTransaction.
// Check the length with:
//
//	len(mockedClient.RegisterTransactionCalls())
func (mock *ClientMock) RegisterTransactionCalls() []struct {
	Ctx  context.Context
	Hash []byte
} {
	var calls []struct {
		Ctx  context.Context
		Hash []byte
	}
	mock.lockRegisterTransaction.RLock()
	calls = mock.calls.RegisterTransaction
	mock.lockRegisterTransaction.RUnlock()
	return calls
}

// RegisterTransactions calls RegisterTransactionsFunc.
func (mock *ClientMock) RegisterTransactions(ctx context.Context, hashes [][]byte) error {
	if mock.RegisterTransactionsFunc == nil {
		panic("ClientMock.RegisterTransactionsFunc: method is nil but Client.RegisterTransactions was just called")
	}
	callInfo := struct {
		Ctx    context.Context
		Hashes [][]byte
	}{
		Ctx:    ctx,
		Hashes: hashes,
	}
	mock.lockRegisterTransactions.Lock()
	mock.calls.RegisterTransactions = append(mock.calls.RegisterTransactions, callInfo)
	mock.lockRegisterTransactions.Unlock()
	return mock.RegisterTransactionsFunc(ctx, hashes)
}

// RegisterTransactionsCalls gets all the calls that were made to RegisterTransactions.
// Check the length with:
//
//	len(mockedClient.RegisterTransactionsCalls())
func (mock *ClientMock) RegisterTransactionsCalls() []struct {
	Ctx    context.Context
	Hashes [][]byte
} {
	var calls []struct {
		Ctx    context.Context
		Hashes [][]byte
	}
	mock.lockRegisterTransactions.RLock()
	calls = mock.calls.RegisterTransactions
	mock.lockRegisterTransactions.RUnlock()
	return calls
}
