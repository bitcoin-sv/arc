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
	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

var ErrTxRejectedByPeer = errors.New("transaction rejected by peer")

type TxStatusMessage struct {
	Start        time.Time
	Hash         *chainhash.Hash
	Status       metamorph_api.Status
	Peer         string
	Err          error
	CompetingTxs []string
}

var _ p2p.MessageHandlerI = (*MsgHandler)(nil)

type MsgHandler struct {
	logger    *slog.Logger
	store     store.MetamorphStore
	messageCh chan<- *TxStatusMessage
}

func NewMsgHandler(l *slog.Logger, s store.MetamorphStore, messageCh chan<- *TxStatusMessage) *MsgHandler {
	ph := &MsgHandler{
		logger:    l,
		store:     s,
		messageCh: messageCh,
	}

	return ph
}

// OnReceive handles incoming messages depending on command type
func (h *MsgHandler) OnReceive(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdInv:
		h.handleReceivedInv(msg, peer)

	case wire.CmdTx:
		h.handleReceivedTx(msg, peer)

	case wire.CmdReject:
		h.handleReceivedReject(msg, peer)

	case wire.CmdGetData:
		h.handleReceivedGetData(msg, peer)

	default:
		// ignore other messages
	}
}

// OnSend handles outgoing messages depending on command type
func (h *MsgHandler) OnSend(msg wire.Message, peer p2p.PeerI) {
	cmd := msg.Command()
	switch cmd {
	case wire.CmdTx:
		txMsg, ok := msg.(*wire.MsgTx)
		if !ok {
			return
		}

		hash := txMsg.TxHash()
		h.messageCh <- &TxStatusMessage{
			Hash:   &hash,
			Status: metamorph_api.Status_SENT_TO_NETWORK,
			Peer:   peer.String(),
		}
	default:
		// ignore other messages
	}
}

func (h *MsgHandler) handleReceivedInv(wireMsg wire.Message, peer p2p.PeerI) {
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

func (h *MsgHandler) handleReceivedTx(wireMsg wire.Message, peer p2p.PeerI) {
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

func (h *MsgHandler) handleReceivedReject(wireMsg wire.Message, peer p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgReject)
	if !ok {
		return
	}

	h.messageCh <- &TxStatusMessage{
		Hash:   &msg.Hash,
		Status: metamorph_api.Status_REJECTED,
		Peer:   peer.String(),
		Err:    errors.Join(ErrTxRejectedByPeer, fmt.Errorf("peer: %s reason: %s", peer.String(), msg.Reason)),
	}
}

func (h *MsgHandler) handleReceivedGetData(wireMsg wire.Message, peer p2p.PeerI) {
	msg, ok := wireMsg.(*wire.MsgGetData)
	if !ok {
		return
	}

	// do not block main goroutine
	go func(msg *wire.MsgGetData, peer p2p.PeerI) {
		// handle tx INV
		txRequests := make([][]byte, 0, len(msg.InvList))

		for _, iv := range msg.InvList {
			if iv.Type == wire.InvTypeTx {
				txRequests = append(txRequests, iv.Hash[:])
			}
			// ignore other INV types
		}

		rtx, err := h.store.GetRawTxs(context.Background(), txRequests)
		if err != nil {
			h.logger.Error("Unable to fetch txs from store", slog.Int("count", len(txRequests)), slog.String("err", err.Error()))
			return
		}

		for _, txBytes := range rtx {
			tx, err := bsvutil.NewTxFromBytes(txBytes)
			if err != nil {
				h.logger.Error("failed to parse tx", slog.String("rawHex", hex.EncodeToString(txBytes)), slog.String("err", err.Error()))
				continue
			}

			h.messageCh <- &TxStatusMessage{
				Hash:   tx.Hash(),
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
				Peer:   peer.String(),
			}

			wm := tx.MsgTx()
			peer.WriteMsg(wm)
		}
	}(msg, peer)
}
