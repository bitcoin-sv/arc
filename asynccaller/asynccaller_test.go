package asynccaller

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/labstack/gommon/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestData struct {
	Number int
	String string
}

type TestCaller[T TestData] struct {
	mu          sync.Mutex
	called      []*TestData
	callerError error
}

func (t *TestCaller[T]) GetCalled() []*TestData {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.called
}

func (t *TestCaller[T]) SetCallerError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.callerError = err
}

func (t *TestCaller[T]) Caller(data *TestData) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.callerError != nil {
		return t.callerError
	}

	t.called = append(t.called, data)
	return nil
}

func (t *TestCaller[T]) MarshalString(data *TestData) (string, error) {
	return fmt.Sprintf("%d,%s", data.Number, data.String), nil
}

func (t *TestCaller[T]) UnmarshalString(data string) (*TestData, error) {
	str := strings.Split(data, ",")
	if len(str) != 2 {
		return nil, fmt.Errorf("invalid data")
	}

	nr, err := strconv.ParseInt(str[0], 10, 64)
	if err != nil {
		return nil, err
	}

	return &TestData{
		Number: int(nr),
		String: str[1],
	}, nil
}

func TestNew(t *testing.T) {
	t.Run("should create a new AsyncCaller", func(t *testing.T) {
		dirName := "./test-asynccaller" + random.String(10)
		defer os.RemoveAll(dirName)

		testCaller := &TestCaller[TestData]{}

		ac, err := New[TestData](&p2p.TestLogger{}, dirName, 10*time.Second, testCaller)
		require.NoError(t, err)
		require.NotNil(t, ac)

		_, err = os.Stat(dirName)
		require.NoError(t, err)
		assert.False(t, os.IsNotExist(err))
	})
}

func TestCall(t *testing.T) {
	t.Run("should call the caller", func(t *testing.T) {
		dirName := "./test-asynccaller" + random.String(10)
		defer os.RemoveAll(dirName)

		testCaller := &TestCaller[TestData]{}

		ac, err := New[TestData](&p2p.TestLogger{}, dirName, 10*time.Second, testCaller)
		require.NoError(t, err)
		require.NotNil(t, ac)

		testData := &TestData{
			Number: 1,
			String: "test",
		}

		ac.Call(testData)

		// wait for worker to finish
		time.Sleep(10 * time.Millisecond)

		assert.Equal(t, 1, len(testCaller.GetCalled()))
		assert.Equal(t, testData, testCaller.GetCalled()[0])

		files, err := os.ReadDir(dirName)
		require.NoError(t, err)
		assert.Equal(t, 0, len(files))
	})

	t.Run("should save to file", func(t *testing.T) {
		dirName := "./test-asynccaller" + random.String(10)
		defer os.RemoveAll(dirName)

		testError := fmt.Errorf("test error")
		testCaller := &TestCaller[TestData]{
			callerError: testError,
		}

		ac, err := New[TestData](&p2p.TestLogger{}, dirName, 10*time.Millisecond, testCaller)
		require.NoError(t, err)
		require.NotNil(t, ac)

		testData := &TestData{
			Number: 1,
			String: "test",
		}

		ac.Call(testData)
		assert.Equal(t, 0, len(testCaller.called))

		time.Sleep(20 * time.Millisecond)

		files, err := os.ReadDir(dirName)
		require.NoError(t, err)
		assert.Equal(t, 1, len(files))

		testCaller.SetCallerError(nil)
		ac.processFiles()
		assert.Equal(t, 1, len(testCaller.GetCalled()))
		assert.Equal(t, testData, testCaller.GetCalled()[0])
	})
}
