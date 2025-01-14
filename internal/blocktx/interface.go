package blocktx

import (
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

type BlockRequest struct {
	Hash *chainhash.Hash
	Peer p2p.PeerI
}

type Peer interface {
	Connected() bool
	WriteMsg(msg wire.Message) error
	String() string
	AnnounceTransaction(txHash *chainhash.Hash)
	RequestTransaction(txHash *chainhash.Hash)
	AnnounceBlock(blockHash *chainhash.Hash)
	RequestBlock(blockHash *chainhash.Hash)
	Network() wire.BitcoinNet
	IsHealthy() bool
	IsUnhealthyCh() <-chan struct{}
	Shutdown()
	Restart()
}

type PeerManager interface {
	AnnounceTransaction(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI
	RequestTransaction(txHash *chainhash.Hash) p2p.PeerI
	AnnounceBlock(blockHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI
	RequestBlock(blockHash *chainhash.Hash) p2p.PeerI
	AddPeer(peer p2p.PeerI) error
	GetPeers() []p2p.PeerI
	Shutdown()
}
