package p2p

import (
	"net"
	"time"

	"github.com/libsv/go-p2p/wire"
)

type PeerOptions func(p *Peer)

func WithMaximumMessageSize(maximumMessageSize int64) PeerOptions {
	return func(p *Peer) {
		p.maxMsgSize = maximumMessageSize
	}
}

func WithReadBufferSize(size int) PeerOptions {
	return func(p *Peer) {
		p.readBuffSize = size
	}
}

func WithUserAgent(userAgentName string, userAgentVersion string) PeerOptions {
	return func(p *Peer) {
		p.userAgentName = &userAgentName
		p.userAgentVersion = &userAgentVersion
	}
}

func WithNrOfWriteHandlers(n uint8) PeerOptions {
	return func(p *Peer) {
		p.nWriters = n
	}
}

func WithWriteChannelSize(n uint16) PeerOptions {
	return func(p *Peer) {
		p.writeCh = make(chan wire.Message, n)
	}
}

func WithPingInterval(interval time.Duration, connectionHealthThreshold time.Duration) PeerOptions {
	return func(p *Peer) {
		p.pingInterval = interval
		p.healthThreshold = connectionHealthThreshold
	}
}

func WithServiceFlag(flag wire.ServiceFlag) PeerOptions {
	return func(p *Peer) {
		p.servicesFlag = flag
	}
}

func WithDialer(dial func(network, address string) (net.Conn, error)) PeerOptions {
	return func(p *Peer) {
		p.dial = dial
	}
}
