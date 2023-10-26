package blocktx

import (
	"sync"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blocktxmock "github.com/bitcoin-sv/arc/blocktx/blocktx_api/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type Subscription struct {
	B blocktx_api.Block
	grpc.ClientStream
	Error error
	Curr  int
}

func (d Subscription) Recv() (*blocktx_api.Block, error) {
	return &d.B, d.Error
}

func TestStart2(t *testing.T) {
	for _, c := range []struct {
		Block             blocktx_api.Block
		SubscriptionError error
		ClientError       error
	}{
		{
			Block:             blocktx_api.Block{Hash: []byte("test1")},
			SubscriptionError: nil,
			ClientError:       nil,
		},
	} {
		sub := Subscription{
			B:     c.Block,
			Error: c.SubscriptionError,
			Curr:  0,
		}

		clientMock := new(blocktxmock.BlockTxAPIClientMock)
		clientMock.On("GetBlockNotificationStream", mock.Anything, mock.Anything).Return(sub, c.ClientError)

		client := NewClient(clientMock)
		ch := make(chan *blocktx_api.Block)
		client.Start(ch)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			select {
			case block := <-ch:
				t.Logf("received block %+v", block)
				assert.Equal(t, string(c.Block.Hash), string(block.Hash))
			}
			wg.Done()
		}()
		wg.Wait()
		client.Shutdown()
	}
}
