package bcnet

import (
	"io"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

// BlockMessage only stores the transaction IDs of the block, not the full transactions
type BlockMessage struct {
	Hash              *chainhash.Hash
	Header            *wire.BlockHeader
	Height            uint64
	TransactionHashes []*chainhash.Hash
	Size              uint64
}

func (bm *BlockMessage) Bsvdecode(io.Reader, uint32, wire.MessageEncoding) error {
	return nil
}
func (bm *BlockMessage) BsvEncode(io.Writer, uint32, wire.MessageEncoding) error {
	return nil
}
func (bm *BlockMessage) Command() string {
	return "block"
}
func (bm *BlockMessage) MaxPayloadLength(uint32) uint64 {
	return wire.MaxExtMsgPayload
}

type BlockMessagePeer struct {
	BlockMessage
	Peer string
}
