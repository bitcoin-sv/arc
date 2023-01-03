package p2p

import (
	"fmt"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/memorystore"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	tx1         = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	tx1Bytes, _ = utils.DecodeAndReverseHexString(tx1)
)

type MockPeerStore struct {
	s store.Store
}

func NewMockPeerStore() *MockPeerStore {
	s, _ := memorystore.New()

	return &MockPeerStore{
		s: s,
	}
}

func (m *MockPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *MockPeerStore) ProcessBlock(hash []byte) error {
	return nil
}

func TestNewPeerManager(t *testing.T) {
	t.Run("nil peers no error", func(t *testing.T) {
		pm := NewPeerManager(nil)
		require.NotNil(t, pm)
	})

	t.Run("1 peer", func(t *testing.T) {
		pm := NewPeerManager(nil)
		require.NotNil(t, pm)

		peerStore := NewMockPeerStore()

		err := pm.AddPeer("localhost:18333", peerStore)
		require.NoError(t, err)
		assert.Len(t, pm.GetPeers(), 1)
	})

	t.Run("1 peer - de dup", func(t *testing.T) {
		peers := []string{
			"localhost:18333",
			"localhost:18333",
			"localhost:18333",
			"localhost:18333",
		}

		pm := NewPeerManager(nil)
		require.NotNil(t, pm)

		peerStore := NewMockPeerStore()

		for _, peer := range peers {
			pm.AddPeer(peer, peerStore)
		}

		assert.Len(t, pm.GetPeers(), 1)
	})
}

func TestAnnounceNewTransaction(t *testing.T) {
	t.Run("announce tx", func(t *testing.T) {

		var messageCh chan *PMMessage

		pm := NewPeerManager(messageCh, 1*time.Millisecond)
		require.NotNil(t, pm)

		peerStore := NewMockPeerStore()

		peer, _ := NewPeerMock("localhost:18333", peerStore, messageCh)
		err := pm.addPeer(peer)
		require.NoError(t, err)

		pm.AnnounceNewTransaction(tx1Bytes)

		// we need to wait for the batcher to send the inv
		time.Sleep(5 * time.Millisecond)

		messages := peer.getMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, "inv", messages[0].Command())
		message, ok := messages[0].(*wire.MsgInv)
		require.True(t, ok)
		assert.Len(t, message.InvList, 1)
		assert.Equal(t, tx1, message.InvList[0].Hash.String())
	})

	t.Run("announce tx - multiple peers", func(t *testing.T) {
		var messageCh chan *PMMessage
		pm := NewPeerManager(messageCh, 1*time.Millisecond)
		require.NotNil(t, pm)

		peerStore := NewMockPeerStore()

		numberOfPeers := 5
		peers := make([]*PeerMock, numberOfPeers)
		for i := 0; i < numberOfPeers; i++ {
			peers[i], _ = NewPeerMock(fmt.Sprintf("localhost:1833%d", i), peerStore, messageCh)
			err := pm.addPeer(peers[i])
			require.NoError(t, err)
		}

		pm.AnnounceNewTransaction(tx1Bytes)

		// we need to wait for the batcher to send the inv
		time.Sleep(5 * time.Millisecond)

		peersMessaged := 0
		for _, peer := range peers {
			if peer.Len() == 0 {
				continue
			}
			messages := peer.getMessages()
			require.Len(t, messages, 1)
			assert.Equal(t, "inv", messages[0].Command())
			message, ok := messages[0].(*wire.MsgInv)
			require.True(t, ok)
			assert.Len(t, message.InvList, 1)
			assert.Equal(t, tx1, message.InvList[0].Hash.String())
			peersMessaged++
		}
		assert.GreaterOrEqual(t, peersMessaged, len(peers)/2)
	})
}

func TestPeerManager_addPeer(t *testing.T) {
}

func TestPeerManager_sendInvBatch(t *testing.T) {
}
