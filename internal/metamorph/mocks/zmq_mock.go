// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"sync"
)

// Ensure, that ZMQIMock does implement metamorph.ZMQI.
// If this is not the case, regenerate this file with moq.
var _ metamorph.ZMQI = &ZMQIMock{}

// ZMQIMock is a mock implementation of metamorph.ZMQI.
//
//	func TestSomethingThatUsesZMQI(t *testing.T) {
//
//		// make and configure a mocked metamorph.ZMQI
//		mockedZMQI := &ZMQIMock{
//			SubscribeFunc: func(s string, stringsCh chan []string) error {
//				panic("mock out the Subscribe method")
//			},
//		}
//
//		// use mockedZMQI in code that requires metamorph.ZMQI
//		// and then make assertions.
//
//	}
type ZMQIMock struct {
	// SubscribeFunc mocks the Subscribe method.
	SubscribeFunc func(s string, stringsCh chan []string) error

	// calls tracks calls to the methods.
	calls struct {
		// Subscribe holds details about calls to the Subscribe method.
		Subscribe []struct {
			// S is the s argument value.
			S string
			// StringsCh is the stringsCh argument value.
			StringsCh chan []string
		}
	}
	lockSubscribe sync.RWMutex
}

// Subscribe calls SubscribeFunc.
func (mock *ZMQIMock) Subscribe(s string, stringsCh chan []string) error {
	if mock.SubscribeFunc == nil {
		panic("ZMQIMock.SubscribeFunc: method is nil but ZMQI.Subscribe was just called")
	}
	callInfo := struct {
		S         string
		StringsCh chan []string
	}{
		S:         s,
		StringsCh: stringsCh,
	}
	mock.lockSubscribe.Lock()
	mock.calls.Subscribe = append(mock.calls.Subscribe, callInfo)
	mock.lockSubscribe.Unlock()
	return mock.SubscribeFunc(s, stringsCh)
}

// SubscribeCalls gets all the calls that were made to Subscribe.
// Check the length with:
//
//	len(mockedZMQI.SubscribeCalls())
func (mock *ZMQIMock) SubscribeCalls() []struct {
	S         string
	StringsCh chan []string
} {
	var calls []struct {
		S         string
		StringsCh chan []string
	}
	mock.lockSubscribe.RLock()
	calls = mock.calls.Subscribe
	mock.lockSubscribe.RUnlock()
	return calls
}