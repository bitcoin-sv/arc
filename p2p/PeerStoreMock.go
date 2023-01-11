package p2p

type MockPeerStore struct {
}

func NewMockPeerStore() *MockPeerStore {
	return &MockPeerStore{}
}

func (m *MockPeerStore) GetTransactionBytes(txID []byte) ([]byte, error) {
	return nil, nil
}

func (m *MockPeerStore) HandleBlockAnnouncement(hash []byte, peer PeerI) error {
	return nil
}

func (m *MockPeerStore) InsertBlock(blockHash []byte, merkleRoot []byte, prevhash []byte, height uint64, peer PeerI) (uint64, error) {
	return 0, nil
}

func (m *MockPeerStore) MarkTransactionsAsMined(blockId uint64, txHashes [][]byte) error {
	return nil
}

func (m *MockPeerStore) MarkBlockAsProcessed(block *Block) error {
	return nil
}
