package p2p

import (
	"time"

	"github.com/libsv/go-p2p/wire"
)

type PeerManagerOptions func(p *PeerManager)

func WithRestartUnhealthyPeers() PeerManagerOptions {
	return func(p *PeerManager) {
		p.restartUnhealthyPeers = true
	}
}

// SetExcessiveBlockSize sets global setting for block size
func SetExcessiveBlockSize(ebs uint64) {
	wire.SetLimits(ebs)
}

func SetPeerCheckInterval(interval time.Duration) PeerManagerOptions {
	return func(p *PeerManager) {
		p.peerCheckInterval = interval
	}
}

func SetReconnectDelay(delay time.Duration) PeerManagerOptions {
	return func(p *PeerManager) {
		p.reconnectDelay = delay
	}
}
