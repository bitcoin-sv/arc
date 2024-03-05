package blocktx

import (
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

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
}
