package p2p

import (
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils/batcher"
)

type PeerManager struct {
	mu         sync.RWMutex
	peers      map[string]PeerI
	invBatcher *batcher.Batcher[[]byte]
	messageCh  chan *PMMessage

	// this is needed to be able to mock the peer creation in the peer manager
	peerCreator func(peerAddress string, peerStore PeerStoreI) (PeerI, error)
}

type PMMessage struct {
	Start  time.Time
	Txid   string
	Status metamorph_api.Status
	Err    error
}

func NewPeerManager(messageCh chan *PMMessage, batchDuration ...time.Duration) PeerManagerI {
	pm := &PeerManager{
		peers:     make(map[string]PeerI),
		messageCh: messageCh,
		peerCreator: func(peerAddress string, peerStore PeerStoreI) (PeerI, error) {
			return NewPeer(peerAddress, peerStore)
		},
	}

	batchDelay := 500 * time.Millisecond
	if len(batchDuration) > 0 {
		batchDelay = batchDuration[0]
	}
	pm.invBatcher = batcher.New(500, batchDelay, pm.sendInvBatch, true)

	return pm
}

func (pm *PeerManager) PeerCreator(peerCreator func(peerAddress string, peerStore PeerStoreI) (PeerI, error)) {
	pm.peerCreator = peerCreator
}

func (pm *PeerManager) AddPeer(peerAddress string, peerStore PeerStoreI) error {
	// check peer is not already in the list
	if _, ok := pm.peers[peerAddress]; ok {
		return nil
	}

	peer, err := pm.peerCreator(peerAddress, peerStore)
	if err != nil {
		return err
	}

	if pm.messageCh != nil {
		peer.AddParentMessageChannel(pm.messageCh)
	}

	return pm.addPeer(peer)
}

func (pm *PeerManager) RemovePeer(peerURL string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.peers, peerURL)

	return nil
}

func (pm *PeerManager) GetPeers() []PeerI {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]PeerI, 0, len(pm.peers))
	for _, peer := range pm.peers {
		peers = append(peers, peer)
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
		if len(pm.peers) > 1 && len(sendToPeers) >= len(pm.peers)/2 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	for _, peer := range sendToPeers {
		peer.WriteMsg(invMsg)
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
