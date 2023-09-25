package blocktx

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils/expiringmap"
	"github.com/ordishs/gocore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mocking wire.peerI as it's third party library and need to mock in here
type MockedPeer struct{}

func (peer *MockedPeer) Connected() bool                            { return true }
func (peer *MockedPeer) WriteMsg(msg wire.Message) error            { return nil }
func (peer *MockedPeer) String() string                             { return "" }
func (peer *MockedPeer) AnnounceTransaction(txHash *chainhash.Hash) {}
func (peer *MockedPeer) RequestTransaction(txHash *chainhash.Hash)  {}
func (peer *MockedPeer) AnnounceBlock(blockHash *chainhash.Hash)    {}
func (peer *MockedPeer) RequestBlock(blockHash *chainhash.Hash)     {}
func (peer *MockedPeer) Network() wire.BitcoinNet                   { return 0 }

func TestExtractHeight(t *testing.T) {
	coinbase, _ := hex.DecodeString("01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff570350cc0b041547b5630cfabe6d6d0000000000000000000000000000000000000000000000000000000000000000010000000000000047ed20542096bd0000000000143362663865373833636662643732306431383436000000000140be4025000000001976a914c9b0abe09b7dd8e9d1e8c1e3502d32ab0d7119e488ac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := bt.NewTxFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := extractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(773200), height, "height should be 773200, got %d", height)
}

func TestExtractHeightForRegtest(t *testing.T) {
	coinbase, _ := hex.DecodeString("02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0502dc070101ffffffff012f500900000000002321032efe256e14fd77eea05d0453374f8920e0a7a4a573bb3937ef3f567f3937129cac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := bt.NewTxFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := extractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(2012), height, "height should be 2012, got %d", height)
}

func TestGetAnnouncedCacheBlockHashes(t *testing.T) {
	peerHandler := PeerHandler{
		announcedCache: expiringmap.New[chainhash.Hash, []p2p.PeerI](5 * time.Minute),
	}

	peer, err := p2p.NewPeerMock("", &peerHandler, wire.MainNet)
	assert.NoError(t, err)

	hash, err := chainhash.NewHashFromStr("00000000000000000e3c9aafb4c823562dd38f15b75849be348131a785154e33")
	assert.NoError(t, err)
	peerHandler.announcedCache.Set(*hash, []p2p.PeerI{peer})

	hash, err = chainhash.NewHashFromStr("00000000000000000cd097bf90c0f8480b930c88f3994503abccf45d579c601c")
	assert.NoError(t, err)
	peerHandler.announcedCache.Set(*hash, []p2p.PeerI{peer})

	hashes := peerHandler.getAnnouncedCacheBlockHashes()

	assert.ElementsMatch(t, hashes, []string{"00000000000000000e3c9aafb4c823562dd38f15b75849be348131a785154e33", "00000000000000000cd097bf90c0f8480b930c88f3994503abccf45d579c601c"})
}

func TestHandleBlock(t *testing.T) {
	// create mocked peer handler
	var blockTxLogger = gocore.Log("btx", gocore.NewLogLevelFromString("INFO"))
	var storeMock = &store.InterfaceMock{}

	// mock specific functions of storage that we are about to call
	storeMock.GetBlockFunc = func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
		return &blocktx_api.Block{}, nil
	}

	storeMock.GetBlockFunc = func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
		return &blocktx_api.Block{}, nil
	}

	storeMock.InsertBlockFunc = func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
		return 0, nil
	}

	// main assert for the test to make sure block with a single transaction doesn't have any merkle paths other than empty ones "0000"
	storeMock.InsertBlockTransactionsFunc = func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error {
		assert.Equal(t, uint64(1), uint64(len(merklePaths)))
		assert.Equal(t, merklePaths[0], "0000")
		return nil
	}

	storeMock.MarkBlockAsDoneFunc = func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
		return nil
	}

	// build peer manager
	peerHandler := NewPeerHandler(blockTxLogger, storeMock, make(chan *blocktx_api.Block, 1))

	// define HandleBlock function parameters (BlockMessage and p2p.PeerI)
	txHash, _ := chainhash.NewHashFromStr("3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f")
	prevBlockHash, _ := chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
	merkleRootHash, _ := chainhash.NewHashFromStr("3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f")
	blockMessage := &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash,
			MerkleRoot: *merkleRootHash,
			Timestamp:  time.Time{},
			Bits:       436732028,
			Nonce:      3694498168},
		Height:            1573650,
		TransactionHashes: []*chainhash.Hash{txHash},
		Size:              216,
	}
	peer := &MockedPeer{}

	// call tested function
	err := peerHandler.HandleBlock(blockMessage, peer)
	require.NoError(t, err)
}
