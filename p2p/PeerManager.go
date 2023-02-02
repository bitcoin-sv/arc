package p2p

import (
	"sort"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/batcher"
	"github.com/ordishs/gocore"
)

type PeerManager struct {
	mu          sync.RWMutex
	peers       map[string]PeerI
	network     wire.BitcoinNet
	dataBatcher *batcher.Batcher[[]byte]
	logger      utils.Logger

	// this is needed to be able to mock the peer creation in the peer manager
	peerCreator func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error)
}

var ebs int

func init() {
	ebs, _ = gocore.Config().GetInt("excessive_block_size", 4000000000)
	wire.SetLimits(uint64(ebs))
}

// NewPeerManager creates a new PeerManager
// messageCh is a channel that will be used to send messages from peers to the parent process
// this is used to pass INV messages from the bitcoin network peers to the parent process
// at the moment this is only used for Inv tx message for "seen", "sent" and "rejected" transactions
func NewPeerManager(logger utils.Logger, network wire.BitcoinNet, batchDuration ...time.Duration) PeerManagerI {
	logger.Infof("Excessive block size set to %d", ebs)

	pm := &PeerManager{
		peers:   make(map[string]PeerI),
		network: network,
		logger:  logger,
		peerCreator: func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error) {
			return NewPeer(logger, peerAddress, peerHandler, network)
		},
	}

	batchDelayMillis, _ := gocore.Config().GetInt("peerManager_batchDelay_millis", 10)

	batchDelay := time.Duration(batchDelayMillis) * time.Millisecond
	if len(batchDuration) > 0 {
		batchDelay = batchDuration[0]
	}
	pm.dataBatcher = batcher.New(500, batchDelay, pm.sendDataBatch, true)

	return pm
}

func (pm *PeerManager) PeerCreator(peerCreator func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error)) {
	pm.peerCreator = peerCreator
}

func (pm *PeerManager) AddPeer(peerAddress string, peerHandler PeerHandlerI) error {
	// check peer is not already in the list
	if _, ok := pm.peers[peerAddress]; ok {
		return nil
	}

	peer, err := pm.peerCreator(peerAddress, peerHandler)
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

func (pm *PeerManager) GetTransaction(txID []byte) {
	pm.dataBatcher.Put(&txID)
}

// AnnounceTransaction will send an INV message to the provided peers or to selected peers if peers is nil
// it will return the peers that the transaction was actually announced to
func (pm *PeerManager) AnnounceTransaction(txID []byte, peers []PeerI) []PeerI {
	if len(peers) == 0 {
		peers = pm.GetAnnouncedPeers()
	}

	for _, peer := range peers {
		peer.AnnounceTransaction(txID)
	}

	return peers
}

func (pm *PeerManager) GetAnnouncedPeers() []PeerI {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Get a list of peers that are connected
	connectedPeers := make([]PeerI, 0, len(pm.peers))
	for _, peer := range pm.peers {
		if peer.Connected() {
			connectedPeers = append(connectedPeers, peer)
		}
	}

	// sort peers by address
	sort.SliceStable(connectedPeers, func(i, j int) bool {
		return connectedPeers[i].String() < connectedPeers[j].String()
	})

	// send to a subset of peers to be able to listen on the rest
	sendToPeers := make([]PeerI, 0, len(connectedPeers))
	for _, peer := range connectedPeers {

		if len(connectedPeers) > 1 && len(sendToPeers) >= (len(connectedPeers)+1)/2 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	return sendToPeers
}

func (pm *PeerManager) sendDataBatch(batch []*[]byte) {
	dataMsg := wire.NewMsgGetData()

	for _, txid := range batch {
		hash, err := chainhash.NewHash(*txid)
		if err != nil {
			pm.logger.Infof("ERROR getting tx [%x]: %v", txid, err)
			continue
		}

		iv := wire.NewInvVect(wire.InvTypeTx, hash)
		_ = dataMsg.AddInvVect(iv)
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// send to the first found peer that is connected
	sendToPeers := make([]PeerI, 0, len(pm.peers))
	for _, peer := range pm.peers {
		if peer.Connected() && len(sendToPeers) >= 1 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	for _, peer := range sendToPeers {
		if err := peer.WriteMsg(dataMsg); err != nil {
			pm.logger.Infof("ERROR sending data message to peer [%s]: %v", peer.String(), err)
		} else {
			pm.logger.Infof("Sent GETDATA (%d items) to peer: %s", len(batch), peer.String())
		}
	}
}
