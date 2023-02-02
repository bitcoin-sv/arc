package p2p

type TestLogger struct{}

func (l TestLogger) LogLevel() int {
	return 0
}
func (l TestLogger) Debugf(format string, args ...interface{}) {}
func (l TestLogger) Infof(format string, args ...interface{})  {}
func (l TestLogger) Warnf(format string, args ...interface{})  {}
func (l TestLogger) Errorf(format string, args ...interface{}) {}
func (l TestLogger) Fatalf(format string, args ...interface{}) {}

type PeerManagerMock struct {
	Peers       map[string]PeerI
	Announced   [][]byte
	GetData     [][]byte
	peerCreator func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error)
}

func NewPeerManagerMock() *PeerManagerMock {
	return &PeerManagerMock{
		Peers: make(map[string]PeerI),
	}
}

func (p *PeerManagerMock) GetTransaction(txID []byte) {
	p.GetData = append(p.GetData, txID)
}

func (p *PeerManagerMock) AnnounceTransaction(txID []byte, peers []PeerI) []PeerI {
	p.Announced = append(p.Announced, txID)
	return nil
}

func (p *PeerManagerMock) PeerCreator(peerCreator func(peerAddress string, peerHandler PeerHandlerI) (PeerI, error)) {
	p.peerCreator = peerCreator
}

func (p *PeerManagerMock) AddPeer(peerURL string, peerHandler PeerHandlerI) error {
	peer, err := NewPeerMock(peerURL, peerHandler)
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
