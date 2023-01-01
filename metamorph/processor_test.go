package metamorph

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/memorystore"
	"github.com/TAAL-GmbH/arc/p2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessor(t *testing.T) {
	t.Run("NewProcessor", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		messageCh := make(chan *p2p.PMMessage)
		pm := p2p.NewPeerManagerMock(s, messageCh)

		processor := NewProcessor(1, s, pm)
		if processor == nil {
			t.Error("Expected a non-nil Processor")
		}
	})
}

func TestLoadUnseen(t *testing.T) {
	t.Run("LoadUnseen empty", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		messageCh := make(chan *p2p.PMMessage)
		pm := p2p.NewPeerManagerMock(s, messageCh)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())
		processor.LoadUnseen()
		assert.Equal(t, 0, processor.tx2ChMap.Len())
	})

	t.Run("LoadUnseen", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)
		setStoreTestData(t, s)

		messageCh := make(chan *p2p.PMMessage)
		pm := p2p.NewPeerManagerMock(s, messageCh)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())
		processor.LoadUnseen()
		assert.Equal(t, 2, processor.tx2ChMap.Len())
		items := processor.tx2ChMap.Items()
		assert.Equal(t, tx1Bytes, items[tx1].Hash)
		assert.Equal(t, tx2Bytes, items[tx2].Hash)
	})
}

func TestProcessTransaction(t *testing.T) {
	t.Run("ProcessTransaction", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		messageCh := make(chan *p2p.PMMessage)
		pm := p2p.NewPeerManagerMock(s, messageCh)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		expectedResponses := []metamorph_api.Status{
			metamorph_api.Status_RECEIVED,
			metamorph_api.Status_STORED,
			metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		}

		responseChannel := make(chan *ProcessorResponse)

		var wg sync.WaitGroup
		wg.Add(len(expectedResponses))
		go func() {
			n := 0
			for response := range responseChannel {
				status := response.GetStatus()
				assert.Equal(t, tx1Bytes, response.Hash)
				assert.Equalf(t, expectedResponses[n], status, "Iteration %d: Expected %s, got %s", n, expectedResponses[n].String(), status.String())
				wg.Done()
				n++
			}
		}()

		processor.ProcessTransaction(NewProcessorRequest(
			&store.StoreData{
				Hash: tx1Bytes,
			},
			responseChannel,
		))
		wg.Wait()

		assert.Equal(t, 1, processor.tx2ChMap.Len())
		items := processor.tx2ChMap.Items()
		assert.Equal(t, tx1Bytes, items[tx1].Hash)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, items[tx1].status)

		assert.Len(t, pm.Announced, 1)
		assert.Equal(t, tx1Bytes, pm.Announced[0])

		assert.Len(t, s.Store, 1)
		txStored, err := s.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, tx1Bytes, txStored.Hash)
	})
}

func TestSendStatusForTransaction(t *testing.T) {
	t.Run("SendStatusForTransaction unknown tx", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(tx1, metamorph_api.Status_SEEN_ON_NETWORK, nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.tx2ChMap.Len())
		assert.Len(t, s.Store, 0)
	})

	t.Run("SendStatusForTransaction err", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		throwErr := fmt.Errorf("some error")
		ok, sendErr := processor.SendStatusForTransaction(tx1, metamorph_api.Status_REJECTED, throwErr)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		txStored, err := s.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, txStored.Status)
		assert.Equal(t, throwErr.Error(), txStored.RejectReason)
	})

	t.Run("SendStatusForTransaction invalid tx id", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		ok, sendErr := processor.SendStatusForTransaction("test", metamorph_api.Status_REJECTED, nil)
		assert.False(t, ok)
		assert.ErrorIs(t, sendErr, hex.InvalidByteError('t'))
	})

	t.Run("SendStatusForTransaction known tx", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(tx1, metamorph_api.Status_MINED, nil)
		assert.True(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		txStored, err := s.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_MINED, txStored.Status)
	})

	t.Run("SendStatusForTransaction known tx - no update", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)
		setStoreTestData(t, s)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(tx1, metamorph_api.Status_ANNOUNCED_TO_NETWORK, nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		txStored, err := s.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, txStored.Status)
	})

	t.Run("SendStatusForTransaction known tx - processed", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		pm := p2p.NewPeerManagerMock(s, nil)

		processor := NewProcessor(1, s, pm)
		assert.Equal(t, 0, processor.tx2ChMap.Len())

		responseChannel := make(chan *ProcessorResponse)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for response := range responseChannel {
				status := response.GetStatus()
				fmt.Printf("response: %s\n", status)
				if status == metamorph_api.Status_ANNOUNCED_TO_NETWORK {
					wg.Done()
					close(responseChannel)
					return
				}
			}
		}()

		processor.ProcessTransaction(NewProcessorRequest(
			&store.StoreData{
				Hash: tx1Bytes,
			},
			responseChannel,
		))
		wg.Wait()

		assert.Equal(t, 1, processor.tx2ChMap.Len())

		ok, sendErr := processor.SendStatusForTransaction(tx1, metamorph_api.Status_SEEN_ON_NETWORK, nil)
		assert.False(t, ok)
		assert.NoError(t, sendErr)
		assert.Equal(t, 0, processor.tx2ChMap.Len(), "should have been removed from the map")

		txStored, err := s.Get(context.Background(), tx1Bytes)
		require.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, txStored.Status)
	})
}
