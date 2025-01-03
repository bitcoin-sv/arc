// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/callbacker"
	"github.com/nats-io/nats.go/jetstream"
	"sync"
)

// Ensure, that MessageQueueClientMock does implement callbacker.MessageQueueClient.
// If this is not the case, regenerate this file with moq.
var _ callbacker.MessageQueueClient = &MessageQueueClientMock{}

// MessageQueueClientMock is a mock implementation of callbacker.MessageQueueClient.
//
//	func TestSomethingThatUsesMessageQueueClient(t *testing.T) {
//
//		// make and configure a mocked callbacker.MessageQueueClient
//		mockedMessageQueueClient := &MessageQueueClientMock{
//			ShutdownFunc: func()  {
//				panic("mock out the Shutdown method")
//			},
//			SubscribeMsgFunc: func(topic string, msgFunc func(msg jetstream.Msg) error) error {
//				panic("mock out the SubscribeMsg method")
//			},
//		}
//
//		// use mockedMessageQueueClient in code that requires callbacker.MessageQueueClient
//		// and then make assertions.
//
//	}
type MessageQueueClientMock struct {
	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// SubscribeMsgFunc mocks the SubscribeMsg method.
	SubscribeMsgFunc func(topic string, msgFunc func(msg jetstream.Msg) error) error

	// calls tracks calls to the methods.
	calls struct {
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
		// SubscribeMsg holds details about calls to the SubscribeMsg method.
		SubscribeMsg []struct {
			// Topic is the topic argument value.
			Topic string
			// MsgFunc is the msgFunc argument value.
			MsgFunc func(msg jetstream.Msg) error
		}
	}
	lockShutdown     sync.RWMutex
	lockSubscribeMsg sync.RWMutex
}

// Shutdown calls ShutdownFunc.
func (mock *MessageQueueClientMock) Shutdown() {
	if mock.ShutdownFunc == nil {
		panic("MessageQueueClientMock.ShutdownFunc: method is nil but MessageQueueClient.Shutdown was just called")
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

// SubscribeMsg calls SubscribeMsgFunc.
func (mock *MessageQueueClientMock) SubscribeMsg(topic string, msgFunc func(msg jetstream.Msg) error) error {
	if mock.SubscribeMsgFunc == nil {
		panic("MessageQueueClientMock.SubscribeMsgFunc: method is nil but MessageQueueClient.SubscribeMsg was just called")
	}
	callInfo := struct {
		Topic   string
		MsgFunc func(msg jetstream.Msg) error
	}{
		Topic:   topic,
		MsgFunc: msgFunc,
	}
	mock.lockSubscribeMsg.Lock()
	mock.calls.SubscribeMsg = append(mock.calls.SubscribeMsg, callInfo)
	mock.lockSubscribeMsg.Unlock()
	return mock.SubscribeMsgFunc(topic, msgFunc)
}

// SubscribeMsgCalls gets all the calls that were made to SubscribeMsg.
// Check the length with:
//
//	len(mockedMessageQueueClient.SubscribeMsgCalls())
func (mock *MessageQueueClientMock) SubscribeMsgCalls() []struct {
	Topic   string
	MsgFunc func(msg jetstream.Msg) error
} {
	var calls []struct {
		Topic   string
		MsgFunc func(msg jetstream.Msg) error
	}
	mock.lockSubscribeMsg.RLock()
	calls = mock.calls.SubscribeMsg
	mock.lockSubscribeMsg.RUnlock()
	return calls
}
