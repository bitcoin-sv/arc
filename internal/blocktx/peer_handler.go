package blocktx

import (
	"errors"
	"io"
	"log/slog"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/util"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

func init() {
	// override the default wire block handler with our own that streams and stores only the transaction ids
	wire.SetExternalHandler(wire.CmdBlock, func(reader io.Reader, length uint64, bytesRead int) (int, wire.Message, []byte, error) {
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
				blockMessage.Height = ExtractHeightFromCoinbaseTx(tx)
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

func (ph *PeerHandler) HandleTransactionsGet(_ []*wire.InvVect, peer p2p.PeerI) ([][]byte, error) {
	return nil, nil
}

func (ph *PeerHandler) HandleTransactionSent(_ *wire.MsgTx, peer p2p.PeerI) error {
	return nil
}

func (ph *PeerHandler) HandleTransactionAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
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
		errMsg := "unable to cast wire.Message to p2p.BlockMessage"
		ph.logger.Debug(errMsg)
		return errors.New(errMsg)
	}

	ph.blockProcessCh <- msg

	return nil
}
