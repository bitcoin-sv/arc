package ignore_p2p

import (
	"github.com/libsv/go-p2p/wire"
)

type PeerI interface {
	Restart() (ok bool)
	Shutdown()
	Connected() bool
	IsUnhealthyCh() <-chan struct{}
	WriteMsg(msg wire.Message)
	Network() wire.BitcoinNet
	String() string
}

type MessageHandlerI interface {
	// should be fire & forget
	OnReceive(msg wire.Message, peer PeerI)
	// should be fire & forget
	OnSend(msg wire.Message, peer PeerI)
}
