package p2p

import "github.com/TAAL-GmbH/arc/metamorph/store"

type PeerManagerMock struct {
	store     store.Store
	Peers     map[string]PeerI
	messageCh chan *PMMessage
	Announced [][]byte
}

func NewPeerManagerMock(s store.Store, messageCh chan *PMMessage) *PeerManagerMock {
	return &PeerManagerMock{
		store:     s,
		Peers:     make(map[string]PeerI),
		messageCh: messageCh,
	}
}

func (p *PeerManagerMock) AnnounceNewTransaction(txID []byte) {
	p.Announced = append(p.Announced, txID)
}

func (p *PeerManagerMock) AddPeer(peerURL string, peerStore PeerStoreI) error {
	peer, err := NewPeerMock(peerURL, peerStore, p.messageCh)
	if err != nil {
		return err
	}

	return p.addPeer(peer)
}

func (p *PeerManagerMock) RemovePeer(peerURL string) error {
	delete(p.Peers, peerURL)
	return nil
}

func (p *PeerManagerMock) GetPeers() []PeerI {
	peers := make([]PeerI, 0, len(p.Peers))
	for _, peer := range p.Peers {
		peers = append(peers, peer)
	}
	return peers
}

func (p *PeerManagerMock) addPeer(peer PeerI) error {
	p.Peers[peer.String()] = peer
	return nil
}
