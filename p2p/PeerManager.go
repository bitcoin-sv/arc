package p2p

import (
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/p2p/chaincfg/chainhash"
	"github.com/TAAL-GmbH/arc/p2p/wire"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/batcher"
	"github.com/ordishs/gocore"
)

type PeerManager struct {
	mu          sync.RWMutex
	peers       map[string]PeerI
	network     wire.BitcoinNet
	invBatcher  *batcher.Batcher[[]byte]
	dataBatcher *batcher.Batcher[[]byte]
	logger      utils.Logger

	// this is needed to be able to mock the peer creation in the peer manager
	peerCreator func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error)
}

func init() {
	ebs, _ := gocore.Config().GetInt("excessive_block_size", 4000000000)
	logger := gocore.Log("ebs")
	logger.Infof("Setting excessive block size to %d", ebs)
	wire.SetLimits(uint64(ebs))
}

// NewPeerManager creates a new PeerManager
// messageCh is a channel that will be used to send messages from peers to the parent process
// this is used to pass INV messages from the bitcoin network peers to the parent process
// at the moment this is only used for Inv tx message for "seen", "sent" and "rejected" transactions
func NewPeerManager(logger utils.Logger, network wire.BitcoinNet, batchDuration ...time.Duration) PeerManagerI {
	pm := &PeerManager{
		peers:   make(map[string]PeerI),
		network: network,
		logger:  logger,
		peerCreator: func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error) {
			return NewPeer(logger, peerAddress, peerHandler, network)
		},
	}

	batchDelay := 500 * time.Millisecond
	if len(batchDuration) > 0 {
		batchDelay = batchDuration[0]
	}
	pm.invBatcher = batcher.New(500, batchDelay, pm.sendInvBatch, true)
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

func (pm *PeerManager) AnnounceNewTransaction(txID []byte) {
	pm.invBatcher.Put(&txID)
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
			pm.logger.Infof("ERROR sending data message to peer [%s]: %v", peer, err)
		} else {
			pm.logger.Infof("Sent GETDATA (%d items) to peer: %s", len(batch), sendToPeers[0].String())
		}
	}
}

func (pm *PeerManager) sendInvBatch(batch []*[]byte) {
	invMsg := wire.NewMsgInvSizeHint(uint(len(batch)))

	for _, txid := range batch {
		hash, err := chainhash.NewHash(*txid)
		if err != nil {
			pm.logger.Infof("ERROR announcing new tx [%x]: %v", txid, err)
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

		if peer.Connected() && len(pm.peers) > 1 && len(sendToPeers) >= (len(pm.peers)+1)/2 {
			break
		}
		sendToPeers = append(sendToPeers, peer)
	}

	var peerAddresses []string
	for _, peer := range sendToPeers {
		peerAddresses = append(peerAddresses, peer.String())
		_ = peer.WriteMsg(invMsg)
	}

	pm.logger.Infof("Sent INV (%d items) to %d peers: %s", len(batch), len(sendToPeers), peerAddresses)
	for _, txid := range batch {
		pm.logger.Debugf("        %x", bt.ReverseBytes(*txid))
	}
}
