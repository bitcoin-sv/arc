package metamorph

import (
	"context"
	"fmt"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils"
)

type PeerHandler struct {
	store     store.MetamorphStore
	messageCh chan *PeerTxMessage
	stats     map[string]*tracing.PeerHandlerStats
}

func NewPeerHandler(s store.MetamorphStore, messageCh chan *PeerTxMessage) p2p.PeerHandlerI {
	ph := &PeerHandler{
		store:     s,
		messageCh: messageCh,
		stats:     make(map[string]*tracing.PeerHandlerStats),
	}

	_ = tracing.NewPeerHandlerCollector("metamorph", ph.stats)

	return ph
}

// HandleTransactionSent is called when a transaction is sent to a peer.
func (m *PeerHandler) HandleTransactionSent(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].TransactionSent.Add(1)

	hash := msg.TxHash()
	m.messageCh <- &PeerTxMessage{
		Txid:   utils.HexEncodeAndReverseBytes(hash.CloneBytes()),
		Status: p2p.Status(metamorph_api.Status_SENT_TO_NETWORK),
		Peer:   peer.String(),
	}

	return nil
}

// HandleTransactionAnnouncement is a message sent to the PeerHandler when a transaction INV message is received from a peer.
func (m *PeerHandler) HandleTransactionAnnouncement(msg *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].TransactionAnnouncement.Add(1)

	m.messageCh <- &PeerTxMessage{
		Txid:   utils.HexEncodeAndReverseBytes(msg.Hash.CloneBytes()),
		Status: p2p.Status(metamorph_api.Status_SEEN_ON_NETWORK),
		Peer:   peer.String(),
	}

	return nil
}

// HandleTransactionRejection is called when a transaction is rejected by a peer.
func (m *PeerHandler) HandleTransactionRejection(rejMsg *wire.MsgReject, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].TransactionRejection.Add(1)

	m.messageCh <- &PeerTxMessage{
		Txid:   utils.HexEncodeAndReverseBytes(rejMsg.Hash.CloneBytes()),
		Status: p2p.Status(metamorph_api.Status_REJECTED),
		Err:    fmt.Errorf("transaction rejected by peer %s: %s", peer.String(), rejMsg.Reason),
	}

	return nil
}

// HandleTransactionGet is called when a peer requests a transaction.
func (m *PeerHandler) HandleTransactionGet(msg *wire.InvVect, peer p2p.PeerI) ([]byte, error) {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].TransactionGet.Add(1)

	sd, err := m.store.Get(context.Background(), msg.Hash.CloneBytes())
	if err != nil {
		return nil, err
	}

	return sd.RawTx, nil
}

// HandleTransaction is called when a transaction is received from a peer.
func (m *PeerHandler) HandleTransaction(msg *wire.MsgTx, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].Transaction.Add(1)

	m.messageCh <- &PeerTxMessage{
		Txid:   msg.TxHash().String(),
		Status: p2p.Status(metamorph_api.Status_SEEN_ON_NETWORK),
		Peer:   peer.String(),
	}

	return nil
}

// HandleBlockAnnouncement is called when a block INV message is received from a peer.
func (m *PeerHandler) HandleBlockAnnouncement(_ *wire.InvVect, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].BlockAnnouncement.Add(1)
	return nil
}

// HandleBlock is called when a block is received from a peer.
func (m *PeerHandler) HandleBlock(msg *p2p.BlockMessage, peer p2p.PeerI) error {
	peerStr := peer.String()
	if _, ok := m.stats[peerStr]; !ok {
		m.stats[peerStr] = &tracing.PeerHandlerStats{}
	}
	m.stats[peerStr].Block.Add(1)
	return nil
}
