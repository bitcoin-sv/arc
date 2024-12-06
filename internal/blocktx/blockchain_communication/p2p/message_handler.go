package blocktx_p2p

import (
	"errors"
	"log/slog"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"

	blockchain "github.com/bitcoin-sv/arc/internal/blocktx/blockchain_communication"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

var ErrUnableToCastWireMessage = errors.New("unable to cast wire.Message to blockchain.BlockMessage")

type BlockRequest struct {
	Hash *chainhash.Hash
	Peer p2p.PeerI
}

var _ p2p.MessageHandlerI = (*MsgHandler)(nil)

type MsgHandler struct {
	logger            *slog.Logger
	blockRequestingCh chan<- BlockRequest
	blockProcessingCh chan<- *blockchain.BlockMessage
}

func NewMsgHandler(logger *slog.Logger, blockRequestCh chan<- BlockRequest, blockProcessCh chan<- *blockchain.BlockMessage) *MsgHandler {
	return &MsgHandler{
		logger:            logger.With(slog.String("module", "peer-msg-handler")),
		blockRequestingCh: blockRequestCh,
		blockProcessingCh: blockProcessCh,
	}
}

// OnReceive should be fire & forget
func (h *MsgHandler) OnReceive(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()

	switch cmd {
	case wire.CmdInv:
		invMsg, ok := msg.(*wire.MsgInv)
		if !ok {
			return
		}

		go func() {
			for _, iv := range invMsg.InvList {
				if iv.Type == wire.InvTypeBlock {
					req := BlockRequest{
						Hash: &iv.Hash,
						Peer: peer,
					}

					h.blockRequestingCh <- req
				}
				// ignore INV with transaction or error
			}
		}()

	case wire.CmdBlock:
		blockMsg, ok := msg.(*blockchain.BlockMessage)
		if !ok {
			h.logger.Error("Block msg receive", slog.Any("err", ErrUnableToCastWireMessage))
			return
		}

		h.blockProcessingCh <- blockMsg

	default:
		// ignore other messages
	}
}

func (h *MsgHandler) OnSend(_ wire.Message, _ p2p.PeerI) {
	// ignore
}
