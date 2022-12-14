package metamorph

import (
	"testing"

	"github.com/ordishs/go-utils"

	"github.com/stretchr/testify/assert"
)

func TestChannel(t *testing.T) {
	ch := make(chan int)

	go func() {
		for i := range ch {
			t.Log("Received:", i)
		}
	}()

	assert.True(t, utils.SafeSend(ch, 1))

	close(ch)

	assert.False(t, utils.SafeSend(ch, 1))

}

func TestStringChannel(t *testing.T) {
	ch := make(chan string)

	go func() {
		for i := range ch {
			t.Log("Received:", i)
		}
	}()

	assert.True(t, utils.SafeSend(ch, "hello1"))

	close(ch)

	assert.False(t, utils.SafeSend(ch, "hello2"))

}
