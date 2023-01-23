package p2p

import (
	"io"

	"github.com/TAAL-GmbH/arc/p2p/wire"
)

// BlockMessage only stores the transaction IDs of the block, not the full transactions
type BlockMessage struct {
	Header         *wire.BlockHeader
	Height         uint64
	TransactionIDs [][]byte
	Size           uint64
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
