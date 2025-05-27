package p2p

import (
	"context"
	"errors"
	"github.com/libsv/go-p2p/wire"
	"log/slog"
	"sort"
	"sync"
	"time"
)

var ErrPeerNetworkMismatch = errors.New("peer network mismatch")

type PeerManager struct {
	execWg        sync.WaitGroup
	execCtx       context.Context
	cancelExecCtx context.CancelFunc

	l       *slog.Logger
	network wire.BitcoinNet

	mu    sync.RWMutex
	peers []PeerI

	restartUnhealthyPeers bool
}

func NewPeerManager(logger *slog.Logger, network wire.BitcoinNet, options ...PeerManagerOptions) *PeerManager {
	ctx, cancelFn := context.WithCancel(context.Background())

	m := &PeerManager{
		execCtx:       ctx,
		cancelExecCtx: cancelFn,

		network: network,
		l:       logger,
	}

	for _, opt := range options {
		opt(m)
	}

	return m
}

func (m *PeerManager) AddPeer(peer PeerI) error {
	if peer.Network() != m.network {
		return ErrPeerNetworkMismatch
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.peers = append(m.peers, peer)

	if m.restartUnhealthyPeers {
		m.startMonitorPeerHealth(peer)
	}

	return nil
}

func (m *PeerManager) RemovePeer(peer PeerI) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	index := -1
	for i, p := range m.peers {
		if p == peer {
			index = i
			break
		}
	}

	if index != -1 {
		m.peers = append(m.peers[:index], m.peers[index+1:]...)
	}

	return index != -1
}
func (m *PeerManager) GetPeers() []PeerI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peersCopy := make([]PeerI, len(m.peers))
	copy(peersCopy, m.peers)

	return peersCopy
}

func (m *PeerManager) CountConnectedPeers() uint {
	m.mu.RLock()
	c := uint(0)

	for _, p := range m.peers {
		if p.Connected() {
			c++
		}
	}

	m.mu.RUnlock()
	return c
}

func (m *PeerManager) Shutdown() {
	m.l.Info("Shutting down peer manager")

	m.cancelExecCtx()
	m.execWg.Wait()

	for _, peer := range m.peers {
		peer.Shutdown()
	}
}

func (m *PeerManager) GetPeersForAnnouncement() []PeerI {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get a list of peers that are connected
	connectedPeers := make([]PeerI, 0, len(m.peers))
	for _, peer := range m.peers {
		if peer.Connected() {
			connectedPeers = append(connectedPeers, peer)
		}
	}

	// sort peers by address
	sort.SliceStable(connectedPeers, func(i, j int) bool {
		return connectedPeers[i].String() < connectedPeers[j].String()
	})

	// send to a subset of peers to be able to listen on the rest
	sendToPeers := make([]PeerI, 0, len(connectedPeers))
	for _, peer := range connectedPeers {
		if len(connectedPeers) > 1 && len(sendToPeers) >= (len(connectedPeers)+1)/2 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	return sendToPeers
}

func (m *PeerManager) startMonitorPeerHealth(peer PeerI) {
	m.l.Info("Starting peer health monitoring", slog.String("peer", peer.String()))
	m.execWg.Add(1)

	go func(p PeerI) {
		defer m.execWg.Done()

		for {
			select {
			case <-m.execCtx.Done():
				return

			case <-p.IsUnhealthyCh():
				m.l.Warn("Peer unhealthy - restarting", slog.String("peer", peer.String()))
				if m.restartPeers(p, peer) {
					return
				}
			}
		}
	}(peer)
}

func (m *PeerManager) restartPeers(p PeerI, peer PeerI) bool {
	for {
		select {
		case <-m.execCtx.Done():
			return true
		default:
			success := p.Restart()
			if success {
				return false
			}
			m.l.Error("Peer restart failed", slog.String("peer", peer.String()))
			time.Sleep(5 * time.Second)

			m.l.Warn("Try restart peer again", slog.String("peer", peer.String()))
		}
	}
}
