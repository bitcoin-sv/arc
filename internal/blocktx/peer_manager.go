package blocktx

import (
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type PeerManager interface {
	AnnounceTransaction(txHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI
	RequestTransaction(txHash *chainhash.Hash) p2p.PeerI
	AnnounceBlock(blockHash *chainhash.Hash, peers []p2p.PeerI) []p2p.PeerI
	RequestBlock(blockHash *chainhash.Hash) p2p.PeerI
	AddPeer(peer p2p.PeerI) error
	GetPeers() []p2p.PeerI
	Shutdown()
}
