package blocktx_p2p

import (
	"log/slog"

	"github.com/libsv/go-p2p/wire"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

var _ p2p.MessageHandlerI = (*HybridMsgHandler)(nil)

type HybridMsgHandler struct {
	logger            *slog.Logger
	blockProcessingCh chan<- *bcnet.BlockMessagePeer
}

func NewHybridMsgHandler(l *slog.Logger, blockProcessCh chan<- *bcnet.BlockMessagePeer) *HybridMsgHandler {
	return &HybridMsgHandler{
		logger: l.With(
			slog.String("module", "peer-msg-handler"),
			slog.String("mode", "hybrid"),
		),
		blockProcessingCh: blockProcessCh,
	}
}

// OnReceive handles incoming messages depending on command type
func (h *HybridMsgHandler) OnReceive(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()

	switch cmd {
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
func (h *HybridMsgHandler) OnSend(_ wire.Message, _ p2p.PeerI) {
	// ignore
}
