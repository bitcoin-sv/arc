package p2p

import (
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

const (
	batchInterval   time.Duration = 200 * time.Millisecond
	batchSize                     = 512
	batchBufferSize               = 2048
)

type NetworkMessenger struct {
	logger          *slog.Logger
	manager         *PeerManager
	requestBatcher  *batchProcessor
	announceBatcher *batchProcessor
}

func NewNetworkMessenger(l *slog.Logger, pm *PeerManager) *NetworkMessenger {
	m := &NetworkMessenger{
		logger:  l.With(slog.String("module", "network-messenger")),
		manager: pm,
	}

	m.requestBatcher = newBatchProcessor(batchSize, batchInterval, m.sendGetDataMsg, batchBufferSize)
	m.announceBatcher = newBatchProcessor(batchSize, batchInterval, m.sendInvMsg, batchBufferSize)

	return m
}

func (m *NetworkMessenger) CountConnectedPeers() uint {
	return m.manager.CountConnectedPeers()
}

func (m *NetworkMessenger) GetPeers() []PeerI {
	return m.manager.GetPeers()
}

func (m *NetworkMessenger) Shutdown() {
	m.announceBatcher.Shutdown()
	m.announceBatcher = nil
	m.requestBatcher.Shutdown()
	m.requestBatcher = nil
}

// AnnounceTransactions will send an INV messages to the provided peers or to selected peers if peers is nil.
// It will return the peers that the transaction was actually announced to.
func (m *NetworkMessenger) AnnounceTransactions(txHashes []*chainhash.Hash, peers []PeerI) []PeerI {
	// create INV messages
	var messages []*wire.MsgInv

	mi, err := api.SafeIntToUint(min(batchSize, len(txHashes)))
	if err != nil {
		m.logger.Error("Failed to convert batch size to uint32", slog.String("error", err.Error()))
		return nil
	}
	invMsg := wire.NewMsgInvSizeHint(mi)
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
		peers = m.manager.GetPeersForAnnouncement()
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
func (m *NetworkMessenger) AnnounceTransaction(txHash *chainhash.Hash, peers []PeerI) []PeerI {
	return m.AnnounceTransactions([]*chainhash.Hash{txHash}, peers)
}

// RequestTransactions will send an GETDATA messages to the first connected peer.
// It will return the peer that the message was actually sent or nil if now peers are connected.
func (m *NetworkMessenger) RequestTransactions(txHashes []*chainhash.Hash) PeerI {
	// create GETDATA messages
	var messages []*wire.MsgGetData

	mi, err := api.SafeIntToUint(min(batchSize, len(txHashes)))
	if err != nil {
		m.logger.Error("Failed to convert batch size to uint32", slog.String("error", err.Error()))
		return nil
	}
	getMsg := wire.NewMsgGetDataSizeHint(mi)
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
	for _, p := range m.manager.GetPeersForAnnouncement() {
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
func (m *NetworkMessenger) RequestTransaction(txHash *chainhash.Hash) PeerI {
	return m.RequestTransactions([]*chainhash.Hash{txHash})
}

// AnnounceBlock will send an INV message to the provided peers or to selected peers if peers is nil.
// It will return the peers that the block was actually announced to.
func (m *NetworkMessenger) AnnounceBlock(blockHash *chainhash.Hash, peers []PeerI) []PeerI {
	// create INV message
	invMsg := wire.NewMsgInvSizeHint(1)
	iv := wire.NewInvVect(wire.InvTypeBlock, blockHash)
	_ = invMsg.AddInvVect(iv)

	// choose peers to announce transactions
	if len(peers) == 0 {
		peers = m.manager.GetPeersForAnnouncement()
	}

	// send message
	for _, peer := range peers {
		peer.WriteMsg(invMsg)
	}

	return peers
}

// RequestBlock will send an GETDATA message to the first connected peer.
// It will return the peer that the message was actually sent or nil if now peers are connected.
func (m *NetworkMessenger) RequestBlock(blockHash *chainhash.Hash) PeerI {
	// create GETDATA message
	getMsg := wire.NewMsgGetDataSizeHint(1)
	iv := wire.NewInvVect(wire.InvTypeBlock, blockHash)
	_ = getMsg.AddInvVect(iv)

	// get first connected peer
	var peer PeerI
	for _, p := range m.manager.GetPeersForAnnouncement() {
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

func (m *NetworkMessenger) AnnounceWithAutoBatch(hash *chainhash.Hash, invType wire.InvType) {
	m.announceBatcher.Put(wire.NewInvVect(invType, hash))
}

func (m *NetworkMessenger) RequestWithAutoBatch(hash *chainhash.Hash, invType wire.InvType) {
	m.requestBatcher.Put(wire.NewInvVect(invType, hash))
}

func (m *NetworkMessenger) sendInvMsg(inv []*wire.InvVect) {
	if len(inv) == 0 {
		return
	}

	invMsg := wire.NewMsgInvSizeHint(uint(len(inv)))
	for _, v := range inv {
		_ = invMsg.AddInvVect(v)
	}

	// choose peers to announce transactions
	peers := m.manager.GetPeersForAnnouncement()
	if len(peers) == 0 {
		m.logger.Error("Cannot send INV - 0 connected peers")
		return
	}

	// send message
	for _, peer := range peers {
		peer.WriteMsg(invMsg)
	}
}

func (m *NetworkMessenger) sendGetDataMsg(inv []*wire.InvVect) {
	if len(inv) == 0 {
		return
	}

	getMsg := wire.NewMsgGetDataSizeHint(uint(len(inv)))
	for _, v := range inv {
		_ = getMsg.AddInvVect(v)
	}

	// choose peers to announce transactions
	peers := m.manager.GetPeersForAnnouncement()
	if len(peers) == 0 {
		m.logger.Error("Cannot send GET DATA - 0 connected peers")
		return
	}

	// send message
	for _, peer := range peers {
		peer.WriteMsg(getMsg)
	}
}
