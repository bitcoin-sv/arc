package p2p_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/bitcoin-sv/arc/internal/p2p/mocks"
)

const peerManagerNetwork = wire.TestNet

func Test_PeerManagerAddPeer(t *testing.T) {
	tt := []struct {
		name          string
		peerNetwork   wire.BitcoinNet
		expectedError error
	}{
		{
			name:        "Add peer with matching network",
			peerNetwork: peerManagerNetwork,
		},
		{
			name:          "Add peer with mismatched network",
			peerNetwork:   wire.MainNet,
			expectedError: p2p.ErrPeerNetworkMismatch,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			peerMq := &mocks.PeerIMock{
				NetworkFunc: func() wire.BitcoinNet { return tc.peerNetwork },
			}

			sut := p2p.NewPeerManager(slog.Default(), wire.TestNet)

			// when
			err := sut.AddPeer(peerMq)

			// then
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Len(t, sut.GetPeers(), 1)
			} else {
				require.ErrorIs(t, err, p2p.ErrPeerNetworkMismatch)
				require.Len(t, sut.GetPeers(), 0)
			}
		})
	}
}

func Test_PeerManagerRemovePeer(t *testing.T) {
	t.Run("Remove an existing peer", func(t *testing.T) {
		// given
		peerMq := &mocks.PeerIMock{
			NetworkFunc: func() wire.BitcoinNet { return peerManagerNetwork },
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)
		err := sut.AddPeer(peerMq)
		require.NoError(t, err)

		// when
		removed := sut.RemovePeer(peerMq)

		// then
		require.True(t, removed)
		require.Len(t, sut.GetPeers(), 0)
	})

	t.Run("Remove a non-existent peer", func(t *testing.T) {
		// given
		peerMq := &mocks.PeerIMock{
			NetworkFunc: func() wire.BitcoinNet { return peerManagerNetwork },
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)

		// when
		removed := sut.RemovePeer(peerMq)

		// then
		require.False(t, removed)
		require.Len(t, sut.GetPeers(), 0)
	})
}

func Test_PeerManagerCountConnectedPeers(t *testing.T) {
	t.Run("Count connected peers", func(t *testing.T) {
		// given
		connectedPeerMq := &mocks.PeerIMock{
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
		}
		notConnectedPeerMq := &mocks.PeerIMock{
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return false },
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)

		err := sut.AddPeer(connectedPeerMq)
		require.NoError(t, err)

		err = sut.AddPeer(notConnectedPeerMq)
		require.NoError(t, err)

		// when
		count := sut.CountConnectedPeers()

		// then
		require.Equal(t, uint(1), count)
	})
}

func Test_PeerManagerShutdown(t *testing.T) {
	t.Run("Shutdown peer manager - should call shutdown on all peers", func(t *testing.T) {
		// given
		var peers []*mocks.PeerIMock

		for range 10 {
			peerMq := &mocks.PeerIMock{
				NetworkFunc:  func() wire.BitcoinNet { return peerManagerNetwork },
				ShutdownFunc: func() {},
			}

			peers = append(peers, peerMq)
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)

		for _, p := range peers {
			err := sut.AddPeer(p)
			require.NoError(t, err)
		}

		// when
		sut.Shutdown()

		// then
		for _, p := range peers {
			require.Equal(t, 1, len(p.ShutdownCalls()), "PeerManager didn't call Shutdown() on its peers")
		}
	})
}

func Test_PeerManagerGetPeersForAnnouncement(t *testing.T) {
	t.Run("Get connected peers for announcement", func(t *testing.T) {
		// given
		connectedPeerMq1 := &mocks.PeerIMock{
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
			StringFunc:    func() string { return "peer1" },
		}
		connectedPeerMq2 := &mocks.PeerIMock{
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return true },
			StringFunc:    func() string { return "peer2" },
		}
		notConnectedPeerMq := &mocks.PeerIMock{
			NetworkFunc:   func() wire.BitcoinNet { return peerManagerNetwork },
			ConnectedFunc: func() bool { return false },
			StringFunc:    func() string { return "peer3" },
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork)

		err := sut.AddPeer(connectedPeerMq1)
		require.NoError(t, err)

		err = sut.AddPeer(connectedPeerMq2)
		require.NoError(t, err)

		err = sut.AddPeer(notConnectedPeerMq)
		require.NoError(t, err)

		// when
		announcementPeers := sut.GetPeersForAnnouncement()

		// then
		require.Len(t, announcementPeers, 1) // should pick approximately half of the connected peers
	})
}

func Test_PeerManagerPeerHealthMonitoring(t *testing.T) {
	t.Run("Restart unhealthy peer", func(t *testing.T) {
		// given
		unhealthyCh := make(chan struct{}, 1)

		peer := &mocks.PeerIMock{
			NetworkFunc:       func() wire.BitcoinNet { return peerManagerNetwork },
			RestartFunc:       func() bool { return true },
			StringFunc:        func() string { return "peer1" },
			IsUnhealthyChFunc: func() <-chan struct{} { return unhealthyCh },
		}

		sut := p2p.NewPeerManager(slog.Default(), peerManagerNetwork, p2p.WithRestartUnhealthyPeers())

		err := sut.AddPeer(peer)
		require.NoError(t, err)

		// when
		// peer signals it's unhealthy
		unhealthyCh <- struct{}{}

		// give some time for the restart process to be triggered
		time.Sleep(100 * time.Millisecond)

		// then
		require.Equal(t, 1, len(peer.RestartCalls()))
	})
}

func Test_PeerManagerUnhealthyPeers(t *testing.T) {
	t.Run("Recover unhealthy peers", func(t *testing.T) {
		restarts := 0

		unhealthyPeer := &mocks.PeerIMock{
			ConnectedFunc: func() bool {
				if restarts <= 2 {
					restarts++
					return false
				}
				return true
			},
			RestartFunc: func() bool {
				return true
			},
		}

		connected := unhealthyPeer.Connected()
		require.Equal(t, false, connected, "Unhealthy peer should not connect")

		pm := p2p.NewPeerManager(slog.Default(), peerManagerNetwork, p2p.WithRestartUnhealthyPeers(), p2p.SetPeerCheckInterval(50*time.Millisecond), p2p.SetReconnectDelay(50*time.Millisecond))
		err := pm.AddPeer(unhealthyPeer)
		require.NoError(t, err)
		require.Len(t, pm.GetPeers(), 2)

		// give some time for the peer manager to monitor the unhealthy peer
		time.Sleep(70 * time.Millisecond)
		require.Equal(t, false, unhealthyPeer.Connected())
		require.Equal(t, 1, len(unhealthyPeer.RestartCalls()), "Unhealthy peer should be restarted once")

		// let reconnecting delay pass
		time.Sleep(50 * time.Millisecond)
		require.Equal(t, true, unhealthyPeer.Connected())
	})
}
