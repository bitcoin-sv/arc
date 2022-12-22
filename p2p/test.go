package p2p

import "github.com/TAAL-GmbH/arc/metamorph/store"

type PeerManagerMock struct {
	store     store.Store
	peers     map[string]*Peer
	messageCh chan *PMMessage
	Announced [][]byte
}

func NewPeerManagerMock(s store.Store, messageCh chan *PMMessage) *PeerManagerMock {
	return &PeerManagerMock{
		store:     s,
		peers:     make(map[string]*Peer),
		messageCh: messageCh,
	}
}

func (p *PeerManagerMock) AnnounceNewTransaction(txID []byte) {
	p.Announced = append(p.Announced, txID)
}
