package p2p

import (
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

type NetworkMessanger struct {
	pm *PeerManager
}

func NewHerald(m *PeerManager) *NetworkMessanger {
	return &NetworkMessanger{pm: m}
}

func (h *NetworkMessanger) CountConnectedPeers() uint {
	return h.pm.CountConnectedPeers()
}

// AnnounceTransactions will send an INV messages to the provided peers or to selected peers if peers is nil.
// It will return the peers that the transaction was actually announced to.
func (h *NetworkMessanger) AnnounceTransactions(txHashes []*chainhash.Hash, peers []PeerI) []PeerI {
	// create INV messages
	const batchSize = 256

	var messages []*wire.MsgInv

	invMsg := wire.NewMsgInvSizeHint(uint(min(batchSize, len(txHashes))))
	messages = append(messages, invMsg)

	for i, hash := range txHashes {
		iv := wire.NewInvVect(wire.InvTypeTx, hash)
		_ = invMsg.AddInvVect(iv)

		// create new message if we met batch size
		if (i+1)%batchSize == 0 && (i+1) < len(txHashes) {
			invMsg = wire.NewMsgInvSizeHint(batchSize)
			messages = append(messages, invMsg)
		}
	}

	// choose peers to announce transactions
	if len(peers) == 0 {
		peers = h.pm.GetPeersForAnnouncement()
	}

	// send messages
	for _, peer := range peers {
		for _, msg := range messages {
			peer.WriteMsg(msg)
		}
	}

	return peers
}

// AnnounceTransaction will send an INV message to the provided peers or to selected peers if peers is nil.
// It will return the peers that the transaction was actually announced to.
func (h *NetworkMessanger) AnnounceTransaction(txHash *chainhash.Hash, peers []PeerI) []PeerI {
	return h.AnnounceTransactions([]*chainhash.Hash{txHash}, peers)
}

// RequestTransactions will send an GETDATA messages to the first connected peer.
// It will return the peer that the message was actually sent or nil if now peers are connected.
func (h *NetworkMessanger) RequestTransactions(txHashes []*chainhash.Hash) PeerI {
	// create GETDATA messages
	const batchSize = 256

	var messages []*wire.MsgGetData

	getMsg := wire.NewMsgGetDataSizeHint(uint(min(batchSize, len(txHashes))))
	messages = append(messages, getMsg)

	for i, hash := range txHashes {
		iv := wire.NewInvVect(wire.InvTypeTx, hash)
		_ = getMsg.AddInvVect(iv)

		// create new message if we met batch size
		if (i+1)%batchSize == 0 && (i+1) < len(txHashes) {
			getMsg = wire.NewMsgGetDataSizeHint(batchSize)
			messages = append(messages, getMsg)
		}
	}

	// get first connected peer
	var peer PeerI
	for _, p := range h.pm.GetPeersForAnnouncement() {
		if p.Connected() {
			peer = p
			break
		}
	}

	if peer == nil {
		return nil
	}

	// send messages
	for _, msg := range messages {
		peer.WriteMsg(msg)
	}

	return peer
}

// RequestTransaction will send an GETDATA message to the first connected peer.
// It will return the peer that the message was actually sent or nil if now peers are connected.
func (h *NetworkMessanger) RequestTransaction(txHash *chainhash.Hash) PeerI {
	return h.RequestTransactions([]*chainhash.Hash{txHash})
}

// AnnounceBlock will send an INV message to the provided peers or to selected peers if peers is nil.
// It will return the peers that the block was actually announced to.
func (h *NetworkMessanger) AnnounceBlock(blockHash *chainhash.Hash, peers []PeerI) []PeerI {
	// create INV message
	invMsg := wire.NewMsgInvSizeHint(1)
	iv := wire.NewInvVect(wire.InvTypeBlock, blockHash)
	_ = invMsg.AddInvVect(iv)

	// choose peers to announce transactions
	if len(peers) == 0 {
		peers = h.pm.GetPeersForAnnouncement()
	}

	// send message
	for _, peer := range peers {
		peer.WriteMsg(invMsg)
	}

	return peers
}

// RequestBlock will send an GETDATA message to the first connected peer.
// It will return the peer that the message was actually sent or nil if now peers are connected.
func (h *NetworkMessanger) RequestBlock(blockHash *chainhash.Hash) PeerI {
	// create GETDATA message
	getMsg := wire.NewMsgGetDataSizeHint(1)
	iv := wire.NewInvVect(wire.InvTypeBlock, blockHash)
	_ = getMsg.AddInvVect(iv)

	// get first connected peer
	var peer PeerI
	for _, p := range h.pm.GetPeersForAnnouncement() {
		if p.Connected() {
			peer = p
			break
		}
	}

	if peer != nil {
		peer.WriteMsg(getMsg)
	}

	return peer
}
