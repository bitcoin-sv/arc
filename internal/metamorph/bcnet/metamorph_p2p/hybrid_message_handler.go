package metamorph_p2p

import (
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/libsv/go-p2p/wire"
)

var _ p2p.MessageHandlerI = (*HybridMsgHandler)(nil)

type HybridMsgHandler struct {
	logger    *slog.Logger
	messageCh chan<- *TxStatusMessage
}

func NewHybridMsgHandler(l *slog.Logger, messageCh chan<- *TxStatusMessage) *HybridMsgHandler {
	return &HybridMsgHandler{
		logger: l.With(
			slog.String("module", "peer-msg-handler"),
			slog.String("mode", "hybrid"),
		),

		messageCh: messageCh,
	}
}

// OnReceive handles incoming messages depending on command type
func (h *HybridMsgHandler) OnReceive(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdInv:
		h.handleReceivedInv(msg, peer)

	case wire.CmdTx:
		h.handleReceivedTx(msg, peer)

	default:
		// ignore other
	}
}

// OnSend handles outgoing messages depending on command type
func (h *HybridMsgHandler) OnSend(_ wire.Message, _ p2p.PeerI) {
	// ignore
}

func (h *HybridMsgHandler) handleReceivedInv(wireMsg wire.Message, peer p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgInv)
	if !ok {
		return
	}

	go func() {
		for _, iv := range msg.InvList {
			if iv.Type == wire.InvTypeTx {
				select {
				case h.messageCh <- &TxStatusMessage{
					Hash:   &iv.Hash,
					Status: metamorph_api.Status_SEEN_ON_NETWORK,
					Peer:   peer.String(),
				}:
				default: // Ensure that writing to channel is non-blocking -- probably we should give up on this
				}
			}
			// ignore INV with block or error
		}
	}()
}

func (h *HybridMsgHandler) handleReceivedTx(wireMsg wire.Message, peer p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgTx)
	if !ok {
		return
	}

	hash := msg.TxHash()
	h.messageCh <- &TxStatusMessage{
		Hash:   &hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}
}
