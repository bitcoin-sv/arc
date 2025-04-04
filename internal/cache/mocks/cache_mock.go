// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package mocks

import (
	"github.com/bitcoin-sv/arc/internal/cache"
	"sync"
	"time"
)

// Ensure, that StoreMock does implement cache.Store.
// If this is not the case, regenerate this file with moq.
var _ cache.Store = &StoreMock{}

// StoreMock is a mock implementation of cache.Store.
//
//	func TestSomethingThatUsesStore(t *testing.T) {
//
//		// make and configure a mocked cache.Store
//		mockedStore := &StoreMock{
//			DelFunc: func(keys ...string) error {
//				panic("mock out the Del method")
//			},
//			GetFunc: func(key string) ([]byte, error) {
//				panic("mock out the Get method")
//			},
//			MapDelFunc: func(hashsetKey string, fields ...string) error {
//				panic("mock out the MapDel method")
//			},
//			MapExtractAllFunc: func(hashsetKey string) (map[string][]byte, error) {
//				panic("mock out the MapExtractAll method")
//			},
//			MapGetFunc: func(hashsetKey string, field string) ([]byte, error) {
//				panic("mock out the MapGet method")
//			},
//			MapGetAllFunc: func(hashsetKey string) (map[string][]byte, error) {
//				panic("mock out the MapGetAll method")
//			},
//			MapLenFunc: func(hashsetKey string) (int64, error) {
//				panic("mock out the MapLen method")
//			},
//			MapSetFunc: func(hashsetKey string, field string, value []byte) error {
//				panic("mock out the MapSet method")
//			},
//			SetFunc: func(key string, value []byte, ttl time.Duration) error {
//				panic("mock out the Set method")
//			},
//		}
//
//		// use mockedStore in code that requires cache.Store
//		// and then make assertions.
//
//	}
type StoreMock struct {
	// DelFunc mocks the Del method.
	DelFunc func(keys ...string) error

	// GetFunc mocks the Get method.
	GetFunc func(key string) ([]byte, error)

	// MapDelFunc mocks the MapDel method.
	MapDelFunc func(hashsetKey string, fields ...string) error

	// MapExtractAllFunc mocks the MapExtractAll method.
	MapExtractAllFunc func(hashsetKey string) (map[string][]byte, error)

	// MapGetFunc mocks the MapGet method.
	MapGetFunc func(hashsetKey string, field string) ([]byte, error)

	// MapGetAllFunc mocks the MapGetAll method.
	MapGetAllFunc func(hashsetKey string) (map[string][]byte, error)

	// MapLenFunc mocks the MapLen method.
	MapLenFunc func(hashsetKey string) (int64, error)

	// MapSetFunc mocks the MapSet method.
	MapSetFunc func(hashsetKey string, field string, value []byte) error

	// SetFunc mocks the Set method.
	SetFunc func(key string, value []byte, ttl time.Duration) error

	// calls tracks calls to the methods.
	calls struct {
		// Del holds details about calls to the Del method.
		Del []struct {
			// Keys is the keys argument value.
			Keys []string
		}
		// Get holds details about calls to the Get method.
		Get []struct {
			// Key is the key argument value.
			Key string
		}
		// MapDel holds details about calls to the MapDel method.
		MapDel []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
			// Fields is the fields argument value.
			Fields []string
		}
		// MapExtractAll holds details about calls to the MapExtractAll method.
		MapExtractAll []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
		}
		// MapGet holds details about calls to the MapGet method.
		MapGet []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
			// Field is the field argument value.
			Field string
		}
		// MapGetAll holds details about calls to the MapGetAll method.
		MapGetAll []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
		}
		// MapLen holds details about calls to the MapLen method.
		MapLen []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
		}
		// MapSet holds details about calls to the MapSet method.
		MapSet []struct {
			// HashsetKey is the hashsetKey argument value.
			HashsetKey string
			// Field is the field argument value.
			Field string
			// Value is the value argument value.
			Value []byte
		}
		// Set holds details about calls to the Set method.
		Set []struct {
			// Key is the key argument value.
			Key string
			// Value is the value argument value.
			Value []byte
			// TTL is the ttl argument value.
			TTL time.Duration
		}
	}
	lockDel           sync.RWMutex
	lockGet           sync.RWMutex
	lockMapDel        sync.RWMutex
	lockMapExtractAll sync.RWMutex
	lockMapGet        sync.RWMutex
	lockMapGetAll     sync.RWMutex
	lockMapLen        sync.RWMutex
	lockMapSet        sync.RWMutex
	lockSet           sync.RWMutex
}

// Del calls DelFunc.
func (mock *StoreMock) Del(keys ...string) error {
	if mock.DelFunc == nil {
		panic("StoreMock.DelFunc: method is nil but Store.Del was just called")
	}
	callInfo := struct {
		Keys []string
	}{
		Keys: keys,
	}
	mock.lockDel.Lock()
	mock.calls.Del = append(mock.calls.Del, callInfo)
	mock.lockDel.Unlock()
	return mock.DelFunc(keys...)
}

// DelCalls gets all the calls that were made to Del.
// Check the length with:
//
//	len(mockedStore.DelCalls())
func (mock *StoreMock) DelCalls() []struct {
	Keys []string
} {
	var calls []struct {
		Keys []string
	}
	mock.lockDel.RLock()
	calls = mock.calls.Del
	mock.lockDel.RUnlock()
	return calls
}

// Get calls GetFunc.
func (mock *StoreMock) Get(key string) ([]byte, error) {
	if mock.GetFunc == nil {
		panic("StoreMock.GetFunc: method is nil but Store.Get was just called")
	}
	callInfo := struct {
		Key string
	}{
		Key: key,
	}
	mock.lockGet.Lock()
	mock.calls.Get = append(mock.calls.Get, callInfo)
	mock.lockGet.Unlock()
	return mock.GetFunc(key)
}

// GetCalls gets all the calls that were made to Get.
// Check the length with:
//
//	len(mockedStore.GetCalls())
func (mock *StoreMock) GetCalls() []struct {
	Key string
} {
	var calls []struct {
		Key string
	}
	mock.lockGet.RLock()
	calls = mock.calls.Get
	mock.lockGet.RUnlock()
	return calls
}

// MapDel calls MapDelFunc.
func (mock *StoreMock) MapDel(hashsetKey string, fields ...string) error {
	if mock.MapDelFunc == nil {
		panic("StoreMock.MapDelFunc: method is nil but Store.MapDel was just called")
	}
	callInfo := struct {
		HashsetKey string
		Fields     []string
	}{
		HashsetKey: hashsetKey,
		Fields:     fields,
	}
	mock.lockMapDel.Lock()
	mock.calls.MapDel = append(mock.calls.MapDel, callInfo)
	mock.lockMapDel.Unlock()
	return mock.MapDelFunc(hashsetKey, fields...)
}

// MapDelCalls gets all the calls that were made to MapDel.
// Check the length with:
//
//	len(mockedStore.MapDelCalls())
func (mock *StoreMock) MapDelCalls() []struct {
	HashsetKey string
	Fields     []string
} {
	var calls []struct {
		HashsetKey string
		Fields     []string
	}
	mock.lockMapDel.RLock()
	calls = mock.calls.MapDel
	mock.lockMapDel.RUnlock()
	return calls
}

// MapExtractAll calls MapExtractAllFunc.
func (mock *StoreMock) MapExtractAll(hashsetKey string) (map[string][]byte, error) {
	if mock.MapExtractAllFunc == nil {
		panic("StoreMock.MapExtractAllFunc: method is nil but Store.MapExtractAll was just called")
	}
	callInfo := struct {
		HashsetKey string
	}{
		HashsetKey: hashsetKey,
	}
	mock.lockMapExtractAll.Lock()
	mock.calls.MapExtractAll = append(mock.calls.MapExtractAll, callInfo)
	mock.lockMapExtractAll.Unlock()
	return mock.MapExtractAllFunc(hashsetKey)
}

// MapExtractAllCalls gets all the calls that were made to MapExtractAll.
// Check the length with:
//
//	len(mockedStore.MapExtractAllCalls())
func (mock *StoreMock) MapExtractAllCalls() []struct {
	HashsetKey string
} {
	var calls []struct {
		HashsetKey string
	}
	mock.lockMapExtractAll.RLock()
	calls = mock.calls.MapExtractAll
	mock.lockMapExtractAll.RUnlock()
	return calls
}

// MapGet calls MapGetFunc.
func (mock *StoreMock) MapGet(hashsetKey string, field string) ([]byte, error) {
	if mock.MapGetFunc == nil {
		panic("StoreMock.MapGetFunc: method is nil but Store.MapGet was just called")
	}
	callInfo := struct {
		HashsetKey string
		Field      string
	}{
		HashsetKey: hashsetKey,
		Field:      field,
	}
	mock.lockMapGet.Lock()
	mock.calls.MapGet = append(mock.calls.MapGet, callInfo)
	mock.lockMapGet.Unlock()
	return mock.MapGetFunc(hashsetKey, field)
}

// MapGetCalls gets all the calls that were made to MapGet.
// Check the length with:
//
//	len(mockedStore.MapGetCalls())
func (mock *StoreMock) MapGetCalls() []struct {
	HashsetKey string
	Field      string
} {
	var calls []struct {
		HashsetKey string
		Field      string
	}
	mock.lockMapGet.RLock()
	calls = mock.calls.MapGet
	mock.lockMapGet.RUnlock()
	return calls
}

// MapGetAll calls MapGetAllFunc.
func (mock *StoreMock) MapGetAll(hashsetKey string) (map[string][]byte, error) {
	if mock.MapGetAllFunc == nil {
		panic("StoreMock.MapGetAllFunc: method is nil but Store.MapGetAll was just called")
	}
	callInfo := struct {
		HashsetKey string
	}{
		HashsetKey: hashsetKey,
	}
	mock.lockMapGetAll.Lock()
	mock.calls.MapGetAll = append(mock.calls.MapGetAll, callInfo)
	mock.lockMapGetAll.Unlock()
	return mock.MapGetAllFunc(hashsetKey)
}

// MapGetAllCalls gets all the calls that were made to MapGetAll.
// Check the length with:
//
//	len(mockedStore.MapGetAllCalls())
func (mock *StoreMock) MapGetAllCalls() []struct {
	HashsetKey string
} {
	var calls []struct {
		HashsetKey string
	}
	mock.lockMapGetAll.RLock()
	calls = mock.calls.MapGetAll
	mock.lockMapGetAll.RUnlock()
	return calls
}

// MapLen calls MapLenFunc.
func (mock *StoreMock) MapLen(hashsetKey string) (int64, error) {
	if mock.MapLenFunc == nil {
		panic("StoreMock.MapLenFunc: method is nil but Store.MapLen was just called")
	}
	callInfo := struct {
		HashsetKey string
	}{
		HashsetKey: hashsetKey,
	}
	mock.lockMapLen.Lock()
	mock.calls.MapLen = append(mock.calls.MapLen, callInfo)
	mock.lockMapLen.Unlock()
	return mock.MapLenFunc(hashsetKey)
}

// MapLenCalls gets all the calls that were made to MapLen.
// Check the length with:
//
//	len(mockedStore.MapLenCalls())
func (mock *StoreMock) MapLenCalls() []struct {
	HashsetKey string
} {
	var calls []struct {
		HashsetKey string
	}
	mock.lockMapLen.RLock()
	calls = mock.calls.MapLen
	mock.lockMapLen.RUnlock()
	return calls
}

// MapSet calls MapSetFunc.
func (mock *StoreMock) MapSet(hashsetKey string, field string, value []byte) error {
	if mock.MapSetFunc == nil {
		panic("StoreMock.MapSetFunc: method is nil but Store.MapSet was just called")
	}
	callInfo := struct {
		HashsetKey string
		Field      string
		Value      []byte
	}{
		HashsetKey: hashsetKey,
		Field:      field,
		Value:      value,
	}
	mock.lockMapSet.Lock()
	mock.calls.MapSet = append(mock.calls.MapSet, callInfo)
	mock.lockMapSet.Unlock()
	return mock.MapSetFunc(hashsetKey, field, value)
}

// MapSetCalls gets all the calls that were made to MapSet.
// Check the length with:
//
//	len(mockedStore.MapSetCalls())
func (mock *StoreMock) MapSetCalls() []struct {
	HashsetKey string
	Field      string
	Value      []byte
} {
	var calls []struct {
		HashsetKey string
		Field      string
		Value      []byte
	}
	mock.lockMapSet.RLock()
	calls = mock.calls.MapSet
	mock.lockMapSet.RUnlock()
	return calls
}

// Set calls SetFunc.
func (mock *StoreMock) Set(key string, value []byte, ttl time.Duration) error {
	if mock.SetFunc == nil {
		panic("StoreMock.SetFunc: method is nil but Store.Set was just called")
	}
	callInfo := struct {
		Key   string
		Value []byte
		TTL   time.Duration
	}{
		Key:   key,
		Value: value,
		TTL:   ttl,
	}
	mock.lockSet.Lock()
	mock.calls.Set = append(mock.calls.Set, callInfo)
	mock.lockSet.Unlock()
	return mock.SetFunc(key, value, ttl)
}

// SetCalls gets all the calls that were made to Set.
// Check the length with:
//
//	len(mockedStore.SetCalls())
func (mock *StoreMock) SetCalls() []struct {
	Key   string
	Value []byte
	TTL   time.Duration
} {
	var calls []struct {
		Key   string
		Value []byte
		TTL   time.Duration
	}
	mock.lockSet.RLock()
	calls = mock.calls.Set
	mock.lockSet.RUnlock()
	return calls
}
