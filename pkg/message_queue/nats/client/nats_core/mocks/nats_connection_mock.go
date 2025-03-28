// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/pkg/message_queue/nats/client/nats_core"
	"github.com/nats-io/nats.go"
	"sync"
)

// Ensure, that NatsConnectionMock does implement nats_core.NatsConnection.
// If this is not the case, regenerate this file with moq.
var _ nats_core.NatsConnection = &NatsConnectionMock{}

// NatsConnectionMock is a mock implementation of nats_core.NatsConnection.
//
//	func TestSomethingThatUsesNatsConnection(t *testing.T) {
//
//		// make and configure a mocked nats_core.NatsConnection
//		mockedNatsConnection := &NatsConnectionMock{
//			DrainFunc: func() error {
//				panic("mock out the Drain method")
//			},
//			PublishFunc: func(subj string, data []byte) error {
//				panic("mock out the Publish method")
//			},
//			QueueSubscribeFunc: func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
//				panic("mock out the QueueSubscribe method")
//			},
//			StatusFunc: func() nats.Status {
//				panic("mock out the Status method")
//			},
//		}
//
//		// use mockedNatsConnection in code that requires nats_core.NatsConnection
//		// and then make assertions.
//
//	}
type NatsConnectionMock struct {
	// DrainFunc mocks the Drain method.
	DrainFunc func() error

	// PublishFunc mocks the Publish method.
	PublishFunc func(subj string, data []byte) error

	// QueueSubscribeFunc mocks the QueueSubscribe method.
	QueueSubscribeFunc func(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error)

	// StatusFunc mocks the Status method.
	StatusFunc func() nats.Status

	// calls tracks calls to the methods.
	calls struct {
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
		// Status holds details about calls to the Status method.
		Status []struct {
		}
	}
	lockDrain          sync.RWMutex
	lockPublish        sync.RWMutex
	lockQueueSubscribe sync.RWMutex
	lockStatus         sync.RWMutex
}

// Drain calls DrainFunc.
func (mock *NatsConnectionMock) Drain() error {
	if mock.DrainFunc == nil {
		panic("NatsConnectionMock.DrainFunc: method is nil but NatsConnection.Drain was just called")
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
//	len(mockedNatsConnection.DrainCalls())
func (mock *NatsConnectionMock) DrainCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockDrain.RLock()
	calls = mock.calls.Drain
	mock.lockDrain.RUnlock()
	return calls
}

// Publish calls PublishFunc.
func (mock *NatsConnectionMock) Publish(subj string, data []byte) error {
	if mock.PublishFunc == nil {
		panic("NatsConnectionMock.PublishFunc: method is nil but NatsConnection.Publish was just called")
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
//	len(mockedNatsConnection.PublishCalls())
func (mock *NatsConnectionMock) PublishCalls() []struct {
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
func (mock *NatsConnectionMock) QueueSubscribe(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	if mock.QueueSubscribeFunc == nil {
		panic("NatsConnectionMock.QueueSubscribeFunc: method is nil but NatsConnection.QueueSubscribe was just called")
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
//	len(mockedNatsConnection.QueueSubscribeCalls())
func (mock *NatsConnectionMock) QueueSubscribeCalls() []struct {
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

// Status calls StatusFunc.
func (mock *NatsConnectionMock) Status() nats.Status {
	if mock.StatusFunc == nil {
		panic("NatsConnectionMock.StatusFunc: method is nil but NatsConnection.Status was just called")
	}
	callInfo := struct {
	}{}
	mock.lockStatus.Lock()
	mock.calls.Status = append(mock.calls.Status, callInfo)
	mock.lockStatus.Unlock()
	return mock.StatusFunc()
}

// StatusCalls gets all the calls that were made to Status.
// Check the length with:
//
//	len(mockedNatsConnection.StatusCalls())
func (mock *NatsConnectionMock) StatusCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockStatus.RLock()
	calls = mock.calls.Status
	mock.lockStatus.RUnlock()
	return calls
}
