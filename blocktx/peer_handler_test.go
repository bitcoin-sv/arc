package blocktx

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/blocktx/store"
	"github.com/libsv/go-bc"
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
	// define HandleBlock function parameters (BlockMessage and p2p.PeerI)
	//txHashes :=

	prevBlockHash1573650, _ := chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
	merkleRootHash1573650, _ := chainhash.NewHashFromStr("3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f")

	prevBlockHash1584899, _ := chainhash.NewHashFromStr("000000000000370a7d710d5d24968567618fa0c707950890ba138861fb7c9879")
	merkleRootHash1584899, _ := chainhash.NewHashFromStr("de877b5f2ef9f3e294ce44141c832b84efabea0d825fd3aa7024f23c38feb696")
	tt := []struct {
		name          string
		prevBlockHash chainhash.Hash
		merkleRoot    chainhash.Hash
		height        uint64
		txHashes      []string
		size          uint64
		nonce         uint32

		expectedMerkle string
	}{
		{
			name:          "block height 1573650",
			txHashes:      []string{"3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f"},
			prevBlockHash: *prevBlockHash1573650,
			merkleRoot:    *merkleRootHash1573650,
			height:        1573650,
			nonce:         3694498168,
			size:          216,

			expectedMerkle: "fe12031800010100027fda0fc8f5d26a8616869add086c8421fa07245a96d1b6ac5ae8d46bbbb2643d",
		},
		{
			name: "block height 1584899",
			txHashes: []string{
				"30f00edf09d7c4483509a52962e2e6ddfd16a0a146b9068288b1a5a2242e5c7b",
				"63dc4a8c11ec26e141f501e5c0dfa19b463eb5660e483ca5e0c8520979bb37bb",
				"fe220040445774788309ef0399939b70b90f7182dbf3ff24b2eaf6eeac04d395",
				"dcd51904bc0e58199b0c6fa37b8fe3b6f8ba696e6af8ecff27fe181f173346f4",
				"192ec6b58f1087f68728aabac2ce37ebe66e9bfc6f3af51cd39a2535e1100353",
				"e45955e1b4b7d184ffa3f2469f18b4f9b604dce1ba2265523ec2f407ed99ee14",
				"1d03c4f081a9c41b6ec1e45c1edb411de2765f0df3c7dfd5c91f49509af18960",
				"7607fabbd665e1b540647d0df197ec272751257a83265fe6d312909909c25827",
				"4c870f373eac5fb6f0a9e98dce2970047ad9c9f5b0479ae78bab86432439718a",
				"0e28a91a0ff248ef33dba449299a6663b5401f32695b22cb5ee21e0cd2a822d9",
				"d7f5f4ba7d1ae16cc6ff320693bc4299b4117e64afb0e2cc0634950d5a4d054f",
				"c4cebb360bc82d1a6bd1aad631a825ec0dd57eea6964b29551616486255399e1",
				"6346a7249eb0c40efcd5674f0f022e17b720d6f263be2cd2637326f3ee80d16f",
				"d0d4eaaf40a4414f11f895b66ee0ecbe2f71033b45e2faeea2805c9c1da976ef",
			},
			prevBlockHash: *prevBlockHash1584899,
			merkleRoot:    *merkleRootHash1584899,
			height:        1584899,
			nonce:         1234660301,
			size:          3150,

			expectedMerkle: "fe032f1800041000027b5c2e24a2a5b1888206b946a1a016fddde6e26229a5093548c4d709df0ef0300100bb37bb790952c8e0a53c480e66b53e469ba1dfc0e501f541e126ec118c4adc63020095d304aceef6eab224fff3db82710fb9709b939903ef098378745744400022fe0300f44633171f18fe27ffecf86a6e69baf8b6e38f7ba36f0c9b19580ebc0419d5dc0400530310e135259ad31cf53a6ffc9b6ee6eb37cec2baaa2887f687108fb5c62e19050014ee99ed07f4c23e526522bae1dc04b6f9b4189f46f2a3ff84d1b7b4e15559e406006089f19a50491fc9d5dfc7f30d5f76e21d41db1e5ce4c16e1bc4a981f0c4031d07002758c209999012d3e65f26837a25512727ec97f10d7d6440b5e165d6bbfa077608008a7139244386ab8be79a47b0f5c9d97a047029ce8de9a9f0b65fac3e370f874c0900d922a8d20c1ee25ecb225b69321f40b563669a2949a4db33ef48f20f1aa9280e0a004f054d5a0d953406cce2b0af647e11b49942bc930632ffc66ce11a7dbaf4f5d70b00e19953258664615195b26469ea7ed50dec25a831d6aad16b1a2dc80b36bbcec40c006fd180eef3267363d22cbe63f2d620b7172e020f4f67d5fc0ec4b09e24a746630d00ef76a91d9c5c80a2eefae2453b03712fbeece06eb695f8114f41a440afead4d00e010f0108000064582b9ba76731f52e5a518ba8b650c2d4daa77e62ae2ae7afce99ea31969ccc01006c659b680371a5281be16327861f19c86bc1ba1677ba6c47db50fe8efd827ec6020070635357c7cfe97348156239c12739747cf013b0193e9b6a5d52198b852b87080300556fc354836aeeeff96af64e0c2fb3c7e77555473c307cada9f2e25cda836de70400adfe6cdd5cb1a0585eb520af80dbdaa621e3be9dd989b09bcdee4d7bcc2028580500b5237d6341cb5b875ad053efa32014dc7f5d947396fcaa017d98491b9024fc890600d1d75c691a554cf99c8017565c9b676aa6c22319ae34fe10316281099449ab70070104000025d4ad6487fd8d288249d1f04919a652bc317a272698e649518a373874109a3101007b50b06ebc1e2819ec9ed5d8168f418cf2f9a36013baef58336c85b51da5b3e5020012cec9ae166a2ec49b7a96137ef6f5c975286957df530d6fb9fd99f4e4d447bd0300a8ca17600e2481bbb3d1e36ed232a932316c69f5db2768e79c015cfde9e8be1b0200004dd16dd256ddbe4e7d45da3a357090cb9aeb07aa8d76a33650eb1a3876ff12f201005e852bfcff3fdbe53d0c17ef837a8f8c56c72d8ac42cd82177d88de4dfbac723",
		},
	}

	batchSize := 4
	var storeMock = &store.InterfaceMock{
		GetBlockFunc: func(ctx context.Context, hash *chainhash.Hash) (*blocktx_api.Block, error) {
			return &blocktx_api.Block{}, nil
		},
		InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
			return 0, nil
		},
		MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
			return nil
		},
	}
	// create mocked peer handler
	var blockTxLogger = gocore.Log("btx", gocore.NewLogLevelFromString("INFO"))

	bockChannel := make(chan *blocktx_api.Block, 1)

	// build peer manager
	peerHandler := NewPeerHandler(blockTxLogger, storeMock, bockChannel, WithTransactionBatchSize(batchSize))

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			expectedInsertedTransactions := []*blocktx_api.TransactionAndSource{}
			transactionHashes := make([]*chainhash.Hash, len(tc.txHashes))
			for i, hash := range tc.txHashes {
				txHash, err := chainhash.NewHashFromStr(hash)
				require.NoError(t, err)
				transactionHashes[i] = txHash

				expectedInsertedTransactions = append(expectedInsertedTransactions, &blocktx_api.TransactionAndSource{Hash: txHash[:]})
			}

			var insertedBlockTransactions []*blocktx_api.TransactionAndSource

			storeMock.InsertBlockTransactionsFunc = func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) error {

				require.True(t, len(merklePaths) <= batchSize)
				require.True(t, len(transactions) <= batchSize)

				for i, path := range merklePaths {
					bump, err := bc.NewBUMPFromStr(path)
					require.NoError(t, err)
					tx, err := chainhash.NewHash(transactions[i].Hash)
					require.NoError(t, err)
					root, err := bump.CalculateRootGivenTxid(tx.String())
					require.NoError(t, err)

					require.Equal(t, root, tc.merkleRoot.String())
				}

				insertedBlockTransactions = append(insertedBlockTransactions, transactions...)
				return nil
			}

			peer := &MockedPeer{}

			blockMessage := &p2p.BlockMessage{
				Header: &wire.BlockHeader{
					Version:    541065216,
					PrevBlock:  tc.prevBlockHash,
					MerkleRoot: tc.merkleRoot,
					Bits:       436732028,
					Nonce:      tc.nonce,
				},
				Height:            tc.height,
				TransactionHashes: transactionHashes,
				Size:              tc.size,
			}

			// call tested function
			err := peerHandler.HandleBlock(blockMessage, peer)
			require.NoError(t, err)

			require.ElementsMatch(t, expectedInsertedTransactions, insertedBlockTransactions)
			<-bockChannel
		})
	}
}
