package metamorph

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils/safemap"
)

type PeerHandler struct {
	store     store.MetamorphStore
	messageCh chan *PeerTxMessage
	stats     *safemap.Safemap[string, *tracing.PeerHandlerStats]
}

func NewPeerHandler(s store.MetamorphStore, messageCh chan *PeerTxMessage) p2p.PeerHandlerI {
	ph := &PeerHandler{
		store:     s,
		messageCh: messageCh,
		stats:     safemap.New[string, *tracing.PeerHandlerStats](),
	}

	_ = tracing.NewPeerHandlerCollector("metamorph", ph.stats)

	return ph
}

// HandleTransactionSent is called when a transaction is sent to a peer.
func (m *PeerHandler) HandleTransactionSent(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.TransactionSent.Add(1)

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
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.TransactionAnnouncement.Add(1)

	m.messageCh <- &PeerTxMessage{
		Hash:   &msg.Hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}

	return nil
}

// HandleTransactionRejection is called when a transaction is rejected by a peer.
func (m *PeerHandler) HandleTransactionRejection(rejMsg *wire.MsgReject, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.TransactionRejection.Add(1)

	m.messageCh <- &PeerTxMessage{
		Hash:   &rejMsg.Hash,
		Status: metamorph_api.Status_REJECTED,
		Err:    fmt.Errorf("transaction rejected by peer %s: %s", peer.String(), rejMsg.Reason),
	}

	return nil
}

// HandleTransactionGet is called when a peer requests a transaction.
func (m *PeerHandler) HandleTransactionGet(msg *wire.InvVect, peer p2p.PeerI) ([]byte, error) {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.TransactionGet.Add(1)

	m.messageCh <- &PeerTxMessage{
		Hash:   &msg.Hash,
		Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
		Peer:   peer.String(),
	}

	sd, err := m.store.Get(context.Background(), msg.Hash.CloneBytes())
	if err != nil {
		return nil, err
	}

	return sd.RawTx, nil
}

// HandleTransaction is called when a transaction is received from a peer.
func (m *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.Transaction.Add(1)

	hash := msg.TxHash()

	m.messageCh <- &PeerTxMessage{
		Hash:   &hash,
		Status: metamorph_api.Status_SEEN_ON_NETWORK,
		Peer:   peer.String(),
	}

	return nil
}

// HandleBlockAnnouncement is called when a block INV message is received from a peer.
func (m *PeerHandler) HandleBlockAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.BlockAnnouncement.Add(1)

	return nil
}

// HandleBlock is called when a block is received from a peer.
func (m *PeerHandler) HandleBlock(msg *p2p.BlockMessage, peer p2p.PeerI) error {
	peerStr := peer.String()

	stat, ok := m.stats.Get(peerStr)
	if !ok {
		stat = &tracing.PeerHandlerStats{}
		m.stats.Set(peerStr, stat)
	}

	stat.Block.Add(1)

	return nil
}
