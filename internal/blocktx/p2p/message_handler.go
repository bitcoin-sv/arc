package blocktx_p2p

import (
	"log/slog"

	"github.com/libsv/go-p2p/better_p2p"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

var _ better_p2p.MessageHandlerI = (*PeerMsgHandler)(nil)

type PeerMsgHandler struct {
	l         *slog.Logger
	requestCh chan<- BlockRequest
	processCh chan<- *BlockMessage
}

type BlockRequest struct {
	Hash *chainhash.Hash
	Peer better_p2p.PeerI
}

func NewPeerMsgHandler(logger *slog.Logger, blockRequestCh chan<- BlockRequest, blockProcessCh chan<- *BlockMessage) *PeerMsgHandler {
	return &PeerMsgHandler{
		l:         logger.With(slog.String("module", "peer-msg-handler")),
		requestCh: blockRequestCh,
		processCh: blockProcessCh,
	}
}

func (h *PeerMsgHandler) OnReceive(wireMsg wire.Message, peer better_p2p.PeerI) {
	cmd := wireMsg.Command()
	switch cmd {
	case wire.CmdInv:
		msg, ok := wireMsg.(*wire.MsgInv)
		if !ok {
			return
		}

		for _, iv := range msg.InvList {
			if iv.Type == wire.InvTypeBlock {
				req := BlockRequest{
					Hash: &iv.Hash,
					Peer: peer,
				}

				h.requestCh <- req
			}
		}

	case wire.CmdBlock:
		msg, ok := wireMsg.(*BlockMessage)
		if !ok {
			return
		}

		h.processCh <- msg

	default:
		// ignore other messages
	}
}

func (h *PeerMsgHandler) OnSend(_ wire.Message, _ better_p2p.PeerI) {
	// ignore
}
