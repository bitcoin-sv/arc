package p2p_test

import (
	"log/slog"
	"testing"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/ccoveille/go-safecast"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/p2p/mocks"
)

var (
	txHash, _ = chainhash.NewHashFromStr("88eab41a8d0b7b4bc395f8f988ea3d6e63c8bc339526fd2f00cb7ce6fd7df0f7")
)

func Test_AnnounceTransactions(t *testing.T) {
	testutils.RunParallel(t, true, "Announce transactions to specified peers", func(t *testing.T) {
		// given
		peer1 := &mocks.PeerIMock{
			WriteMsgFunc: func(_ wire.Message) {},
			StringFunc:   func() string { return "peer1" },
		}
		peer2 := &mocks.PeerIMock{
			WriteMsgFunc: func(_ wire.Message) {},
			StringFunc:   func() string { return "peer2" }}
		peer3 := &mocks.PeerIMock{
			WriteMsgFunc: func(_ wire.Message) {},
			StringFunc:   func() string { return "peer3" },
			NetworkFunc:  func() wire.BitcoinNet { return peerManagerNetwork },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer3)
		require.NoError(t, err)

		txHashes := []*chainhash.Hash{txHash}

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		peers := sut.AnnounceTransactions(txHashes, []p2p.PeerI{peer1, peer2})

		// then
		require.Len(t, peers, 2)

		require.Equal(t, peer1.String(), peers[0].String())
		require.Len(t, peer1.WriteMsgCalls(), 1)

		require.Equal(t, peer2.String(), peers[1].String())
		require.Len(t, peer1.WriteMsgCalls(), 1)

		require.Len(t, peer3.WriteMsgCalls(), 0, "Network messagner shouldn't write msg to its peer if it wasn't specified")
	})

	testutils.RunParallel(t, true, "Announce transactions to default peers if none specified", func(t *testing.T) {
		// given
		peer := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer)
		require.NoError(t, err)

		txHashes := []*chainhash.Hash{txHash}

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		peers := sut.AnnounceTransactions(txHashes, nil)

		// then
		require.Len(t, peers, 1)
		require.Equal(t, peer.String(), peers[0].String())
		require.Len(t, peer.WriteMsgCalls(), 1)
	})

	// test INV batching
	tt := []struct {
		name            string
		numOfTxs        uint
		expecedNumOfMsg uint
	}{
		{
			name:            "Announce 1 transaction",
			numOfTxs:        1,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Announce less transactions than batch size (<512)",
			numOfTxs:        137,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Announce exact number of transactions as batch size (512)",
			numOfTxs:        512,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Announce more transactions than batch size (>512)",
			numOfTxs:        513,
			expecedNumOfMsg: 2,
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			// given
			var txHashes []*chainhash.Hash
			for range tc.numOfTxs {
				txHashes = append(txHashes, txHash)
			}

			peer := &mocks.PeerIMock{
				WriteMsgFunc:  func(_ wire.Message) {},
				StringFunc:    func() string { return "peer" },
				NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
				ConnectedFunc: func() bool { return true },
			}

			pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
			err := pm.AddPeer(peer)
			require.NoError(t, err)

			sut := p2p.NewNetworkMessenger(slog.Default(), pm)

			// when
			peers := sut.AnnounceTransactions(txHashes, nil)

			// then
			require.Len(t, peers, 1)
			require.Equal(t, peer.String(), peers[0].String())
			enom, err := safecast.ToInt(tc.expecedNumOfMsg)
			assert.NoError(t, err)
			require.Len(t, peer.WriteMsgCalls(), enom)
		})
	}
}

func Test_RequestTransactions(t *testing.T) {
	testutils.RunParallel(t, true, "Request transactions from first connected peer", func(t *testing.T) {
		// given
		peer := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer)
		require.NoError(t, err)

		txHashes := []*chainhash.Hash{txHash}

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		sentPeer := sut.RequestTransactions(txHashes)

		// then
		require.NotNil(t, sentPeer)
		require.Len(t, peer.WriteMsgCalls(), 1)
	})

	testutils.RunParallel(t, true, "Return nil if no connected peers", func(t *testing.T) {
		// given
		peer := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return false },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer)
		require.NoError(t, err)

		txHashes := []*chainhash.Hash{txHash}

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		sentPeer := sut.RequestTransactions(txHashes)

		// then
		require.Nil(t, sentPeer)
		require.Len(t, peer.WriteMsgCalls(), 0)
	})

	// test GETDATA batching
	tt := []struct {
		name            string
		numOfTxs        uint
		expecedNumOfMsg uint
	}{
		{
			name:            "Request 1 transaction",
			numOfTxs:        1,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Request less transactions than batch size (<512)",
			numOfTxs:        137,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Request exact number of transactions as batch size (512)",
			numOfTxs:        512,
			expecedNumOfMsg: 1,
		},
		{
			name:            "Request more transactions than batch size (>512)",
			numOfTxs:        513,
			expecedNumOfMsg: 2,
		},
	}

	for _, tc := range tt {
		testutils.RunParallel(t, true, tc.name, func(t *testing.T) {
			// given
			var txHashes []*chainhash.Hash
			for range tc.numOfTxs {
				txHashes = append(txHashes, txHash)
			}

			peer := &mocks.PeerIMock{
				WriteMsgFunc:  func(_ wire.Message) {},
				StringFunc:    func() string { return "peer" },
				NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
				ConnectedFunc: func() bool { return true },
			}

			pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
			err := pm.AddPeer(peer)
			require.NoError(t, err)

			sut := p2p.NewNetworkMessenger(slog.Default(), pm)

			// when
			sentPeer := sut.RequestTransactions(txHashes)

			// then
			require.NotNil(t, sentPeer)
			enom, err := safecast.ToInt(tc.expecedNumOfMsg)
			assert.NoError(t, err)
			require.Len(t, peer.WriteMsgCalls(), enom)
		})
	}
}

func Test_AnnounceBlock(t *testing.T) {
	testutils.RunParallel(t, true, "Announce block to specified peers", func(t *testing.T) {
		// given
		peer1 := &mocks.PeerIMock{
			WriteMsgFunc: func(_ wire.Message) {},
			StringFunc:   func() string { return "peer1" },
		}
		peer2 := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer2" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer2)
		require.NoError(t, err)

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		peers := sut.AnnounceBlock(blockHash, []p2p.PeerI{peer1})

		// then
		require.Len(t, peers, 1)
		require.Equal(t, peer1.String(), peers[0].String())
		require.Len(t, peer1.WriteMsgCalls(), 1)

		require.Len(t, peer2.WriteMsgCalls(), 0, "Network messagner shouldn't write msg to its peer if it wasn't specified")
	})
}

func Test_RequestBlock(t *testing.T) {
	testutils.RunParallel(t, true, "Request block from first connected peer", func(t *testing.T) {
		// given
		peer := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(peer)
		require.NoError(t, err)

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		sentPeer := sut.RequestBlock(blockHash)

		// then
		require.NotNil(t, sentPeer)
		require.Len(t, peer.WriteMsgCalls(), 1)
	})

	testutils.RunParallel(t, true, "Return nil if no connected peers", func(t *testing.T) {
		// given
		notConnectedPeer := &mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			StringFunc:    func() string { return "peer" },
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return false },
		}

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := pm.AddPeer(notConnectedPeer)
		require.NoError(t, err)

		sut := p2p.NewNetworkMessenger(slog.Default(), pm)

		// when
		sentPeer := sut.RequestBlock(blockHash)

		// then
		require.Nil(t, sentPeer)
		require.Len(t, notConnectedPeer.WriteMsgCalls(), 0)
	})
}
