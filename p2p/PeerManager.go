package p2p

import (
	"fmt"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/TAAL-GmbH/arc/store"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"

	pb "github.com/TAAL-GmbH/arc/metamorph_api"

	batcher "github.com/ordishs/go-utils/batcher"
	"github.com/ordishs/gocore"
)

type PeerManager struct {
	mu         sync.RWMutex
	peers      map[string]*Peer
	invBatcher *batcher.Batcher[[]byte]
}

type PMMessage struct {
	Start  time.Time
	Txid   string
	Status pb.Status
	Err    error
}

func NewPeerManager(s store.Store, messageCh chan *PMMessage) *PeerManager {

	pm := &PeerManager{
		peers: make(map[string]*Peer),
	}

	pm.invBatcher = batcher.New(500, 500*time.Millisecond, pm.sendInvBatch, true)

	peerCount, _ := gocore.Config().GetInt("peerCount", 0)
	if peerCount == 0 {
		logger.Fatalf("peerCount must be set")
	}

	for i := 1; i <= peerCount; i++ {
		host, _ := gocore.Config().Get(fmt.Sprintf("peer_%d_host", i), "localhost")
		port, _ := gocore.Config().GetInt(fmt.Sprintf("peer_%d_p2pPort", i), 18333)

		peer, err := NewPeer(fmt.Sprintf("%s:%d", host, port), s, messageCh)
		if err != nil {
			logger.Fatalf("Error creating peer: %v", err)
		}

		pm.AddPeer(peer)
	}

	return pm
}

func (pm *PeerManager) AddPeer(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.peers[peer.address] = peer
}

func (pm *PeerManager) AnnounceNewTransaction(txid []byte) {
	pm.invBatcher.Put(&txid)
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

	for _, peer := range pm.peers {
		utils.SafeSend[wire.Message](peer.writeChan, invMsg)
	}

	// if len(batch) <= 10 {
	logger.Infof("Sent INV (%d items) to %d peers", len(batch), len(pm.peers))
	for _, txid := range batch {
		logger.Infof("        %x", bt.ReverseBytes(*txid))
	}
	// } else {
	// 	logger.Infof("Sent INV (%d items) to %d peers", len(batch), len(pm.peers))
	// }
}
