// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/blocktx/async"
	"github.com/nats-io/nats.go"
	"sync"
)

// Ensure, that NatsClientMock does implement async.NatsClient.
// If this is not the case, regenerate this file with moq.
var _ async.NatsClient = &NatsClientMock{}

// NatsClientMock is a mock implementation of async.NatsClient.
//
//	func TestSomethingThatUsesNatsClient(t *testing.T) {
//
//		// make and configure a mocked async.NatsClient
//		mockedNatsClient := &NatsClientMock{
//			CloseFunc: func()  {
//				panic("mock out the Close method")
//			},
//			DrainFunc: func() error {
//				panic("mock out the Drain method")
//			},
//			PublishFunc: func(subj string, data []byte) error {
//				panic("mock out the Publish method")
//			},
//			QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
//				panic("mock out the QueueSubscribe method")
//			},
//		}
//
//		// use mockedNatsClient in code that requires async.NatsClient
//		// and then make assertions.
//
//	}
type NatsClientMock struct {
	// CloseFunc mocks the Close method.
	CloseFunc func()

	// DrainFunc mocks the Drain method.
	DrainFunc func() error

	// PublishFunc mocks the Publish method.
	PublishFunc func(subj string, data []byte) error

	// QueueSubscribeFunc mocks the QueueSubscribe method.
	QueueSubscribeFunc func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error)

	// calls tracks calls to the methods.
	calls struct {
		// Close holds details about calls to the Close method.
		Close []struct {
		}
		// Drain holds details about calls to the Drain method.
		Drain []struct {
		}
		// Publish holds details about calls to the Publish method.
		Publish []struct {
			// Subj is the subj argument value.
			Subj string
			// Data is the data argument value.
			Data []byte
		}
		// QueueSubscribe holds details about calls to the QueueSubscribe method.
		QueueSubscribe []struct {
			// Subj is the subj argument value.
			Subj string
			// Queue is the queue argument value.
			Queue string
			// Cb is the cb argument value.
			Cb nats.MsgHandler
		}
	}
	lockClose          sync.RWMutex
	lockDrain          sync.RWMutex
	lockPublish        sync.RWMutex
	lockQueueSubscribe sync.RWMutex
}

// Close calls CloseFunc.
func (mock *NatsClientMock) Close() {
	if mock.CloseFunc == nil {
		panic("NatsClientMock.CloseFunc: method is nil but NatsClient.Close was just called")
	}
	callInfo := struct {
	}{}
	mock.lockClose.Lock()
	mock.calls.Close = append(mock.calls.Close, callInfo)
	mock.lockClose.Unlock()
	mock.CloseFunc()
}

// CloseCalls gets all the calls that were made to Close.
// Check the length with:
//
//	len(mockedNatsClient.CloseCalls())
func (mock *NatsClientMock) CloseCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockClose.RLock()
	calls = mock.calls.Close
	mock.lockClose.RUnlock()
	return calls
}

// Drain calls DrainFunc.
func (mock *NatsClientMock) Drain() error {
	if mock.DrainFunc == nil {
		panic("NatsClientMock.DrainFunc: method is nil but NatsClient.Drain was just called")
	}
	callInfo := struct {
	}{}
	mock.lockDrain.Lock()
	mock.calls.Drain = append(mock.calls.Drain, callInfo)
	mock.lockDrain.Unlock()
	return mock.DrainFunc()
}

// DrainCalls gets all the calls that were made to Drain.
// Check the length with:
//
//	len(mockedNatsClient.DrainCalls())
func (mock *NatsClientMock) DrainCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockDrain.RLock()
	calls = mock.calls.Drain
	mock.lockDrain.RUnlock()
	return calls
}

// Publish calls PublishFunc.
func (mock *NatsClientMock) Publish(subj string, data []byte) error {
	if mock.PublishFunc == nil {
		panic("NatsClientMock.PublishFunc: method is nil but NatsClient.Publish was just called")
	}
	callInfo := struct {
		Subj string
		Data []byte
	}{
		Subj: subj,
		Data: data,
	}
	mock.lockPublish.Lock()
	mock.calls.Publish = append(mock.calls.Publish, callInfo)
	mock.lockPublish.Unlock()
	return mock.PublishFunc(subj, data)
}

// PublishCalls gets all the calls that were made to Publish.
// Check the length with:
//
//	len(mockedNatsClient.PublishCalls())
func (mock *NatsClientMock) PublishCalls() []struct {
	Subj string
	Data []byte
} {
	var calls []struct {
		Subj string
		Data []byte
	}
	mock.lockPublish.RLock()
	calls = mock.calls.Publish
	mock.lockPublish.RUnlock()
	return calls
}

// QueueSubscribe calls QueueSubscribeFunc.
func (mock *NatsClientMock) QueueSubscribe(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	if mock.QueueSubscribeFunc == nil {
		panic("NatsClientMock.QueueSubscribeFunc: method is nil but NatsClient.QueueSubscribe was just called")
	}
	callInfo := struct {
		Subj  string
		Queue string
		Cb    nats.MsgHandler
	}{
		Subj:  subj,
		Queue: queue,
		Cb:    cb,
	}
	mock.lockQueueSubscribe.Lock()
	mock.calls.QueueSubscribe = append(mock.calls.QueueSubscribe, callInfo)
	mock.lockQueueSubscribe.Unlock()
	return mock.QueueSubscribeFunc(subj, queue, cb)
}

// QueueSubscribeCalls gets all the calls that were made to QueueSubscribe.
// Check the length with:
//
//	len(mockedNatsClient.QueueSubscribeCalls())
func (mock *NatsClientMock) QueueSubscribeCalls() []struct {
	Subj  string
	Queue string
	Cb    nats.MsgHandler
} {
	var calls []struct {
		Subj  string
		Queue string
		Cb    nats.MsgHandler
	}
	mock.lockQueueSubscribe.RLock()
	calls = mock.calls.QueueSubscribe
	mock.lockQueueSubscribe.RUnlock()
	return calls
}
