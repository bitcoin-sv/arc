package blocktx_p2p

import (
	"errors"
	"log/slog"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
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
	blockProcessingCh chan<- *bcnet.BlockMessagePeer
}

func NewMsgHandler(l *slog.Logger, blockRequestCh chan<- BlockRequest, blockProcessCh chan<- *bcnet.BlockMessagePeer) *MsgHandler {
	return &MsgHandler{
		logger: l.With(
			slog.String("module", "peer-msg-handler"),
			slog.String("mode", "classic"),
		),
		blockRequestingCh: blockRequestCh,
		blockProcessingCh: blockProcessCh,
	}
}

// OnReceive handles incoming messages depending on command type
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
		blockMsg, ok := msg.(*bcnet.BlockMessage)
		if !ok {
			h.logger.Error("Block msg receive", slog.Any("err", ErrUnableToCastWireMessage))
			return
		}

		blockMsgPeer := &bcnet.BlockMessagePeer{
			BlockMessage: *blockMsg,
		}

		if peer != nil {
			blockMsgPeer.Peer = peer.String()
		}

		h.blockProcessingCh <- blockMsgPeer

	default:
		// ignore other messages
	}
}

// OnSend handles outgoing messages depending on command type
func (h *MsgHandler) OnSend(_ wire.Message, _ p2p.PeerI) {
	// ignore
}
