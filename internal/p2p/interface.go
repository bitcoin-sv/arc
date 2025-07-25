package p2p

import (
	"github.com/libsv/go-p2p/wire"
)

type PeerI interface {
	Restart() (ok bool)
	Shutdown()
	Connected() bool
	Connect() bool
	IsUnhealthyCh() <-chan struct{}
	WriteMsg(msg wire.Message)
	Network() wire.BitcoinNet
	String() string
}

type MessageHandlerI interface {
	// OnReceive handles incoming messages depending on command type
	OnReceive(msg wire.Message, peer PeerI)
	// OnSend handles outgoing messages depending on command type
	OnSend(msg wire.Message, peer PeerI)
}
