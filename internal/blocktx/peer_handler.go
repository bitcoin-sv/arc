package blocktx

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"

	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

var ErrUnableToCastWireMessage = errors.New("unable to cast wire.Message to p2p.BlockMessage")

func init() {
	// override the default wire block handler with our own that streams and stores only the transaction ids
	wire.SetExternalHandler(wire.CmdBlock, func(reader io.Reader, _ uint64, bytesRead int) (int, wire.Message, []byte, error) {
		blockMessage := &p2p.BlockMessage{
			Header: &wire.BlockHeader{},
		}

		err := blockMessage.Header.Deserialize(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += 80 // the bitcoin header is always 80 bytes

		var read int64
		var txCount sdkTx.VarInt
		read, err = txCount.ReadFrom(reader)
		if err != nil {
			return bytesRead, nil, nil, err
		}
		bytesRead += int(read)

		blockMessage.TransactionHashes = make([]*chainhash.Hash, txCount)

		var tx *sdkTx.Transaction
		var hash *chainhash.Hash
		var txBytes []byte
		for i := 0; i < int(txCount); i++ {
			tx = sdkTx.NewTransaction()
			read, err = tx.ReadFrom(reader)
			if err != nil {
				return bytesRead, nil, nil, err
			}
			bytesRead += int(read)
			txBytes = tx.TxIDBytes() // this returns the bytes in BigEndian
			hash, err = chainhash.NewHash(util.ReverseBytes(txBytes))
			if err != nil {
				return 0, nil, nil, err
			}

			blockMessage.TransactionHashes[i] = hash

			if i == 0 {
				blockMessage.Height = extractHeightFromCoinbaseTx(tx)
			}
		}

		blockMessage.Size = uint64(bytesRead)

		return bytesRead, blockMessage, nil, nil
	})
}

type PeerHandler struct {
	logger         *slog.Logger
	blockRequestCh chan BlockRequest
	blockProcessCh chan *p2p.BlockMessage
}

func NewPeerHandler(logger *slog.Logger, blockRequestCh chan BlockRequest, blockProcessCh chan *p2p.BlockMessage) *PeerHandler {
	return &PeerHandler{
		logger:         logger.With(slog.String("module", "peer-handler")),
		blockRequestCh: blockRequestCh,
		blockProcessCh: blockProcessCh,
	}
}

func (ph *PeerHandler) HandleTransactionsGet(_ []*wire.InvVect, _ p2p.PeerI) ([][]byte, error) {
	return nil, nil
}

func (ph *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, _ p2p.PeerI) error {
	return nil
}

func (ph *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, _ p2p.PeerI) error {
	return nil
}

func (ph *PeerHandler) HandleTransactionRejection(_ *wire.MsgReject, _ p2p.PeerI) error {
	return nil
}

func (ph *PeerHandler) HandleTransaction(_ *wire.MsgTx, _ p2p.PeerI) error {
	return nil
}

func (ph *PeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	req := BlockRequest{
		Hash: &msg.Hash,
		Peer: peer,
	}

	ph.blockRequestCh <- req
	return nil
}

func (ph *PeerHandler) HandleBlock(wireMsg wire.Message, _ p2p.PeerI) error {
	msg, ok := wireMsg.(*p2p.BlockMessage)
	if !ok {
		return ErrUnableToCastWireMessage
	}

	ph.blockProcessCh <- msg
	return nil
}

func extractHeightFromCoinbaseTx(tx *sdkTx.Transaction) uint64 {
	// Coinbase tx has a special format, the height is encoded in the first 4 bytes of the scriptSig (BIP-34)

	cscript := *(tx.Inputs[0].UnlockingScript)

	if cscript[0] >= script.Op1 && cscript[0] <= script.Op16 { // first 16 blocks are treated differently (we have to handle it due to our tests)
		return uint64(cscript[0] - 0x50)
	}

	hLenght := int(cscript[0])
	if len(cscript) < hLenght+1 {
		return 0
	}

	b := make([]byte, 8)
	for i := 0; i < hLenght; i++ {
		b[i] = cscript[i+1]
	}

	return binary.LittleEndian.Uint64(b)
}
