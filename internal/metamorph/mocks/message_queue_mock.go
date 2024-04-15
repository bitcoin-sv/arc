// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"sync"
)

// Ensure, that MessageQueueClientMock does implement metamorph.MessageQueueClient.
// If this is not the case, regenerate this file with moq.
var _ metamorph.MessageQueueClient = &MessageQueueClientMock{}

// MessageQueueClientMock is a mock implementation of metamorph.MessageQueueClient.
//
//	func TestSomethingThatUsesMessageQueueClient(t *testing.T) {
//
//		// make and configure a mocked metamorph.MessageQueueClient
//		mockedMessageQueueClient := &MessageQueueClientMock{
//			PublishRegisterTxsFunc: func(hash []byte) error {
//				panic("mock out the PublishRegisterTxs method")
//			},
//			RequestTxFunc: func(hash []byte) error {
//				panic("mock out the RequestTx method")
//			},
//			ShutdownFunc: func() error {
//				panic("mock out the Shutdown method")
//			},
//			SubscribeMinedTxsFunc: func() error {
//				panic("mock out the SubscribeMinedTxs method")
//			},
//		}
//
//		// use mockedMessageQueueClient in code that requires metamorph.MessageQueueClient
//		// and then make assertions.
//
//	}
type MessageQueueClientMock struct {
	// PublishRegisterTxsFunc mocks the PublishRegisterTxs method.
	PublishRegisterTxsFunc func(hash []byte) error

	// RequestTxFunc mocks the RequestTx method.
	RequestTxFunc func(hash []byte) error

	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func() error

	// SubscribeMinedTxsFunc mocks the SubscribeMinedTxs method.
	SubscribeMinedTxsFunc func() error

	// calls tracks calls to the methods.
	calls struct {
		// PublishRegisterTxs holds details about calls to the PublishRegisterTxs method.
		PublishRegisterTxs []struct {
			// Hash is the hash argument value.
			Hash []byte
		}
		// RequestTx holds details about calls to the RequestTx method.
		RequestTx []struct {
			// Hash is the hash argument value.
			Hash []byte
		}
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
		// SubscribeMinedTxs holds details about calls to the SubscribeMinedTxs method.
		SubscribeMinedTxs []struct {
		}
	}
	lockPublishRegisterTxs sync.RWMutex
	lockRequestTx          sync.RWMutex
	lockShutdown           sync.RWMutex
	lockSubscribeMinedTxs  sync.RWMutex
}

// PublishRegisterTxs calls PublishRegisterTxsFunc.
func (mock *MessageQueueClientMock) PublishRegisterTxs(hash []byte) error {
	if mock.PublishRegisterTxsFunc == nil {
		panic("MessageQueueClientMock.PublishRegisterTxsFunc: method is nil but MessageQueueClient.PublishRegisterTxs was just called")
	}
	callInfo := struct {
		Hash []byte
	}{
		Hash: hash,
	}
	mock.lockPublishRegisterTxs.Lock()
	mock.calls.PublishRegisterTxs = append(mock.calls.PublishRegisterTxs, callInfo)
	mock.lockPublishRegisterTxs.Unlock()
	return mock.PublishRegisterTxsFunc(hash)
}

// PublishRegisterTxsCalls gets all the calls that were made to PublishRegisterTxs.
// Check the length with:
//
//	len(mockedMessageQueueClient.PublishRegisterTxsCalls())
func (mock *MessageQueueClientMock) PublishRegisterTxsCalls() []struct {
	Hash []byte
} {
	var calls []struct {
		Hash []byte
	}
	mock.lockPublishRegisterTxs.RLock()
	calls = mock.calls.PublishRegisterTxs
	mock.lockPublishRegisterTxs.RUnlock()
	return calls
}

// RequestTx calls RequestTxFunc.
func (mock *MessageQueueClientMock) RequestTx(hash []byte) error {
	if mock.RequestTxFunc == nil {
		panic("MessageQueueClientMock.RequestTxFunc: method is nil but MessageQueueClient.RequestTx was just called")
	}
	callInfo := struct {
		Hash []byte
	}{
		Hash: hash,
	}
	mock.lockRequestTx.Lock()
	mock.calls.RequestTx = append(mock.calls.RequestTx, callInfo)
	mock.lockRequestTx.Unlock()
	return mock.RequestTxFunc(hash)
}

// RequestTxCalls gets all the calls that were made to RequestTx.
// Check the length with:
//
//	len(mockedMessageQueueClient.RequestTxCalls())
func (mock *MessageQueueClientMock) RequestTxCalls() []struct {
	Hash []byte
} {
	var calls []struct {
		Hash []byte
	}
	mock.lockRequestTx.RLock()
	calls = mock.calls.RequestTx
	mock.lockRequestTx.RUnlock()
	return calls
}

// Shutdown calls ShutdownFunc.
func (mock *MessageQueueClientMock) Shutdown() error {
	if mock.ShutdownFunc == nil {
		panic("MessageQueueClientMock.ShutdownFunc: method is nil but MessageQueueClient.Shutdown was just called")
	}
	callInfo := struct {
	}{}
	mock.lockShutdown.Lock()
	mock.calls.Shutdown = append(mock.calls.Shutdown, callInfo)
	mock.lockShutdown.Unlock()
	return mock.ShutdownFunc()
}

// ShutdownCalls gets all the calls that were made to Shutdown.
// Check the length with:
//
//	len(mockedMessageQueueClient.ShutdownCalls())
func (mock *MessageQueueClientMock) ShutdownCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockShutdown.RLock()
	calls = mock.calls.Shutdown
	mock.lockShutdown.RUnlock()
	return calls
}

// SubscribeMinedTxs calls SubscribeMinedTxsFunc.
func (mock *MessageQueueClientMock) SubscribeMinedTxs() error {
	if mock.SubscribeMinedTxsFunc == nil {
		panic("MessageQueueClientMock.SubscribeMinedTxsFunc: method is nil but MessageQueueClient.SubscribeMinedTxs was just called")
	}
	callInfo := struct {
	}{}
	mock.lockSubscribeMinedTxs.Lock()
	mock.calls.SubscribeMinedTxs = append(mock.calls.SubscribeMinedTxs, callInfo)
	mock.lockSubscribeMinedTxs.Unlock()
	return mock.SubscribeMinedTxsFunc()
}

// SubscribeMinedTxsCalls gets all the calls that were made to SubscribeMinedTxs.
// Check the length with:
//
//	len(mockedMessageQueueClient.SubscribeMinedTxsCalls())
func (mock *MessageQueueClientMock) SubscribeMinedTxsCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockSubscribeMinedTxs.RLock()
	calls = mock.calls.SubscribeMinedTxs
	mock.lockSubscribeMinedTxs.RUnlock()
	return calls
}
