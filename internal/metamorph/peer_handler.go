package metamorph

import (
	"context"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
)

type PeerHandler struct {
	store     store.MetamorphStore
	messageCh chan *PeerTxMessage

	cancelAll context.CancelFunc
	ctx       context.Context
}

func NewPeerHandler(s store.MetamorphStore, messageCh chan *PeerTxMessage) *PeerHandler {
	ph := &PeerHandler{
		store:     s,
		messageCh: messageCh,
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	ph.cancelAll = cancelAll
	ph.ctx = ctx

	return ph
}

// HandleTransactionSent is called when a transaction is sent to a peer.
func (m *PeerHandler) HandleTransactionSent(msg *wire.MsgTx, peer p2p.PeerI) error {
	hash := msg.TxHash()
	m.messageCh <- &PeerTxMessage{
		Hash:   &hash,
		Status: metamorph_api.Status_SENT_TO_NETWORK,
		Peer:   peer.String(),
	}

	return nil
}

// HandleTransactionAnnouncement is a message sent to the PeerHandler when a transaction INV message is received from a peer.
func (m *PeerHandler) HandleTransactionAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	m.messageCh <- &PeerTxMessage{
		Hash:   &msg.Hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}

	return nil
}

// HandleTransactionRejection is called when a transaction is rejected by a peer.
func (m *PeerHandler) HandleTransactionRejection(rejMsg *wire.MsgReject, peer p2p.PeerI) error {
	m.messageCh <- &PeerTxMessage{
		Hash:   &rejMsg.Hash,
		Status: metamorph_api.Status_REJECTED,
		Peer:   peer.String(),
		Err:    fmt.Errorf("transaction rejected by peer %s: %s", peer.String(), rejMsg.Reason),
	}

	return nil
}

// HandleTransactionsGet is called when a peer requests a transaction.
func (m *PeerHandler) HandleTransactionsGet(msgs []*wire.InvVect, peer p2p.PeerI) ([][]byte, error) {
	hashes := make([][]byte, len(msgs))

	for i, msg := range msgs {
		m.messageCh <- &PeerTxMessage{
			Hash:   &msg.Hash,
			Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
			Peer:   peer.String(),
		}

		hashes[i] = msg.Hash.CloneBytes()
	}

	storeDataSlice, err := m.store.GetBulk(m.ctx, hashes)
	if err != nil {
		return nil, err
	}

	rawTxs := make([][]byte, len(storeDataSlice))
	for i, sd := range storeDataSlice {
		rawTxs[i] = sd.RawTx
	}

	return rawTxs, nil
}

// HandleTransaction is called when a transaction is received from a peer.
func (m *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	hash := msg.TxHash()

	m.messageCh <- &PeerTxMessage{
		Hash:   &hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}

	return nil
}

// HandleBlockAnnouncement is called when a block INV message is received from a peer.
func (m *PeerHandler) HandleBlockAnnouncement(_ *wire.InvVect, _ p2p.PeerI) error {
	return nil
}

// HandleBlock is called when a block is received from a peer.
func (m *PeerHandler) HandleBlock(_ wire.Message, _ p2p.PeerI) error {
	return nil
}

func (m *PeerHandler) Shutdown() {
	if m.cancelAll != nil {
		m.cancelAll()
	}
}
