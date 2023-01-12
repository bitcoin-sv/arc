package p2p

import (
	"github.com/TAAL-GmbH/arc/p2p/wire"
)

type Status int32

var (
	StatusSent     Status = 5
	StatusSeen     Status = 6
	StatusRejected Status = 109
)

type PeerManagerI interface {
	AnnounceNewTransaction(txID []byte)
	AddPeer(peerURL string, peerStore PeerHandlerI) error
	RemovePeer(peerURL string) error
	GetPeers() []PeerI
	PeerCreator(peerCreator func(peerAddress string, peerStore PeerHandlerI) (PeerI, error))
	addPeer(peer PeerI) error
}

type PeerI interface {
	Connected() bool
	WriteMsg(msg wire.Message) error
	String() string
}

type PeerHandlerI interface {
	GetTransactionBytes(msg *wire.InvVect) ([]byte, error)
	HandleTransactionSent(msg *wire.MsgTx, peer PeerI) error
	HandleTransactionAnnouncement(msg *wire.InvVect, peer PeerI) error
	HandleTransactionRejection(rejMsg *wire.MsgReject, peer PeerI) error
	HandleBlockAnnouncement(msg *wire.InvVect, peer PeerI) error
	HandleBlock(msg *wire.MsgBlock, peer PeerI) error
}
