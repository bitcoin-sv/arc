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
	AddPeer(peerURL string, peerStore PeerStoreI) error
	RemovePeer(peerURL string) error
	GetPeers() []PeerI
	PeerCreator(peerCreator func(peerAddress string, peerStore PeerStoreI) (PeerI, error))
	addPeer(peer PeerI) error
}

type PeerI interface {
	AddParentMessageChannel(parentMessageCh chan *PMMessage) PeerI
	WriteMsg(msg wire.Message)
	String() string
}

type PeerStoreI interface {
	GetTransactionBytes(hash []byte) ([]byte, error)
	HandleBlockAnnouncement(hash []byte, peer PeerI) error
	InsertBlock(blockHash []byte, merkleRootHash []byte, previousBlockHash []byte, height uint64, peer PeerI) (uint64, error)
	MarkTransactionsAsMined(blockId uint64, txHashes [][]byte) error
	MarkBlockAsProcessed(block *Block) error
}
