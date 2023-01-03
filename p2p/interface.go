package p2p

import "github.com/TAAL-GmbH/arc/p2p/wire"

type PeerManagerI interface {
	AnnounceNewTransaction(txID []byte)
	AddPeer(peerURL string) error
	RemovePeer(peerURL string) error
	GetPeers() []string
	addPeer(peer PeerI) error
}

type PeerI interface {
	WriteChan() chan wire.Message
	String() string
}
