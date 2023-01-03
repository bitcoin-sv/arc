package p2p

import (
	"fmt"
	"testing"
	"time"

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

func TestNewPeerManager(t *testing.T) {
	t.Run("nil peers no error", func(t *testing.T) {
		pm, pmErr := NewPeerManager(nil, nil, nil)
		require.NoError(t, pmErr)
		require.NotNil(t, pm)
	})

	t.Run("no peers no error", func(t *testing.T) {
		peers := make([]string, 0)
		pm, pmErr := NewPeerManager(nil, peers, nil)
		require.NoError(t, pmErr)
		require.NotNil(t, pm)
	})

	t.Run("1 peer", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		peers := []string{
			"localhost:18333",
		}
		pm, pmErr := NewPeerManager(s, peers, nil)
		require.NoError(t, pmErr)
		require.NotNil(t, pm)
		assert.Len(t, pm.GetPeers(), 1)
	})

	t.Run("1 peer - de dup", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		peers := []string{
			"localhost:18333",
			"localhost:18333",
			"localhost:18333",
			"localhost:18333",
		}
		pm, pmErr := NewPeerManager(s, peers, nil)
		require.NoError(t, pmErr)
		require.NotNil(t, pm)
		assert.Len(t, pm.GetPeers(), 1)
	})
}

func TestAnnounceNewTransaction(t *testing.T) {
	t.Run("announce tx", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		var messageCh chan *PMMessage
		pm, pmErr := NewPeerManager(s, nil, messageCh, 1*time.Millisecond)
		require.NoError(t, pmErr)

		peer, _ := NewPeerMock("localhost:18333", s, messageCh)
		err = pm.addPeer(peer)
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
		s, err := memorystore.New()
		require.NoError(t, err)

		var messageCh chan *PMMessage
		pm, pmErr := NewPeerManager(s, nil, messageCh, 1*time.Millisecond)
		require.NoError(t, pmErr)

		numberOfPeers := 5
		peers := make([]*PeerMock, numberOfPeers)
		for i := 0; i < numberOfPeers; i++ {
			peers[i], _ = NewPeerMock(fmt.Sprintf("localhost:1833%d", i), s, messageCh)
			err = pm.addPeer(peers[i])
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
