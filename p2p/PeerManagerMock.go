package p2p

type TestLogger struct{}

func (l TestLogger) Infof(format string, args ...interface{})  {}
func (l TestLogger) Warnf(format string, args ...interface{})  {}
func (l TestLogger) Errorf(format string, args ...interface{}) {}
func (l TestLogger) Fatalf(format string, args ...interface{}) {}

type PeerManagerMock struct {
	Peers       map[string]PeerI
	messageCh   chan *PMMessage
	Announced   [][]byte
	peerCreator func(peerAddress string, peerStore PeerStoreI) (PeerI, error)
}

func NewPeerManagerMock(messageCh chan *PMMessage) *PeerManagerMock {
	return &PeerManagerMock{
		Peers:     make(map[string]PeerI),
		messageCh: messageCh,
	}
}

func (p *PeerManagerMock) AnnounceNewTransaction(txID []byte) {
	p.Announced = append(p.Announced, txID)
}

func (p *PeerManagerMock) PeerCreator(peerCreator func(peerAddress string, peerStore PeerStoreI) (PeerI, error)) {
	p.peerCreator = peerCreator
}
func (p *PeerManagerMock) AddPeer(peerURL string, peerStore PeerStoreI) error {
	peer, err := NewPeerMock(peerURL, peerStore)
	if err != nil {
		return err
	}
	peer.AddParentMessageChannel(p.messageCh)

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
