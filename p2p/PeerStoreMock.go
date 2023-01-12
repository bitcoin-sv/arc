package p2p

import "github.com/TAAL-GmbH/arc/p2p/wire"

type MockPeerHandler struct {
}

func NewMockPeerHandler() PeerHandlerI {
	return &MockPeerHandler{}
}

func (m *MockPeerHandler) GetTransactionBytes(_ *wire.InvVect) ([]byte, error) {
	return nil, nil
}

func (m *MockPeerHandler) HandleTransactionSent(_ *wire.MsgTx, peer PeerI) error {
	return nil
}

func (m *MockPeerHandler) HandleTransactionAnnouncement(msg *wire.InvVect, peer PeerI) error {
	return nil
}

func (m *MockPeerHandler) HandleTransactionRejection(rejMsg *wire.MsgReject, peer PeerI) error {
	return nil
}

func (m *MockPeerHandler) HandleBlockAnnouncement(msg *wire.InvVect, peer PeerI) error {
	return nil
}

func (m *MockPeerHandler) HandleBlock(msg *wire.MsgBlock, peer PeerI) error {
	return nil
}
