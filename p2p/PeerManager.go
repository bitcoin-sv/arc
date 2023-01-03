package p2p

import (
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/batcher"
)

type PeerManager struct {
	mu         sync.RWMutex
	peers      map[string]PeerI
	invBatcher *batcher.Batcher[[]byte]
	store      store.Store
	messageCh  chan *PMMessage
}

type PMMessage struct {
	Start  time.Time
	Txid   string
	Status metamorph_api.Status
	Err    error
}

func NewPeerManager(s store.Store, peers []string, messageCh chan *PMMessage, batchDuration ...time.Duration) (PeerManagerI, error) {
	pm := &PeerManager{
		peers:     make(map[string]PeerI),
		store:     s,
		messageCh: messageCh,
	}

	batchDelay := 500 * time.Millisecond
	if len(batchDuration) > 0 {
		batchDelay = batchDuration[0]
	}
	pm.invBatcher = batcher.New(500, batchDelay, pm.sendInvBatch, true)

	for _, peerURL := range peers {
		if err := pm.AddPeer(peerURL); err != nil {
			return nil, err
		}
	}

	return pm, nil
}

func (pm *PeerManager) AddPeer(peerAddress string) error {
	// check peer is not already in the list
	if _, ok := pm.peers[peerAddress]; ok {
		return nil
	}

	peer, err := NewPeer(peerAddress, pm.store, pm.messageCh)
	if err != nil {
		return err
	}

	return pm.addPeer(peer)
}

func (pm *PeerManager) RemovePeer(peerURL string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.peers, peerURL)

	return nil
}

func (pm *PeerManager) GetPeers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]string, 0, len(pm.peers))
	for peerURL := range pm.peers {
		peers = append(peers, peerURL)
	}

	return peers
}

func (pm *PeerManager) addPeer(peer PeerI) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.peers[peer.String()] = peer

	return nil
}

func (pm *PeerManager) AnnounceNewTransaction(txID []byte) {
	pm.invBatcher.Put(&txID)
}

func (pm *PeerManager) sendInvBatch(batch []*[]byte) {
	invMsg := wire.NewMsgInvSizeHint(uint(len(batch)))

	for _, txid := range batch {
		hash, err := chainhash.NewHash(*txid)
		if err != nil {
			logger.Infof("ERROR announcing new tx [%x]: %v", txid, err)
			continue
		}

		iv := wire.NewInvVect(wire.InvTypeTx, hash)
		_ = invMsg.AddInvVect(iv)
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// send to a subset of peers to be able to listen on the rest
	sendToPeers := make([]PeerI, 0, len(pm.peers))
	for _, peer := range pm.peers {
		if len(pm.peers) > 1 && len(sendToPeers) > len(pm.peers)/2 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	for _, peer := range sendToPeers {
		utils.SafeSend[wire.Message](peer.WriteChan(), invMsg)
	}

	// if len(batch) <= 10 {
	logger.Infof("Sent INV (%d items) to %d peers", len(batch), len(sendToPeers))
	for _, txid := range batch {
		logger.Infof("        %x", bt.ReverseBytes(*txid))
	}
	// } else {
	// 	logger.Infof("Sent INV (%d items) to %d peers", len(batch), len(pm.peers))
	// }
}
