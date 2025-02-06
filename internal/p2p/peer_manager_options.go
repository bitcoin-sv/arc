package p2p

import "github.com/libsv/go-p2p/wire"

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
