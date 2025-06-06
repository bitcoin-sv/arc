// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"sync"
	"time"
)

// Ensure, that TickerMock does implement broadcaster.Ticker.
// If this is not the case, regenerate this file with moq.
var _ broadcaster.Ticker = &TickerMock{}

// TickerMock is a mock implementation of broadcaster.Ticker.
//
//	func TestSomethingThatUsesTicker(t *testing.T) {
//
//		// make and configure a mocked broadcaster.Ticker
//		mockedTicker := &TickerMock{
//			GetTickerChFunc: func() (<-chan time.Time, error) {
//				panic("mock out the GetTickerCh method")
//			},
//		}
//
//		// use mockedTicker in code that requires broadcaster.Ticker
//		// and then make assertions.
//
//	}
type TickerMock struct {
	// GetTickerChFunc mocks the GetTickerCh method.
	GetTickerChFunc func() (<-chan time.Time, error)

	// calls tracks calls to the methods.
	calls struct {
		// GetTickerCh holds details about calls to the GetTickerCh method.
		GetTickerCh []struct {
		}
	}
	lockGetTickerCh sync.RWMutex
}

// GetTickerCh calls GetTickerChFunc.
func (mock *TickerMock) GetTickerCh() (<-chan time.Time, error) {
	if mock.GetTickerChFunc == nil {
		panic("TickerMock.GetTickerChFunc: method is nil but Ticker.GetTickerCh was just called")
	}
	callInfo := struct {
	}{}
	mock.lockGetTickerCh.Lock()
	mock.calls.GetTickerCh = append(mock.calls.GetTickerCh, callInfo)
	mock.lockGetTickerCh.Unlock()
	return mock.GetTickerChFunc()
}

// GetTickerChCalls gets all the calls that were made to GetTickerCh.
// Check the length with:
//
//	len(mockedTicker.GetTickerChCalls())
func (mock *TickerMock) GetTickerChCalls() []struct {
} {
	var calls []struct {
	}
	mock.lockGetTickerCh.RLock()
	calls = mock.calls.GetTickerCh
	mock.lockGetTickerCh.RUnlock()
	return calls
}
