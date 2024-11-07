package metamorph_p2p

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"

	"github.com/libsv/go-p2p/better_p2p"
)

var ErrTxRejectedByPeer = errors.New("transaction rejected by peer")

var _ better_p2p.MessageHandlerI = (*PeerMsgHandler)(nil)

type PeerMsgHandler struct {
	l         *slog.Logger
	s         store.MetamorphStore
	messageCh chan<- *PeerTxMessage
}

type PeerTxMessage struct {
	Start        time.Time
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Peer         string
	Err          error
	CompetingTxs []string
}

func NewPeerMsgHandler(l *slog.Logger, s store.MetamorphStore, messageCh chan<- *PeerTxMessage) *PeerMsgHandler {
	ph := &PeerMsgHandler{
		l:         l,
		s:         s,
		messageCh: messageCh,
	}

	return ph
}

func (h *PeerMsgHandler) OnReceive(wireMsg wire.Message, peer better_p2p.PeerI) {
	cmd := wireMsg.Command()
	switch cmd {
	case wire.CmdInv:
		h.handleInv(wireMsg, peer)

	case wire.CmdTx:
		h.handleTx(wireMsg, peer)

	case wire.CmdReject:
		h.handleReject(wireMsg, peer)

	case wire.CmdGetData:
		h.handleGetData(wireMsg, peer)

	default:
		// ignore other messages
	}
}

func (h *PeerMsgHandler) OnSend(wireMsg wire.Message, peer better_p2p.PeerI) {
	cmd := wireMsg.Command()
	switch cmd {
	case wire.CmdTx:
		msg, ok := wireMsg.(*wire.MsgTx)
		if !ok {
			return
		}

		hash := msg.TxHash()
		h.messageCh <- &PeerTxMessage{
			Hash:   &hash,
			Status: metamorph_api.Status_SENT_TO_NETWORK,
			Peer:   peer.String(),
		}
	default:
		// ignore other messages
	}
}

func (h *PeerMsgHandler) handleInv(wireMsg wire.Message, peer better_p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgInv)
	if !ok {
		return
	}

	for _, iv := range msg.InvList {
		if iv.Type == wire.InvTypeTx {
			select {
			case h.messageCh <- &PeerTxMessage{
				Hash:   &iv.Hash,
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
				Peer:   peer.String(),
			}:
			default: // Ensure that writing to channel is non-blocking
			}
		}
	}
}

func (h *PeerMsgHandler) handleTx(wireMsg wire.Message, peer better_p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgTx)
	if !ok {
		return
	}

	hash := msg.TxHash()
	h.messageCh <- &PeerTxMessage{
		Hash:   &hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}
}

func (h *PeerMsgHandler) handleReject(wireMsg wire.Message, peer better_p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgReject)
	if !ok {
		return
	}

	h.messageCh <- &PeerTxMessage{
		Hash:   &msg.Hash,
		Status: metamorph_api.Status_REJECTED,
		Peer:   peer.String(),
		Err:    errors.Join(ErrTxRejectedByPeer, fmt.Errorf("peer: %s reason: %s", peer.String(), msg.Reason)),
	}
}

func (h *PeerMsgHandler) handleGetData(wireMsg wire.Message, peer better_p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgGetData)
	if !ok {
		return
	}

	// handle tx INV
	txRequests := make([][]byte, 0)

	for _, iv := range msg.InvList {
		if iv.Type == wire.InvTypeTx {
			txRequests = append(txRequests, iv.Hash[:])

			h.messageCh <- &PeerTxMessage{
				Hash:   &iv.Hash,
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
				Peer:   peer.String(),
			}
		}
	}

	rtx, err := h.s.GetRawTxs(context.Background(), txRequests)
	if err != nil {
		h.l.Warn("Unable to fetch txs from store", slog.Int("count", len(txRequests)), slog.String("err", err.Error()))
		return
	}

	for _, txBytes := range rtx {
		tx, err := bsvutil.NewTxFromBytes(txBytes)
		if err != nil {
			h.l.Error("failed to parse tx", slog.String("rawHex", hex.EncodeToString(txBytes)), slog.String("err", err.Error()))
			continue
		}

		wm := tx.MsgTx()
		peer.WriteMsg(wm)
	}
}
