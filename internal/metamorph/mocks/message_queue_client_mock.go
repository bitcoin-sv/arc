// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"google.golang.org/protobuf/proto"
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
//			PublishMarshalFunc: func(ctx context.Context, topic string, m proto.Message) error {
//				panic("mock out the PublishMarshal method")
//			},
//			ShutdownFunc: func()  {
//				panic("mock out the Shutdown method")
//			},
//		}
//
//		// use mockedMessageQueueClient in code that requires metamorph.MessageQueueClient
//		// and then make assertions.
//
//	}
type MessageQueueClientMock struct {
	// PublishMarshalFunc mocks the PublishMarshal method.
	PublishMarshalFunc func(ctx context.Context, topic string, m proto.Message) error

	// ShutdownFunc mocks the Shutdown method.
	ShutdownFunc func()

	// calls tracks calls to the methods.
	calls struct {
		// PublishMarshal holds details about calls to the PublishMarshal method.
		PublishMarshal []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// Topic is the topic argument value.
			Topic string
			// M is the m argument value.
			M proto.Message
		}
		// Shutdown holds details about calls to the Shutdown method.
		Shutdown []struct {
		}
	}
	lockPublishMarshal sync.RWMutex
	lockShutdown       sync.RWMutex
}

// PublishMarshal calls PublishMarshalFunc.
func (mock *MessageQueueClientMock) PublishMarshal(ctx context.Context, topic string, m proto.Message) error {
	if mock.PublishMarshalFunc == nil {
		panic("MessageQueueClientMock.PublishMarshalFunc: method is nil but MessageQueueClient.PublishMarshal was just called")
	}
	callInfo := struct {
		Ctx   context.Context
		Topic string
		M     proto.Message
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
	M     proto.Message
} {
	var calls []struct {
		Ctx   context.Context
		Topic string
		M     proto.Message
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
