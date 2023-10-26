package blocktx

import (
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	blocktxmock "github.com/bitcoin-sv/arc/blocktx/blocktx_api/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type Subscription struct {
	B *blocktx_api.Block
	grpc.ClientStream
	Error error
}

func (d Subscription) Recv() (*blocktx_api.Block, error) {
	return d.B, d.Error
}

func TestStart2(t *testing.T) {
	for _, c := range []struct {
		Blocks            []blocktx_api.Block
		SubscriptionError error
		ClientError       error
	}{
		{
			Blocks: []blocktx_api.Block{
				{Hash: []byte("test1")},
				{Hash: []byte("test2")},
				{Hash: []byte("test3")},
			},
			SubscriptionError: nil,
			ClientError:       nil,
		},
	} {
		blks := c.Blocks
		sub := Subscription{
			B:     &blks[0],
			Error: c.SubscriptionError,
		}

		clientMock := new(blocktxmock.BlockTxAPIClientMock)
		clientMock.On("GetBlockNotificationStream", mock.Anything, mock.Anything).Return(sub, c.ClientError)

		client := NewClient(clientMock)
		ch := make(chan *blocktx_api.Block)
		client.Start(ch)

		go func() {
			for i := 0; i < 3; i++ {
				sub.B = &blks[i]
				select {
				case block := <-ch:
					assert.Equal(t, blks[i], block)
				}
			}
		}()

		client.Shutdown()
	}
}
