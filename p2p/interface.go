package p2p

import (
	"github.com/TAAL-GmbH/arc/p2p/wire"
)

type PeerManagerI interface {
	AnnounceNewTransaction(txID []byte)
	AddPeer(peerURL string, peerStore PeerStoreI) error
	RemovePeer(peerURL string) error
	GetPeers() []PeerI
	addPeer(peer PeerI) error
}

type PeerI interface {
	WriteMsg(msg wire.Message)
	String() string
}
