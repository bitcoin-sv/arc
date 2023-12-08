package p2p

import (
	"fmt"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

var (
	ErrPeerNetworkMismatch = fmt.Errorf("peer network mismatch")
)

type Pair struct {
	Hash *chainhash.Hash
	Peer PeerI
}

type PeerManagerI interface {
	AnnounceTransaction(txHash *chainhash.Hash, peers []PeerI) []PeerI
	RequestTransaction(txHash *chainhash.Hash) PeerI
	AnnounceBlock(blockHash *chainhash.Hash, peers []PeerI) []PeerI
	RequestBlock(blockHash *chainhash.Hash) PeerI
	AddPeer(peer PeerI) error
	GetPeers() []PeerI
}

type PeerI interface {
	Connected() bool
	WriteMsg(msg wire.Message) error
	String() string
	AnnounceTransaction(txHash *chainhash.Hash)
	RequestTransaction(txHash *chainhash.Hash)
	AnnounceBlock(blockHash *chainhash.Hash)
	RequestBlock(blockHash *chainhash.Hash)
	Network() wire.BitcoinNet
}

type PeerHandlerI interface {
	HandleTransactionGet(msg *wire.InvVect, peer PeerI) ([]byte, error)
	HandleTransactionSent(msg *wire.MsgTx, peer PeerI) error
	HandleTransactionAnnouncement(msg *wire.InvVect, peer PeerI) error
	HandleTransactionRejection(rejMsg *wire.MsgReject, peer PeerI) error
	HandleTransaction(msg *wire.MsgTx, peer PeerI) error
	HandleBlockAnnouncement(msg *wire.InvVect, peer PeerI) error
	HandleBlock(msg wire.Message, peer PeerI) error
}

type PeerTxMessage struct {
	Start  time.Time
	Hash   *chainhash.Hash
	Status int32
	Peer   string
	Err    error
}

type Block struct {
	Hash         *chainhash.Hash `json:"hash,omitempty"`          // Little endian
	PreviousHash *chainhash.Hash `json:"previous_hash,omitempty"` // Little endian
	MerkleRoot   *chainhash.Hash `json:"merkle_root,omitempty"`   // Little endian
	Height       uint64          `json:"height,omitempty"`
	Size         uint64          `json:"size,omitempty"`
	TxCount      uint64          `json:"tx_count,omitempty"`
}
