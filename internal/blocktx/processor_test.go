package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/mq"
	p2p_mocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

func TestHandleBlock(t *testing.T) {
	prevBlockHash1573650, _ := chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
	merkleRootHash1573650, _ := chainhash.NewHashFromStr("3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f")

	prevBlockHash1584899, _ := chainhash.NewHashFromStr("000000000000370a7d710d5d24968567618fa0c707950890ba138861fb7c9879")
	merkleRootHash1584899, _ := chainhash.NewHashFromStr("de877b5f2ef9f3e294ce44141c832b84efabea0d825fd3aa7024f23c38feb696")

	prevBlockHash1585018, _ := chainhash.NewHashFromStr("00000000000003fe2dc7e6ca0a37cb36a00742459e65a048d5bee0fc33d9ad32")
	merkleRootHash1585018, _ := chainhash.NewHashFromStr("9c1fe95a7ac4502e281f4f2eaa2902e12b0f486cf610977c73afb3cd060bebde")

	tt := []struct {
		name                  string
		prevBlockHash         chainhash.Hash
		merkleRoot            chainhash.Hash
		height                uint64
		txHashes              []string
		size                  uint64
		nonce                 uint32
		blockAlreadyProcessed bool
	}{
		{
			name:                  "block height 1573650",
			txHashes:              []string{}, // expect this block to not be processed
			prevBlockHash:         *prevBlockHash1573650,
			merkleRoot:            *merkleRootHash1573650,
			height:                1573650,
			nonce:                 3694498168,
			size:                  216,
			blockAlreadyProcessed: true,
		},
		{
			name:          "block height 1573650",
			txHashes:      []string{"3d64b2bb6bd4e85aacb6d1965a2407fa21846c08dd9a8616866ad2f5c80fda7f"},
			prevBlockHash: *prevBlockHash1573650,
			merkleRoot:    *merkleRootHash1573650,
			height:        1573650,
			nonce:         3694498168,
			size:          216,
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
		},
		{
			name: "block height 1585018",
			txHashes: []string{
				"be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b",
				"354cb5b9b3586cca8b82025e7a08f1532fc51128d12c0bbf683f54dbb228efac",
				"2c64a04825dfcc0b87d9f31756d590530bf8c12ccf6670275d4970fb954f50e8",
				"cf5211f97fd59250967a17a0ec865665c9232b0b6ee2faa1e13462161a5509eb",
				"f53c84e09a1628eced8160b14b725cb184d46bf4ee92688372ef019f484ed214",
				"2a5d9ab4e810280dd994dc2eaf7fbe17b245e79b7808297b96f1e0dcc1b37dc6",
				"bdd25de67ea06af3651650a991dc742b4a56ee0707a498fc3aade4343a87fe6d",
				"7d4c950c903f8e4f027bbc5ef34ed189ace85e97ae938591cd5f35b6d2c81dfc",
				"13f2cc98b6a0dd7868853fb7d062391bd0f5c7fe759cf5dc25e269967b36c758",
				"2c9847577ca9ad986b3d1698c03c138d7160c50c16df36bbed2904c1d0b081a0",
				"5abeb598521c1c882f53543fc76bd2321f8bc154b25bccb177dc42f7879a66d8",
				"698a2a78ec1df92355878d9b94cca0a3a15008d15896e24697d5f2c3fb4f0b4b",
				"705a39d2accb41396023a58efbb07e7d508441d21abb0eb9c86a0f7070d4c697",
				"911de4d920159eb622be70f4323c572fe9dd5296e0e2be0611c04920234b810c",
				"b672c3a3a36ce458c2f9424bf35f30fa901580ea07483952d87cedac1c1cb9c0",
				"55b42c74269fd4e38ed1af18793a6e4cba9bba98ab07f7557ab7a05e03f9c74e",
				"89bf69fb351780acf4355a724bcb235374fd9be9fa5872686344896564831989",
				"32a852381b173ca5a2e1119c915c1c0b86df05e6a6198a857b47b098ab5181e1",
				"c79f4a9ff600f50f8da1f876d61aabfd13528d42ddc7f287eee87463439037ca",
				"ce6edcf9908746dac19ac6930d28e1709194d07175da22e9cfe60c54a4ad5f68",
				"c81f1b73a08c471a0bb2ba89fb187511ac35ccc074ca83298a84af1415e84102",
				"0c70102c0d4b87b39a81224981b5a45efdd72cc58c68aa63fd2f055cf5a17cf1",
				"c2038799cb0cab540a9ccd341b2e668ab59a495464b34b277933392f2dba8c12",
				"b33853a4512d22b65cc2ba188e852ca76dd094ab648f461c35875e13b2bb6562",
				"394450c8925e85950334dc1bbbb6387ba82d11337243fff6f8cfde80c7fc076c",
				"bab71c7a6b3d28714f41459dea64ee81ceb0f595e58623b182ab7fd1cf52201e",
				"815b4ec9bec4704c2cf18f0c40a63506a50a647ed5cd4ac7d5c07b0e0e474b0e",
				"3e28c488913e02a47ca81a58e60e6b26c7b483a83d04e7ef6af40ce4cb0fe016",
				"7b5b21da0cedf04bcde86da5dd6bc0db94939053f41d62eb85c950f4fff438e9",
				"1acad7f15e2d4e293949312db3b38d850e7ecb474615ba2df5daad9a0b375e63",
				"a010a3f4171eb88b43d1bc7bbff5a60a5422fb09e8d7e5bc74419b497fa9c63c",
				"df2f423e062e662fad0b29dbd284def3316707767da745f37d1ee5b6beb25781",
				"986a1ff920690de8819abc3d16c91e3cae30d3c5524a6bf895bbb54890736754",
				"056252b2fc1c5b49b7960c27b41b7dfcc28f4087d6c1095ae9dff040d8a39152",
				"1c4708b13d5d1600b1b3b02f5e224aa1edb6730a26ecb0a46e3668793bb4d52b",
				"d97f66206650361ba9bc975266d086692a25c31024f9f4ceaa6e43367787f941",
				"3664bb8205fa806515f1a128ae3c760e34fcb1e78d30c00d2b0bf3ea7d833ef7",
				"d5243d3bc735898c7284b3c7c368fc06543b129c2f2b40836d55f0be5107bf85",
				"e1b74f95639dbe35d01fd72b27214beb224e601c93c669220988f295d948a985",
				"0d93038b34f1d024ee7d942f253b8218d38a3f19e580ec6d7700b24801b62cb2",
			},
			prevBlockHash: *prevBlockHash1585018,
			merkleRoot:    *merkleRootHash1585018,
			height:        1584899,
			nonce:         1428255133,
			size:          8191650,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			const batchSize = 4

			transactionHashes := make([]*chainhash.Hash, len(tc.txHashes))
			for i, hash := range tc.txHashes {
				txHash, err := chainhash.NewHashFromStr(hash)
				require.NoError(t, err)
				transactionHashes[i] = txHash
			}

			actualInsertedBlockTransactionsCh := make(chan string, 100)

			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockFunc: func(_ context.Context, _ *chainhash.Hash) (*blocktx_api.Block, error) {
					if tc.blockAlreadyProcessed {
						return &blocktx_api.Block{Processed: true}, nil
					}
					return nil, store.ErrBlockNotFound
				},
				GetLongestBlockByHeightFunc: func(_ context.Context, _ uint64) (*blocktx_api.Block, error) {
					return nil, store.ErrBlockNotFound
				},
				GetChainTipFunc: func(_ context.Context) (*blocktx_api.Block, error) {
					return nil, store.ErrBlockNotFound
				},
				UpsertBlockFunc: func(_ context.Context, _ *blocktx_api.Block) (uint64, error) {
					return 0, nil
				},
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return nil, nil
				},
				GetRegisteredTxsByBlockHashesFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return nil, nil
				},
				GetBlockTransactionsHashesFunc: func(_ context.Context, _ []byte) ([]*chainhash.Hash, error) {
					return nil, nil
				},
				MarkBlockAsDoneFunc: func(_ context.Context, _ *chainhash.Hash, _ uint64, _ uint64) error { return nil },
			}

			storeMock.InsertBlockTransactionsFunc = func(_ context.Context, _ uint64, txsWithMerklePaths []store.TxHashWithMerkleTreeIndex) error {
				require.LessOrEqual(t, len(txsWithMerklePaths), batchSize)

				for _, txWithMr := range txsWithMerklePaths {
					tx, err := chainhash.NewHash(txWithMr.Hash)
					require.NoError(t, err)

					actualInsertedBlockTransactionsCh <- tx.String()
				}

				return nil
			}

			mq := &mocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error { return nil },
			}

			logger := slog.Default()
			blockProcessCh := make(chan *bcnet.BlockMessage, 1)
			p2pMsgHandler := blocktx_p2p.NewMsgHandler(logger, nil, blockProcessCh)

			sut, err := blocktx.NewProcessor(logger, storeMock, nil, blockProcessCh, blocktx.WithTransactionBatchSize(batchSize), blocktx.WithMessageQueueClient(mq))
			require.NoError(t, err)

			blockMessage := &bcnet.BlockMessage{
				Hash: testdata.Block1Hash,
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

			// when
			sut.StartBlockProcessing()

			// simulate receiving block from node
			p2pMsgHandler.OnReceive(blockMessage, &p2p_mocks.PeerIMock{StringFunc: func() string { return "peer" }})

			var actualInsertedBlockTransactions []string
			time.Sleep(20 * time.Millisecond)
			sut.Shutdown()

		loop:
			for {
				select {
				case inserted := <-actualInsertedBlockTransactionsCh:
					actualInsertedBlockTransactions = append(actualInsertedBlockTransactions, inserted)
				default:
					break loop
				}
			}

			expectedInsertedTransactions := tc.txHashes
			// then
			require.ElementsMatch(t, expectedInsertedTransactions, actualInsertedBlockTransactions)
		})
	}
}

func TestHandleBlockReorgAndOrphans(t *testing.T) {
	testCases := []struct {
		name                     string
		blockAlreadyExists       bool
		prevBlockStatus          blocktx_api.Status
		hasCompetingBlock        bool
		hasGreaterChainwork      bool
		shouldFindOrphanAncestor bool
		ancestorStatus           blocktx_api.Status
		expectedStatus           blocktx_api.Status
	}{
		{
			name:               "block already exists - should be ingored",
			blockAlreadyExists: true,
			expectedStatus:     blocktx_api.Status_UNKNOWN,
		},
		{
			name:              "previous block longest - no competing - no reorg",
			prevBlockStatus:   blocktx_api.Status_LONGEST,
			hasCompetingBlock: false,
			expectedStatus:    blocktx_api.Status_LONGEST,
		},
		{
			name:                "previous block longest - competing - no reorg",
			prevBlockStatus:     blocktx_api.Status_LONGEST,
			hasCompetingBlock:   true,
			hasGreaterChainwork: false,
			expectedStatus:      blocktx_api.Status_STALE,
		},
		{
			name:                "previous block longest - competing - reorg",
			prevBlockStatus:     blocktx_api.Status_LONGEST,
			hasCompetingBlock:   true,
			hasGreaterChainwork: true,
			expectedStatus:      blocktx_api.Status_LONGEST,
		},
		{
			name:                "previous block stale - no reorg",
			prevBlockStatus:     blocktx_api.Status_STALE,
			hasGreaterChainwork: false,
			expectedStatus:      blocktx_api.Status_STALE,
		},
		{
			name:                "previous block stale - reorg",
			prevBlockStatus:     blocktx_api.Status_STALE,
			hasGreaterChainwork: true,
			expectedStatus:      blocktx_api.Status_LONGEST,
		},
		{
			name:                     "previous block orphaned - no ancestor",
			prevBlockStatus:          blocktx_api.Status_ORPHANED,
			shouldFindOrphanAncestor: false,
			expectedStatus:           blocktx_api.Status_ORPHANED,
		},
		{
			name:                     "previous block orphaned - stale ancestor",
			prevBlockStatus:          blocktx_api.Status_ORPHANED,
			shouldFindOrphanAncestor: true,
			ancestorStatus:           blocktx_api.Status_STALE,
			expectedStatus:           blocktx_api.Status_STALE,
		},
		{
			name:                     "previous block orphaned - longest ancestor - no competing",
			prevBlockStatus:          blocktx_api.Status_ORPHANED,
			shouldFindOrphanAncestor: true,
			ancestorStatus:           blocktx_api.Status_LONGEST,
			hasCompetingBlock:        false,
			expectedStatus:           blocktx_api.Status_LONGEST,
		},
		{
			name:                     "previous block orphaned - longest ancestor - competing - no reorg",
			prevBlockStatus:          blocktx_api.Status_ORPHANED,
			shouldFindOrphanAncestor: true,
			ancestorStatus:           blocktx_api.Status_LONGEST,
			hasCompetingBlock:        true,
			hasGreaterChainwork:      false,
			expectedStatus:           blocktx_api.Status_STALE,
		},
		{
			name:                     "previous block orphaned - longest ancestor - competing - reorg",
			prevBlockStatus:          blocktx_api.Status_ORPHANED,
			shouldFindOrphanAncestor: true,
			ancestorStatus:           blocktx_api.Status_LONGEST,
			hasCompetingBlock:        true,
			hasGreaterChainwork:      true,
			expectedStatus:           blocktx_api.Status_LONGEST,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var mtx sync.Mutex
			insertedBlockStatus := blocktx_api.Status_UNKNOWN
			shouldReturnNoBlock := !tc.blockAlreadyExists

			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockFunc: func(_ context.Context, _ *chainhash.Hash) (*blocktx_api.Block, error) {
					if shouldReturnNoBlock {
						shouldReturnNoBlock = false
						return nil, nil
					}

					return &blocktx_api.Block{
						Status:    tc.prevBlockStatus,
						Processed: true,
					}, nil
				},
				GetLongestBlockByHeightFunc: func(_ context.Context, _ uint64) (*blocktx_api.Block, error) {
					if tc.hasCompetingBlock {
						blockHash, err := chainhash.NewHashFromStr("0000000000000000087590e1ad6360c0c491556c9af75c0d22ce9324cb5713cf")
						require.NoError(t, err)

						return &blocktx_api.Block{
							Hash: blockHash[:],
						}, nil
					}
					return nil, store.ErrBlockNotFound
				},
				GetChainTipFunc: func(_ context.Context) (*blocktx_api.Block, error) {
					return &blocktx_api.Block{}, nil
				},
				UpsertBlockFunc: func(_ context.Context, block *blocktx_api.Block) (uint64, error) {
					mtx.Lock()
					insertedBlockStatus = block.Status
					mtx.Unlock()
					return 1, nil
				},
				GetStaleChainBackFromHashFunc: func(_ context.Context, _ []byte) ([]*blocktx_api.Block, error) {
					if tc.hasGreaterChainwork {
						return []*blocktx_api.Block{
							{
								Chainwork: "62209952899966",
							},
							{
								Chainwork: "42069",
							},
							{
								Chainwork: "42069",
							},
						}, nil
					}
					return []*blocktx_api.Block{
						{
							Chainwork: "62209952899966",
						},
					}, nil
				},
				GetLongestChainFromHeightFunc: func(_ context.Context, _ uint64) ([]*blocktx_api.Block, error) {
					return []*blocktx_api.Block{
						{
							Chainwork: "62209952899966",
						},
						{
							Chainwork: "42069",
						},
					}, nil
				},
				UpdateBlocksStatusesFunc: func(_ context.Context, blockStatusUpdates []store.BlockStatusUpdate) error {
					mtx.Lock()
					tipStatusUpdate := blockStatusUpdates[len(blockStatusUpdates)-1]
					insertedBlockStatus = tipStatusUpdate.Status
					mtx.Unlock()
					return nil
				},
				GetOrphansBackToNonOrphanAncestorFunc: func(_ context.Context, hash []byte) ([]*blocktx_api.Block, *blocktx_api.Block, error) {
					if tc.shouldFindOrphanAncestor {
						orphans := []*blocktx_api.Block{{Hash: hash}}
						ancestor := &blocktx_api.Block{Hash: []byte("123"), Status: tc.ancestorStatus, Processed: true}
						return orphans, ancestor, nil
					}
					return nil, nil, nil
				},
				InsertBlockTransactionsFunc: func(_ context.Context, _ uint64, _ []store.TxHashWithMerkleTreeIndex) error {
					return nil
				},
				GetRegisteredTxsByBlockHashesFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return nil, nil
				},
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return nil, nil
				},
				GetBlockTransactionsHashesFunc: func(_ context.Context, _ []byte) ([]*chainhash.Hash, error) {
					return nil, nil
				},
				MarkBlockAsDoneFunc: func(_ context.Context, _ *chainhash.Hash, _, _ uint64) error {
					return nil
				},
			}

			// build peer manager and processor

			logger := slog.Default()
			blockProcessCh := make(chan *bcnet.BlockMessage, 10)
			p2pMsgHandler := blocktx_p2p.NewMsgHandler(logger, nil, blockProcessCh)

			sut, err := blocktx.NewProcessor(logger, storeMock, nil, blockProcessCh)
			require.NoError(t, err)

			txHash, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
			require.NoError(t, err)
			merkleRoot, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
			require.NoError(t, err)

			blockMessage := &bcnet.BlockMessage{
				Hash: testdata.Block1Hash,
				Header: &wire.BlockHeader{
					Version:    541065216,
					MerkleRoot: *merkleRoot,
					Bits:       0x1c2a1115, // chainwork: "26137323115"
				},
				Height:            123,
				TransactionHashes: []*chainhash.Hash{txHash},
			}

			// when
			sut.StartBlockProcessing()

			// simulate receiving block from node
			p2pMsgHandler.OnReceive(blockMessage, nil)

			// then
			time.Sleep(20 * time.Millisecond)
			mtx.Lock()
			require.Equal(t, tc.expectedStatus, insertedBlockStatus)
			mtx.Unlock()
		})
	}
}

func TestStartProcessRegisterTxs(t *testing.T) {
	tx1 := testutils.Chainhash(t, "ff2ea4f998a94d5128ac7663824b2331cc7f95ca60611103f49163d4a4eb547c")
	tx2 := testutils.Chainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883")
	tx3 := testutils.Chainhash(t, "e2d3bbb6005671db9a47767d2278ebe0d7a5515f5891facbd16dc3963bded337")
	tx4 := testutils.Chainhash(t, "24c23a8213eec1f735384c1056757c2784794474b4b534232d3114526b192e1e")
	tx5 := testutils.Chainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")
	tx6 := testutils.Chainhash(t, "4bb96ac67fad56ef2ea6213a139c0144c86de2f71f6679c832b918e409e46008")
	tx7 := testutils.Chainhash(t, "b3024ceba94a32eeb6c20f2a6fb1442703d090393977997c4e6287e6b8731323")
	tx8 := testutils.Chainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3")
	tx9 := testutils.Chainhash(t, "bf272086e71d381269b33deea4e92b1730bda919b04497aed919694c0d29d264")
	merkleRoot1 := testutils.HexDecodeString(t, "c991fcf57466c387779b009b13c85a8ab31f2e24a3b04e97d2dcbda608f7f2d0")
	blockHash1 := testutils.RevHexDecodeString(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")
	tt := []struct {
		name                string
		batchSize           int
		registerErr         error
		getMinedTxs         []store.BlockTransaction
		getMinedTxsErr      error
		getBlockTxHashes    []*chainhash.Hash
		getBlockTxHashesErr error
		registeredHashes    [][]byte

		expectedPublishCalls     int
		expectedRegisterTxsCalls int
	}{
		{
			name:             "no mined txs",
			registeredHashes: [][]byte{},

			expectedRegisterTxsCalls: 0,
			expectedPublishCalls:     0,
		},
		{
			name:             "error - failed to register",
			batchSize:        1,
			registerErr:      errors.New("failed to register"),
			registeredHashes: [][]byte{tx5[:]},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:                "error - failed to get block tx hashes",
			batchSize:           1,
			getBlockTxHashesErr: errors.New("failed to get block tx hashes"),
			registeredHashes:    [][]byte{tx5[:]},
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          tx5[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
				},
			},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:             "error - failed to get mined txs",
			batchSize:        1,
			registeredHashes: [][]byte{tx5[:]},
			getMinedTxsErr:   errors.New("failed to get mined txs"),

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:             "found 0 block tx hashes",
			batchSize:        1,
			registeredHashes: [][]byte{tx5[:]},
			getBlockTxHashes: []*chainhash.Hash{},
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          tx5[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
			},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:      "wrong order of block tx hashes - BUMP does not contain the txid",
			batchSize: 3,
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          tx5[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
			},
			registeredHashes: [][]byte{tx5[:], tx8[:], tx2[:]},
			getBlockTxHashes: []*chainhash.Hash{tx6, tx7, tx8, tx9, tx1, tx2, tx3, tx4, tx5},

			expectedPublishCalls:     0,
			expectedRegisterTxsCalls: 1,
		},
		{
			name:      "correct order of block tx hashes - success",
			batchSize: 4,
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          tx5[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: -1,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          []byte("not valid"),
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          tx8[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 7,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          tx2[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 1,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
			},
			registeredHashes: [][]byte{tx5[:], tx8[:], tx2[:]},
			getBlockTxHashes: []*chainhash.Hash{tx1, tx2, tx3, tx4, tx5, tx6, tx7, tx8, tx9},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     1,
		},
		{
			name:      "correct order of block tx hashes - two messages - success",
			batchSize: 4,
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          tx1[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 0,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          tx2[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 1,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          tx3[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 2,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
				{
					TxHash:          tx4[:],
					BlockHash:       blockHash1,
					BlockHeight:     1661719,
					MerkleTreeIndex: 3,
					BlockStatus:     10,
					MerkleRoot:      merkleRoot1,
				},
			},
			registeredHashes: [][]byte{tx1[:], tx2[:], tx3[:], tx4[:]},
			getBlockTxHashes: []*chainhash.Hash{tx1, tx2, tx3, tx4, tx5, tx6, tx7, tx8, tx9},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			registerErrTest := tc.registerErr
			storeMock := &storeMocks.BlocktxStoreMock{
				RegisterTransactionsFunc: func(_ context.Context, _ [][]byte) (int64, error) {
					return 0, registerErrTest
				},
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return tc.getMinedTxs, tc.getMinedTxsErr
				},
				GetBlockTransactionsHashesFunc: func(_ context.Context, _ []byte) ([]*chainhash.Hash, error) {
					return tc.getBlockTxHashes, tc.getBlockTxHashesErr
				},
			}
			mqClient := &mocks.MessageQueueClientMock{
				PublishMarshalFunc: func(_ context.Context, _ string, _ protoreflect.ProtoMessage) error {
					return nil
				},
			}

			txChan := make(chan []byte, 10)

			for _, txHash := range tc.registeredHashes {
				txChan <- txHash
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			// when
			sut, err := blocktx.NewProcessor(
				logger,
				storeMock,
				nil,
				nil,
				blocktx.WithRegisterTxsInterval(time.Millisecond*80),
				blocktx.WithRegisterTxsChan(txChan),
				blocktx.WithRegisterTxsBatchSize(tc.batchSize),
				blocktx.WithMessageQueueClient(mqClient),
				blocktx.WithPublishMinedMessageSize(3),
			)
			require.NoError(t, err)

			sut.StartProcessRegisterTxs()

			time.Sleep(120 * time.Millisecond)
			sut.Shutdown()

			// then
			require.Equal(t, tc.expectedRegisterTxsCalls, len(storeMock.RegisterTransactionsCalls()))
			require.Equal(t, tc.expectedPublishCalls, len(mqClient.PublishMarshalCalls()))
		})
	}
}

func TestStartBlockRequesting(t *testing.T) {
	blockHash, err := chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
	require.NoError(t, err)

	tt := []struct {
		name                  string
		setBlockProcessingErr error
		bhsProcInProg         []*chainhash.Hash

		expectedSetBlockProcessingCalls int
		expectedPeerWriteMessageCalls   int
	}{
		{
			name: "process block",

			expectedSetBlockProcessingCalls: 1,
			expectedPeerWriteMessageCalls:   1,
		},
		{
			name:                  "block processing maximum reached",
			setBlockProcessingErr: store.ErrBlockProcessingMaximumReached,

			expectedSetBlockProcessingCalls: 1,
			expectedPeerWriteMessageCalls:   0,
		},
		{
			name:                  "block processing already in progress",
			setBlockProcessingErr: store.ErrBlockProcessingInProgress,

			expectedSetBlockProcessingCalls: 1,
			expectedPeerWriteMessageCalls:   0,
		},
		{
			name:                  "failed to set block processing",
			setBlockProcessingErr: errors.New("failed to set block processing"),

			expectedSetBlockProcessingCalls: 1,
			expectedPeerWriteMessageCalls:   0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			setBlockProcessingErrTest := tc.setBlockProcessingErr
			storeMock := &storeMocks.BlocktxStoreMock{
				SetBlockProcessingFunc: func(_ context.Context, _ *chainhash.Hash, _ string, _ time.Duration, _ int) (string, error) {
					return "abc", setBlockProcessingErrTest
				},
			}

			peerMock := &p2p_mocks.PeerIMock{
				WriteMsgFunc: func(_ wire.Message) {},
				StringFunc:   func() string { return "peer" },
			}

			// build peer manager
			logger := slog.Default()

			blockRequestCh := make(chan blocktx_p2p.BlockRequest, 10)
			blockProcessCh := make(chan *bcnet.BlockMessage, 10)

			peerHandler := blocktx_p2p.NewMsgHandler(logger, blockRequestCh, blockProcessCh)

			sut, err := blocktx.NewProcessor(logger, storeMock, blockRequestCh, blockProcessCh)
			require.NoError(t, err)

			// when
			sut.StartBlockRequesting()

			// simulate receiving INV BLOCK msg from node
			invMsg := wire.NewMsgInvSizeHint(1)
			err = invMsg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, blockHash))
			require.NoError(t, err)
			peerHandler.OnReceive(invMsg, peerMock)

			time.Sleep(200 * time.Millisecond)

			// then
			defer sut.Shutdown()

			require.Equal(t, tc.expectedSetBlockProcessingCalls, len(storeMock.SetBlockProcessingCalls()))
			require.Equal(t, tc.expectedPeerWriteMessageCalls, len(peerMock.WriteMsgCalls()))
		})
	}
}

func TestStart(t *testing.T) {
	tt := []struct {
		name     string
		topicErr map[string]error

		expectedError error
	}{
		{
			name: "success",
		},
		{
			name:     "error - subscribe mined txs",
			topicErr: map[string]error{mq.RegisterTxTopic: errors.New("failed to subscribe")},

			expectedError: blocktx.ErrFailedToSubscribeToTopic,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			mqClient := &mocks.MessageQueueClientMock{
				SubscribeFunc: func(topic string, _ func([]byte) error) error {
					err, ok := tc.topicErr[topic]
					if ok {
						return err
					}
					return nil
				},
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			sut, err := blocktx.NewProcessor(logger, nil, nil, nil, blocktx.WithMessageQueueClient(mqClient))
			require.NoError(t, err)

			// when
			err = sut.Start(false)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			// cleanup
			sut.Shutdown()
		})
	}
}
