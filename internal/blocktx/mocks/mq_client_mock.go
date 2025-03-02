// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/blocktx"
	"google.golang.org/protobuf/reflect/protoreflect"
	"sync"
)

// Ensure, that MessageQueueClientMock does implement blocktx.MessageQueueClient.
// If this is not the case, regenerate this file with moq.
var _ blocktx.MessageQueueClient = &MessageQueueClientMock{}

// MessageQueueClientMock is a mock implementation of blocktx.MessageQueueClient.
//
//	func TestSomethingThatUsesMessageQueueClient(t *testing.T) {
//
//		// make and configure a mocked blocktx.MessageQueueClient
//		mockedMessageQueueClient := &MessageQueueClientMock{
//			PublishMarshalFunc: func(ctx context.Context, topic string, m protoreflect.ProtoMessage) error {
//				panic("mock out the PublishMarshal method")
//			},
//			ShutdownFunc: func()  {
//				panic("mock out the Shutdown method")
//			},
//			SubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
//				panic("mock out the Subscribe method")
//			},
//		}
//
//		// use mockedMessageQueueClient in code that requires blocktx.MessageQueueClient
//		// and then make assertions.
//
//	}
type MessageQueueClientMock struct {
	// PublishMarshalFunc mocks the PublishMarshal method.
	PublishMarshalFunc func(ctx context.Context, topic string, m protoreflect.ProtoMessage) error

	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// SubscribeFunc mocks the Subscribe method.
	SubscribeFunc func(topic string, msgFunc func([]byte) error) error

	// calls tracks calls to the methods.
	calls struct {
		// PublishMarshal holds details about calls to the PublishMarshal method.
		PublishMarshal []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Topic is the topic argument value.
			Topic string
			// M is the m argument value.
			M protoreflect.ProtoMessage
		}
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
		// Subscribe holds details about calls to the Subscribe method.
		Subscribe []struct {
			// Topic is the topic argument value.
			Topic string
			// MsgFunc is the msgFunc argument value.
			MsgFunc func([]byte) error
		}
	}
	lockPublishMarshal sync.RWMutex
	lockShutdown       sync.RWMutex
	lockSubscribe      sync.RWMutex
}

// PublishMarshal calls PublishMarshalFunc.
func (mock *MessageQueueClientMock) PublishMarshal(ctx context.Context, topic string, m protoreflect.ProtoMessage) error {
	if mock.PublishMarshalFunc == nil {
		panic("MessageQueueClientMock.PublishMarshalFunc: method is nil but MessageQueueClient.PublishMarshal was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Topic string
		M     protoreflect.ProtoMessage
	}{
		Ctx:   ctx,
		Topic: topic,
		M:     m,
	}
	mock.lockPublishMarshal.Lock()
	mock.calls.PublishMarshal = append(mock.calls.PublishMarshal, callInfo)
	mock.lockPublishMarshal.Unlock()
	return mock.PublishMarshalFunc(ctx, topic, m)
}

// PublishMarshalCalls gets all the calls that were made to PublishMarshal.
// Check the length with:
//
//	len(mockedMessageQueueClient.PublishMarshalCalls())
func (mock *MessageQueueClientMock) PublishMarshalCalls() []struct {
	Ctx   context.Context
	Topic string
	M     protoreflect.ProtoMessage
} {
	var calls []struct {
		Ctx   context.Context
		Topic string
		M     protoreflect.ProtoMessage
	}
	mock.lockPublishMarshal.RLock()
	calls = mock.calls.PublishMarshal
	mock.lockPublishMarshal.RUnlock()
	return calls
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

// Subscribe calls SubscribeFunc.
func (mock *MessageQueueClientMock) Subscribe(topic string, msgFunc func([]byte) error) error {
	if mock.SubscribeFunc == nil {
		panic("MessageQueueClientMock.SubscribeFunc: method is nil but MessageQueueClient.Subscribe was just called")
	}
	callInfo := struct {
		Topic   string
		MsgFunc func([]byte) error
	}{
		Topic:   topic,
		MsgFunc: msgFunc,
	}
	mock.lockSubscribe.Lock()
	mock.calls.Subscribe = append(mock.calls.Subscribe, callInfo)
	mock.lockSubscribe.Unlock()
	return mock.SubscribeFunc(topic, msgFunc)
}

// SubscribeCalls gets all the calls that were made to Subscribe.
// Check the length with:
//
//	len(mockedMessageQueueClient.SubscribeCalls())
func (mock *MessageQueueClientMock) SubscribeCalls() []struct {
	Topic   string
	MsgFunc func([]byte) error
} {
	var calls []struct {
		Topic   string
		MsgFunc func([]byte) error
	}
	mock.lockSubscribe.RLock()
	calls = mock.calls.Subscribe
	mock.lockSubscribe.RUnlock()
	return calls
}
