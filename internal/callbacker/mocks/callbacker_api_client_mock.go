// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	context "context"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	grpc "google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	sync "sync"
)

// Ensure, that CallbackerAPIClientMock does implement callbacker_api.CallbackerAPIClient.
// If this is not the case, regenerate this file with moq.
var _ callbacker_api.CallbackerAPIClient = &CallbackerAPIClientMock{}

// CallbackerAPIClientMock is a mock implementation of callbacker_api.CallbackerAPIClient.
//
//	func TestSomethingThatUsesCallbackerAPIClient(t *testing.T) {
//
//		// make and configure a mocked callbacker_api.CallbackerAPIClient
//		mockedCallbackerAPIClient := &CallbackerAPIClientMock{
//			HealthFunc: func(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*callbacker_api.HealthResponse, error) {
//				panic("mock out the Health method")
//			},
//			SendCallbackFunc: func(ctx context.Context, in *callbacker_api.SendCallbackRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
//				panic("mock out the SendCallback method")
//			},
//			UpdateInstancesFunc: func(ctx context.Context, in *callbacker_api.UpdateInstancesRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
//				panic("mock out the UpdateInstances method")
//			},
//		}
//
//		// use mockedCallbackerAPIClient in code that requires callbacker_api.CallbackerAPIClient
//		// and then make assertions.
//
//	}
type CallbackerAPIClientMock struct {
	// HealthFunc mocks the Health method.
	HealthFunc func(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*callbacker_api.HealthResponse, error)

	// SendCallbackFunc mocks the SendCallback method.
	SendCallbackFunc func(ctx context.Context, in *callbacker_api.SendCallbackRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)

	// UpdateInstancesFunc mocks the UpdateInstances method.
	UpdateInstancesFunc func(ctx context.Context, in *callbacker_api.UpdateInstancesRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)

	// calls tracks calls to the methods.
	calls struct {
		// Health holds details about calls to the Health method.
		Health []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// In is the in argument value.
			In *emptypb.Empty
			// Opts is the opts argument value.
			Opts []grpc.CallOption
		}
		// SendCallback holds details about calls to the SendCallback method.
		SendCallback []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// In is the in argument value.
			In *callbacker_api.SendCallbackRequest
			// Opts is the opts argument value.
			Opts []grpc.CallOption
		}
		// UpdateInstances holds details about calls to the UpdateInstances method.
		UpdateInstances []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// In is the in argument value.
			In *callbacker_api.UpdateInstancesRequest
			// Opts is the opts argument value.
			Opts []grpc.CallOption
		}
	}
	lockHealth          sync.RWMutex
	lockSendCallback    sync.RWMutex
	lockUpdateInstances sync.RWMutex
}

// Health calls HealthFunc.
func (mock *CallbackerAPIClientMock) Health(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*callbacker_api.HealthResponse, error) {
	if mock.HealthFunc == nil {
		panic("CallbackerAPIClientMock.HealthFunc: method is nil but CallbackerAPIClient.Health was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		In   *emptypb.Empty
		Opts []grpc.CallOption
	}{
		Ctx:  ctx,
		In:   in,
		Opts: opts,
	}
	mock.lockHealth.Lock()
	mock.calls.Health = append(mock.calls.Health, callInfo)
	mock.lockHealth.Unlock()
	return mock.HealthFunc(ctx, in, opts...)
}

// HealthCalls gets all the calls that were made to Health.
// Check the length with:
//
//	len(mockedCallbackerAPIClient.HealthCalls())
func (mock *CallbackerAPIClientMock) HealthCalls() []struct {
	Ctx  context.Context
	In   *emptypb.Empty
	Opts []grpc.CallOption
} {
	var calls []struct {
		Ctx  context.Context
		In   *emptypb.Empty
		Opts []grpc.CallOption
	}
	mock.lockHealth.RLock()
	calls = mock.calls.Health
	mock.lockHealth.RUnlock()
	return calls
}

// SendCallback calls SendCallbackFunc.
func (mock *CallbackerAPIClientMock) SendCallback(ctx context.Context, in *callbacker_api.SendCallbackRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	if mock.SendCallbackFunc == nil {
		panic("CallbackerAPIClientMock.SendCallbackFunc: method is nil but CallbackerAPIClient.SendCallback was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		In   *callbacker_api.SendCallbackRequest
		Opts []grpc.CallOption
	}{
		Ctx:  ctx,
		In:   in,
		Opts: opts,
	}
	mock.lockSendCallback.Lock()
	mock.calls.SendCallback = append(mock.calls.SendCallback, callInfo)
	mock.lockSendCallback.Unlock()
	return mock.SendCallbackFunc(ctx, in, opts...)
}

// SendCallbackCalls gets all the calls that were made to SendCallback.
// Check the length with:
//
//	len(mockedCallbackerAPIClient.SendCallbackCalls())
func (mock *CallbackerAPIClientMock) SendCallbackCalls() []struct {
	Ctx  context.Context
	In   *callbacker_api.SendCallbackRequest
	Opts []grpc.CallOption
} {
	var calls []struct {
		Ctx  context.Context
		In   *callbacker_api.SendCallbackRequest
		Opts []grpc.CallOption
	}
	mock.lockSendCallback.RLock()
	calls = mock.calls.SendCallback
	mock.lockSendCallback.RUnlock()
	return calls
}

// UpdateInstances calls UpdateInstancesFunc.
func (mock *CallbackerAPIClientMock) UpdateInstances(ctx context.Context, in *callbacker_api.UpdateInstancesRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	if mock.UpdateInstancesFunc == nil {
		panic("CallbackerAPIClientMock.UpdateInstancesFunc: method is nil but CallbackerAPIClient.UpdateInstances was just called")
	}
	callInfo := struct {
		Ctx  context.Context
		In   *callbacker_api.UpdateInstancesRequest
		Opts []grpc.CallOption
	}{
		Ctx:  ctx,
		In:   in,
		Opts: opts,
	}
	mock.lockUpdateInstances.Lock()
	mock.calls.UpdateInstances = append(mock.calls.UpdateInstances, callInfo)
	mock.lockUpdateInstances.Unlock()
	return mock.UpdateInstancesFunc(ctx, in, opts...)
}

// UpdateInstancesCalls gets all the calls that were made to UpdateInstances.
// Check the length with:
//
//	len(mockedCallbackerAPIClient.UpdateInstancesCalls())
func (mock *CallbackerAPIClientMock) UpdateInstancesCalls() []struct {
	Ctx  context.Context
	In   *callbacker_api.UpdateInstancesRequest
	Opts []grpc.CallOption
} {
	var calls []struct {
		Ctx  context.Context
		In   *callbacker_api.UpdateInstancesRequest
		Opts []grpc.CallOption
	}
	mock.lockUpdateInstances.RLock()
	calls = mock.calls.UpdateInstances
	mock.lockUpdateInstances.RUnlock()
	return calls
}
