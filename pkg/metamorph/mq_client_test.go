package metamorph_test

import (
	"errors"
	"testing"

	"github.com/bitcoin-sv/arc/pkg/metamorph/mocks"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DummyProtoMessage is a simple implementation of the proto.Message interface for testing purposes.
type DummyProtoMessage struct{}

// ProtoReflect implements protoreflect.ProtoMessage.
func (m *DummyProtoMessage) ProtoReflect() protoreflect.Message {
	panic("unimplemented")
}

func (m *DummyProtoMessage) Reset()         {}
func (m *DummyProtoMessage) String() string { return "DummyProtoMessage" }
func (m *DummyProtoMessage) ProtoMessage()  {}

func TestMessageQueueClient_PublishMarshal(t *testing.T) {
	// Given
	mockClient := &mocks.MessageQueueClientMock{
		PublishMarshalFunc: func(topic string, m proto.Message) error {
			return nil
		},
	}

	msg := &DummyProtoMessage{}

	// When
	err := mockClient.PublishMarshal("submit-tx", msg)
	// Then
	require.NoError(t, err)
	require.Equal(t, 1, len(mockClient.PublishMarshalCalls()))
	require.Equal(t, "submit-tx", mockClient.PublishMarshalCalls()[0].Topic)
	require.Equal(t, msg, mockClient.PublishMarshalCalls()[0].M)
}

func TestMessageQueueClient_PublishMarshal_Error(t *testing.T) {
	// Given
	mockClient := &mocks.MessageQueueClientMock{
		PublishMarshalFunc: func(topic string, m proto.Message) error {
			return errors.New("publish failed")
		},
	}
	msg := &DummyProtoMessage{}
	// When
	err := mockClient.PublishMarshal("submit-tx", msg)
	// Then
	require.Error(t, err)
	require.Equal(t, "publish failed", err.Error())
	require.Equal(t, 1, len(mockClient.PublishMarshalCalls()))
	require.Equal(t, "submit-tx", mockClient.PublishMarshalCalls()[0].Topic)
	require.Equal(t, msg, mockClient.PublishMarshalCalls()[0].M)
}

func TestMessageQueueClient_Shutdown(t *testing.T) {
	// Given
	mockClient := &mocks.MessageQueueClientMock{
		ShutdownFunc: func() {},
	}

	// When
	mockClient.Shutdown()

	// Then
	require.Equal(t, 1, len(mockClient.ShutdownCalls()))
}

func TestMessageQueueClient_PublishMarshal_And_Shutdown(t *testing.T) {
	// Given
	mockClient := &mocks.MessageQueueClientMock{
		PublishMarshalFunc: func(topic string, m proto.Message) error {
			return nil
		},
		ShutdownFunc: func() {},
	}
	msg := &DummyProtoMessage{}
	// When
	err := mockClient.PublishMarshal("submit-tx", msg)
	require.NoError(t, err)
	mockClient.Shutdown()

	// Then
	require.Equal(t, 1, len(mockClient.PublishMarshalCalls()))
	require.Equal(t, 1, len(mockClient.ShutdownCalls()))
}
