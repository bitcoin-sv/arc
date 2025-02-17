package blocktx_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
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
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte, _ bool) ([]store.BlockTransaction, error) {
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
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte, _ bool) ([]store.BlockTransaction, error) {
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
			registeredHashes: [][]byte{testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:]},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:                "error - failed to get block tx hashes",
			batchSize:           1,
			getBlockTxHashesErr: errors.New("failed to get block tx hashes"),
			registeredHashes:    [][]byte{testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:]},
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
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
			registeredHashes: [][]byte{testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:]},
			getMinedTxsErr:   errors.New("failed to get mined txs"),

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     0,
		},
		{
			name:             "found 0 block tx hashes",
			batchSize:        1,
			registeredHashes: [][]byte{testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:]},
			getBlockTxHashes: []*chainhash.Hash{},
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
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
					TxHash:          testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
				},
			},
			registeredHashes: [][]byte{
				testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
				testutils.RevChainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3")[:],
				testutils.RevChainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883")[:],
			},
			getBlockTxHashes: []*chainhash.Hash{
				testutils.RevChainhash(t, "4bb96ac67fad56ef2ea6213a139c0144c86de2f71f6679c832b918e409e46008"),
				testutils.RevChainhash(t, "b3024ceba94a32eeb6c20f2a6fb1442703d090393977997c4e6287e6b8731323"),
				testutils.RevChainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3"),
				testutils.RevChainhash(t, "bf272086e71d381269b33deea4e92b1730bda919b04497aed919694c0d29d264"),
				testutils.RevChainhash(t, "ff2ea4f998a94d5128ac7663824b2331cc7f95ca60611103f49163d4a4eb547c"),
				testutils.RevChainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883"),
				testutils.RevChainhash(t, "e2d3bbb6005671db9a47767d2278ebe0d7a5515f5891facbd16dc3963bded337"),
				testutils.RevChainhash(t, "24c23a8213eec1f735384c1056757c2784794474b4b534232d3114526b192e1e"),
				testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08"),
			},

			expectedPublishCalls:     0,
			expectedRegisterTxsCalls: 1,
		},
		{
			name:      "correct order of block tx hashes - success",
			batchSize: 4,
			getMinedTxs: []store.BlockTransaction{
				{
					TxHash:          testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: -1,
					BlockStatus:     10,
				},
				{
					TxHash:          []byte("not valid"),
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: 4,
					BlockStatus:     10,
				},
				{
					TxHash:          testutils.RevChainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: 7,
					BlockStatus:     10,
				},
				{
					TxHash:          testutils.RevChainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883")[:],
					BlockHash:       testutils.RevChainhash(t, "000000000286022fd64a660ec826def3b9cfc6cfcf3d0c7ee992cc81385fef44")[:],
					BlockHeight:     1661719,
					MerkleTreeIndex: 1,
					BlockStatus:     10,
				},
			},
			registeredHashes: [][]byte{
				testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08")[:],
				testutils.RevChainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3")[:],
				testutils.RevChainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883")[:],
			},
			getBlockTxHashes: []*chainhash.Hash{
				testutils.RevChainhash(t, "ff2ea4f998a94d5128ac7663824b2331cc7f95ca60611103f49163d4a4eb547c"),
				testutils.RevChainhash(t, "07c2c86c79713d62a6a1776ce926f7748b5f763dfda920957ab819d159c00883"),
				testutils.RevChainhash(t, "e2d3bbb6005671db9a47767d2278ebe0d7a5515f5891facbd16dc3963bded337"),
				testutils.RevChainhash(t, "24c23a8213eec1f735384c1056757c2784794474b4b534232d3114526b192e1e"),
				testutils.RevChainhash(t, "ff1374619d6d6a7b30f35560f76a97aa430c016459034afa6d6cbd19a428ce08"),
				testutils.RevChainhash(t, "4bb96ac67fad56ef2ea6213a139c0144c86de2f71f6679c832b918e409e46008"),
				testutils.RevChainhash(t, "b3024ceba94a32eeb6c20f2a6fb1442703d090393977997c4e6287e6b8731323"),
				testutils.RevChainhash(t, "d828d1d8b8a53c17b6251bc357ff65771b3df5d28b5f009ee97d860b730502d3"),
				testutils.RevChainhash(t, "bf272086e71d381269b33deea4e92b1730bda919b04497aed919694c0d29d264"),
			},

			expectedRegisterTxsCalls: 1,
			expectedPublishCalls:     2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			registerErrTest := tc.registerErr
			storeMock := &storeMocks.BlocktxStoreMock{
				RegisterTransactionsFunc: func(_ context.Context, _ [][]byte) error {
					return registerErrTest
				},
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte, _ bool) ([]store.BlockTransaction, error) {
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
			topicErr: map[string]error{blocktx.RegisterTxTopic: errors.New("failed to subscribe")},

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

func TestCalcMerkle(t *testing.T) {
	tt := []struct {
		name       string
		txID       string
		merklePath string
	}{
		{
			txID:       "08a473b380b1a7b817b97542e47ad76b08b1b33c562029c0a38190a216c2e15f",
			merklePath: "fecb7e0d00080205009425c509467db794fbfb2bfa37944fb363e31e47c635bc36e3cbcc078c49c38c0402aa649ca7138fef9da921a21df99672b443fa96f912cd57b2a30e1adf4d9ad2f30103002d96a1d3caaf1d00f5ef61a062337f710f9307c9785c648084c838a50d9ca4800100001bfe49c369544fed461dcbd451b703844168bfbcac6cd2c47b0888ab8e55d129010100337868843989dedacfb1af353e0ebd18abb5d0d6a079d6222af2ebd731f9dee30101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "af61c278c1e4b2a1773c92411649e6a3e81a2879231bc4b79f9c34919bdd803a",
			merklePath: "fecb7e0d0008020a004d33e11eaa5f16acd0658ff9c9fef20c5d6fb3b1e039ad7a4ebdacb49277aaa60b02c86dceb6011b7373619417bac459bb3f21ae123bbe3c3f957ec46f528ee53832010400c552532a6079f184b8250e1badfd3c1975becd5b19b200e095376a9d2a89277901030074c77f1a2269b92cadc474b4c37b8b6c47ce58feb2bede50f1a206f32289e081010000a06f6d08de1b2c77f695c2f0d9647f803e6f24e9f42d8fb013a91a51dd887ecc0101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "543076f7d696eabe234387d7bac657a4918d570b02225ae20d6cc260732b87a9",
			merklePath: "fecb7e0d000802110068977b9d2206ce70f058e7c7513609c925dee031b3ae2ee4e4f86f8b652c31001002026aa1f95097c9b87acf88bb0ee4f7c0f144b6438bc6b2f8f6ef660204d3cfb2010900ad60c786cac46fdc1ec6521bfc14c026aab68437b86c2cb225189f692db8372c010500af4c9ae8adcb7e32f45b6cc48f8925886175bf753ff14bb68d552132a6e79e0c01030028da2103f8aa795891dfadfa6dc4b0381f61dbbdc0193f96eb3c12eaac4a229b0100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "ef338b59876317ff578463e592ef9135dd199cf7132ffb2c5d920d50d92e2f04",
			merklePath: "fecb7e0d0008021b009d35af70151e31fc984a52644eb54dc1c1bf5a0017a43fdef6004ffc3f0bb0611a027a127cbdc7da7dbc82bcfa39fc8b12b753f9bde4d4a5d0192aa418262617a3a3010c0075f88026f5fb4d61ab013fb25f92f0e976107d9dbaf8428384d95d805793c8bc0107004f99b8eaaee34ea659b6e78659aac43969511f09c91690110b2308cf2cc6b3bc01020083042726de64ea7ac92d003d018c35e05e12c25a7e79d0e391608d8bed53c2b10100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "e49eb190405f88af03c070fe7222987269af2b1d0de7d6456342d80ca901c7de",
			merklePath: "fecb7e0d000802cc0031f24182f4678d16518a4a48bcf174954b1d02924dbfdb40f9bc29f40f85d6b8cd026b348937e610f735b84695c38bd0480af874113b89e33f71c14a9562ea2b67360167008e6a8fbda81e4e470983cc4073a8e9d4e22db343bdcd7a230f7e91ad14ffcd44013200fa96b27795965f214732d40f39767b0216fe2799cc0f38a7b0bf5bc37eecd554011800243e574de078804478faa2fc8d0f2453857545ff3f875df7f92a234b2344d951010d00f1ccb0f27a84a41e978e6acf0f8398bfad21a2290bd9b7e4543c5e3b274950a40107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "1d4737d04ddbc4e2f0337b3810057f0d159621bf27289ba9cd75f9840f0d153f",
			merklePath: "fecb7e0d0008021e00a08fc40f7394732c1927545bbf696c832adb70be9b3708bfd124515734e9e2af1f024ba255d7364c6e59a919a47504a5b59899b1264cc050f16711226ff1776773e7010e00dd8dd3f000fc1cb472a70fa3aef7157542dd466e9ff7f7f946b42d62e707be0901060021fc17fd706c9ae4d082f9156297beded60b3548b91bd2b884f3fd13bcd3d11f01020083042726de64ea7ac92d003d018c35e05e12c25a7e79d0e391608d8bed53c2b10100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "700e2cb097646b84b3a83e40e4972b1dc86415eeb1df80874ab12a2df81f5b61",
			merklePath: "fecb7e0d0008022500c4447c562fd28db9d81ffb214960c1a8721584ee73cee33ea22e8639e5516d912402381309fac47e35668dd095006c3e30864b833fef0c0422a615770358522149d0011300aa175806f4981a9aa110071dbe31ce64ed27f98281be01b21b6682a16e06835c01080064ba40d8646a3cdbb873317ff85a981e4546b52e273f02ef843ff6e4cf5d61650105002b08907dbf41d0e67d2547caba86111df0f806793c2eaeaa1c538f5b57f0a2900103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "9f9a9c4943760ceb95d44f8fa84d5b89d3f4ee65fe7dec7691a73160f98d4ac3",
			merklePath: "fecb7e0d0008022b002b4955666ce14121457244c087fa2f9548cc6211ff89728cd7bdc1c6ea5e3a0b2a02e3f16b73d8f6f5c0cfb66e78ff03a749ebd9d06479e5118e8ae6462463789840011400727e0abfb476c0d696ea37c54e784402cca0bad7b331b6ad81c73f06607dcbb7010b001fa937279508dd2d0866cb58e0599487d1eda25015004a6c4802560a87983d14010400705b50e71669191f10502ef1c092b1ee30146f9ee43ab80d230f9533ff3f129a0103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "46e23542fca6488abebfe4435e38dfd7e4139335fdf1cc45618132353347b501",
			merklePath: "fecb7e0d0008023100a2f02033eb1cc8d82bc510b9088f93451146f97f4fac7c7033e544244cd8093d30029cefac18f8d873c311e03ce64f4d04e74cb928992aa6ccf70e2df7125ecba32e011900e73845a964744941aab1487615e8cda341bc90b90ff077c74a1d81dde711f1e7010d00c45d6601d965a46b5da957b61278396f3bcd8adbf91100eab299c56e8c8dcd6e0107000b865d50e9d261f269c7c18049ccf18957968961406fd90614aaf09ef8513e9d01020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "87f7e05be614f47edfe6a845613eb0538826614eabdc85923bd69ab2cbc55cd8",
			merklePath: "fecb7e0d0008023400dfda0cedbe9751c887138b08270b493a1fabed2bfa1b52aeea4fd4f8be8fa0cc3502913b80e5966edafa4b4ec9cdb306179c8742a24333667a05ddea3d0e08c25a36011b00001953b57629ff860e4c6ea007905dde7c712c7615ea483f8a31c2911d2be9f6010c00ad43c4dabf6ef080d151afb909d5a53c7b659b401b41c50e13ed7b4ab2ed4d680107000b865d50e9d261f269c7c18049ccf18957968961406fd90614aaf09ef8513e9d01020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "a547ab866310083d19b1969d5a131c2e76e31a8ffbbaf1099a29c7c4b7899056",
			merklePath: "fecb7e0d0008023f00c6eef165929146181a6816bdd4350a710f3e495fbaece86bc7af0efe9416d42a3e0203d34c90325207c9d2f97aced78ed3f43f14c60b5a5672f392a19715f69fde68011e009c969f6d33466d1e7a3ebcceffd052fdcc91a4ec73c8ba5a3c2b5fd3a8a00dd8010e00ae408a1e7e2a50c82455598bf5584b39e1df59608a052ff03e6b9a7d0495588f0106002a8b6783d6184ddedd66b782a26725e272ce3f8840bfbbf4d65fa33f1a9f1ad801020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "75531f937eea9562336d4927ff77094b96fd8364610a2e1b527e351c6909b914",
			merklePath: "fecb7e0d000802430087d7a1fe9108e0e2c3fea8aeb224adfb7c7a5ef45220019d7a61c0db0fa9ecb14202e1f19fabe362b027f60ca0ca539a2f5aeab52eab7c8b9d9d54adad33f80333fe0120006d60998029ec663cc77484603efd0716d5a2cc33baf6bf33ef73056955337bd90111001f6bb660213452b7780436725814d94813eb6b52e385822b876599ec0192a056010900c25180cb837b1c74241bb26b02f188dc1422a9c077a5c9b9c6ad9c28eecb534e0105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "ea3d04c1284ee9e7bdd4637cdbaaaddc381ed0b7e0b4e852eb1f8a455b34eac9",
			merklePath: "fecb7e0d0008024700bf3bc27e13dbfa4f2b5156e8a65dfb44da72de98a530fad17f9693d6ab36d21f4602d9d4bb08f8734970f4da15ec9847e2dabf74fd689cda53cbb5d7650a652f1aa80122000cb5bc184779e3bcad72bcd6f8d4418dd8b30ae22090746bf1cf9cfa50aec96a0110004484f7caeaa6f4a737698005d8fefed9d2ec0493a35e7688262c091ab9f139f9010900c25180cb837b1c74241bb26b02f188dc1422a9c077a5c9b9c6ad9c28eecb534e0105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "112c31df05b3e53aaaafb424b0a945719987a6fc8dee880ed9687cb80e64492f",
			merklePath: "fecb7e0d0008024f00f72c21c55c9e1540c92b2009d76c48a66e5e89e6bd952e96074f0626ea2231844e02c13410410f67bb33183e545e051875e602a9f92fc2340e861ed23aa3d267ab56012600c9e22c3fc3a0416c5a5936526a9aaa7a21f752dcd52419741748d52ae4e90c020112009ef416eeb317e32216a6cbd683dc191a1685b4df8e80f8080c354f75c6a6a2320108000d285959d969da10d98b0ae7ab2ef6ebf71de6382eec4a70f2f8351907c0f5150105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "54e73b31d1ce034f4a86356aedaa4f654a446bd78f12022181c76f7939a917ac",
			merklePath: "fecb7e0d000802520058d3affeae2018d5ad900a1508ea026b389e4025443a6199d03e55436e20d7ee5302f6441914d86f6e39e1f98705366985a1e12a6459a042eae3c70b6f980d1de79a01280005cf81645bd4f539c4033c99f04183bab9bd45a57792c5b8c7078d75ba3f1afd011500b2ad1617e367ac311209af46b981f09064fbc4b0ad183386ad5eedeb80cf7ab5010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "fbda9852127c363e537c39c0b020fbdb75794b93c13eb1106d368ccc5dc13010",
			merklePath: "fecb7e0d0008025600935ebf4d9ee2c099252e8b9131472988bf709ea8b46ed35784c1f92ffd8e36d257021c1c1425bea3d3896f4f71fef28bb336ae3ed6093cdb9e5294aa92f447919ea1012a005162fa94a8dac3e6638fb5182d86e759d53c1b7065157747e7e2ed228c298d6f011400db8ef46ccd1ba67a7a66cb3869ec1805f0aca1a665c5440acbf22080a2f84194010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "bc82af444a1cfa9984dbaf8f3fd8d80b0724ae57ca8692d2895e4281a2c64c62",
			merklePath: "fecb7e0d0008029000982c5e5e293f9d2c4dacbc08325905aceaddd05dbaa19ac479f5a6e0e4faf82491025fe1c216a29081a3c02920563cb3b1086bd77ae44275b917b8a7b180b373a408014900eec277edccc4d538fe504a1ef6d375b7d46c77ec381f7e413354718a0f337b890125008318af79a5eaec2ac392b361dad40cc56de7fc4f8b0aa11c2224a4944ddcef94011300af5b9bd6628404a022b5c3a4ef8e641796663617d485523b44bd1da33548072901080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "68de9ff61597a192f372565a0bc6143ff4d38ed7ce7af9d2c9075232904cd303",
			merklePath: "fecb7e0d00080296009c0ba1131b04c1d2bf674b2e0ac68adfa8a9fbfef8c5c8eff5f7e4dc2d0f301897027f1e0959e2ac49117fcdd363e4c0f1831d371709a28e00dc8ff30709273fa8ba014a002831b4f73645309a44a66ab6dd0aa7e9854c5e12cd3b83979fe10ef77392345b012400430960c35e8e5fc60ecf152603b769f61830a4b1a42b4cb292f47e55bcbfae3e011300af5b9bd6628404a022b5c3a4ef8e641796663617d485523b44bd1da33548072901080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "fe3303f833adad549d9d8b7cab2eb5ea5a2f9a53caa00cf627b062e3ab9ff1e1",
			merklePath: "fecb7e0d0008029a0029cfd0c0bc05f0e3561e8ef71c7c5f3fd5e0189267b80fb01ec1a3c6bd231ebf9b023fd8afdd805eb7bd75a73206df29efb7e4e1f837bb2f0e8ae3f359c53de4c2c8014c008a7c073a6393bc50408928af30b631327e2c77d01f421dce772e7d6c2cd5ef1c012700e7f4ee9d1c81908f8708c583b65726e8bf42be7c4980d10f81da0401a87d91bb0112007ef67cbfd4cd37679be1c49b234c7d9ebbab6449854dad3aba0e24605f1d241601080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "443b62ae6ed9a576c378f3483a0f42e861b4cb520dcb4732acd810e3ae5eff1e",
			merklePath: "fecb7e0d000802a300eb10d50a72cae34d74a74db6edeaecd08a28a4862a3701a3aff8e85d22d3ddb0a202dfa92ef5b4defc86d6c9ea9f7e6dff2403dc728eefb046e12c4b53d16800b182015000463dc1a3df8858d0e77832eba695e840ffd97b0eb48d5709519aacb744dc22c60129006f99522db6a78878e013d103891e6ee5013a8c970e455af1b0381bc30491bef9011500f971274b0ccd4b3ec62c4b21c8c3984f1ce55f306143da3acfb4428806e6a248010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "56ab67d2a33ad21e860e34c22ff9a902e67518055e543e1833bb670f411034c1",
			merklePath: "fecb7e0d000802a600168e8b6058900acb8d96879612766fd01e98e195d31448d6150c5ba9c4c9197aa702042f2ed9500d925d2cfb2f13f79c19dd3591ef92e5638457ff176387598b33ef0152009b48f41d8473dfa5a362aa5e480491005a910ff0ca6a93339032c9af3c781c5a012800fdc7244ee20db2d70a35b029d98fe130cf6934e5a9e9c0aa36093d072b799dc5011500f971274b0ccd4b3ec62c4b21c8c3984f1ce55f306143da3acfb4428806e6a248010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "9ae71d0d986f0bc7e3ea42a059642ae1a18569360587f9e1396e6fd8141944f6",
			merklePath: "fecb7e0d000802ad0006e62c14b9992b1307c6f330095c210c16ca093be2939a7fa873e32b661399fcac023f150d0f84f975cda99b2827bf2196150d7f0510387b33f0e2c4db4dd037471d0157005ffacc5d6ea8f0d412c32aef7f0e24452bd1372f7198c28796a0aeaab2d5c10c012a004dc3e6a881507b58d1bae89181226e0ac74b2fe6b36efc259c7015df62a6a5350114004d0833a489352699ccb68a8c426c6f24e4b907dc119ad2b8a0528ed885a7dd5b010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "419b64c75b1fd6e79bd32c50ae7d6825affd1f91a6af7dcf36dc8fb625f848e3",
			merklePath: "fecb7e0d000802b300f7349e0287a27f5e64fc679137fc8ff8b3ce5597d335d1b5e5df06e326c9e2c2b20205e6bcafa6a9a594540dedc1cec622a888c67312bc64e63f9263c14804a3d0ad0158000f013dffff10f982d201b402b108faad4ea57f476041539e816474bad6157232012d000c66152e40c1fc29fa32c5c29ee61b37a324f219cbd8fb123817d920d9e606c6011700019c66fbd59b7813377e19d28acd935c1d580658dcfbc0ae9d7a13317b29b203010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "2b866563cae7a612238ff6801acc12578ad0f0550cc78b175fdd6c3e3598f1e4",
			merklePath: "fecb7e0d000802b700c34a8df96031a79176ec7dfe65eef4d3895b4da88f4fd495eb0c7643499c9a9fb60225d2ffc170080470f98bf29faf237d24e1435cb166ed0cded8c836ff4bddd3bc015a00434629f432bdc691963cb8d45fcd21d29ff74037932876c28d59e0534cc8fc40012c000a36e47684819fcf8ec2840009337cc5235f4627f1cd194f064e725ada7404f1011700019c66fbd59b7813377e19d28acd935c1d580658dcfbc0ae9d7a13317b29b203010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "960d1dbe63c5713074f97091e69124764d0042aa677e149514b6071b74c191f0",
			merklePath: "fecb7e0d000802bd0001b547333532816145ccf1fd359313e4d7df385e43e4bfbe8a48a6fc4235e246bc0238775f0282229ecebddea132d3f619d50d0b8872ce6aa1adf54b48e0037550e8015f0098512c2f568f49f42f50673ab3a8ec109ce51edc8c4575de635531cdf18dc347012e00707715a28ae55a2f27c83c40d3ff815c7bcab30c2f76e376569f698686632d9801160090d25b33dfefc54116268def6a14ad1d57fd0eb33d656f182862ea0495a4f250010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "ff3e1ecaad75000476015894c9b465624a81b958378f8500c79db0cb51942e4f",
			merklePath: "fecb7e0d00080200000169a2377b57fd4bfe3ce915d02fcc2e96d3fa13410e1dca91a320efd90a303901024beb6b35498505a27063e028004a61d426187359612d3cee0fc362940455f042010100d7185833a11de110ac5e4bd354471cb61aa06c2f779358d375f72815b513622c01010044354ba469cb20f1253474126fa7d958934163dbd28fb7aeeeccdfafe0af47af010100337868843989dedacfb1af353e0ebd18abb5d0d6a079d6222af2ebd731f9dee30101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "d13bf9d65ff03a4de8dfc6c4841c132e131708eb2364942a13f344d5ca2c51c2",
			merklePath: "fecb7e0d00080207001e85bcb098081a4b6349cb83ec8b5b77b67ae25bc154f2e5e4b0ec1daeeda94206021762d8c73510b4767e7e3ef26e9220bea040b1715d8675d73c8392495dca04c9010200d38f30f39caa8e9408509a8e85e6485f2d35661def2000eaa1006d2a7648f1760100001bfe49c369544fed461dcbd451b703844168bfbcac6cd2c47b0888ab8e55d129010100337868843989dedacfb1af353e0ebd18abb5d0d6a079d6222af2ebd731f9dee30101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "bf1e23bdc6a3c11eb00fb8679218e0d53f5f7c1cf78e1e56e3f005bcc0d0cf29",
			merklePath: "fecb7e0d0008020c0097e886a8addcbdc1dc52a9e5acf1c52a946a4c7b34d6954d515da17bfdb459210d02a422f66fb4693652c73af689f9e39dc0558692fe16933ed0618a5cd41a5feeba0107003fa14b255319c9592d034db3dc05e02a1c25d33c26a20cb47d4fa58185de8d5d01020074191d9882d39cd83ad39ff93ff5ad401e4427eb82581062bf9f3220bc076f96010000a06f6d08de1b2c77f695c2f0d9647f803e6f24e9f42d8fb013a91a51dd887ecc0101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "b6d0806edb738f0d39a22916cef3c05805cb112f86f47e80fa827a4603df4d46",
			merklePath: "fecb7e0d0008021300bd13697e69f9b803d13d1670a712e92cdb09791ef5eeb0208c29c3ebac88f11e12023902a16b13340cfa0e2a2a814137bf833887971061a292f7e117c3f7f33f1b620108004cf32cf8141915f4d4939d8a272933ee3430403ce2fd35b21c9e87bb37e051a8010500af4c9ae8adcb7e32f45b6cc48f8925886175bf753ff14bb68d552132a6e79e0c01030028da2103f8aa795891dfadfa6dc4b0381f61dbbdc0193f96eb3c12eaac4a229b0100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "0b9a8ed499c6a976fd0594923934f398a10385bb25768b369a3f6cc545a20fad",
			merklePath: "fecb7e0d0008021c0093f715cc6eba6b6d01f1acf12c9dcb128224bb015fe13d2f286fb1f75d97fa3b1d02ae4e27a7d4125f89e46cb9054c5a17e350279168f6d3b6ce63d9fb89a0a7199f010f00e0e10471cb1be60c94ea203d355db2baa086389a3c2c1b87327d2f161d096cbd01060021fc17fd706c9ae4d082f9156297beded60b3548b91bd2b884f3fd13bcd3d11f01020083042726de64ea7ac92d003d018c35e05e12c25a7e79d0e391608d8bed53c2b10100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "6aaa647b468663bd8b9b9646ddea4738135531300880b33b819d80a77d04138f",
			merklePath: "fecb7e0d0008022000b8963f069aac2bb5fbcdfadf7c0a87948a4b5a7b1d30d202935c459c0e33eb08210221b650f4bd8ce0c03597ecdacf48b8d2e012a282c0f8b016c96628ac7905aa81011100a7e4c917b2c0875eff9d3c184221bf489870084144c2f351c73ffa43d3fa11a20109006015bb213041656f26283becdb7282be7a9a6d7baa7aee045f8824ccf5c723850105002b08907dbf41d0e67d2547caba86111df0f806793c2eaeaa1c538f5b57f0a2900103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "31b03170446c66079216fb48c0f4ce6859d325e09a23578a6c7e45bc0d06b630",
			merklePath: "fecb7e0d0008022600153c6b46daf69399ca6baf156c5d4241939dc3455b2a1d3434d9e77abe3895e827021ac1a2d2011be6e91770dda23f9e9404945ec245a8534409292da20e1c0f91490112002d25c23ed69343c61e912d594076844bd2c60ba205c4325773a3f38ce4e35e9901080064ba40d8646a3cdbb873317ff85a981e4546b52e273f02ef843ff6e4cf5d61650105002b08907dbf41d0e67d2547caba86111df0f806793c2eaeaa1c538f5b57f0a2900103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "687fdfcb413ae2e151163d0ba0f64b407f8ec3440a43072f39c4d3f7e64a4787",
			merklePath: "fecb7e0d0008022d002c34028be6519f10a35f4bd9a2c9d112798e361a809d2f534ab1c56dc7ed7c0a2c0247f6e11f0103be1997b250b3b22ed18252e0dc0b588aad7c4411839830be8e0c01170039a4bcdfd825f870fe73652bf859aac119a80b5c4610d15e373a7e609a456c9b010a002b34a23129c9ea1233c3aea5439091195693f7fa7ad88de5edc88b9bd92eee32010400705b50e71669191f10502ef1c092b1ee30146f9ee43ab80d230f9533ff3f129a0103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "23236381b0503f83be347481b534f7c86b1d975ea8156c50823da691019ad2cb",
			merklePath: "fecb7e0d0008023300b199d4158a7ad9bd4eb49eeb433c22649ed72c723f9fafd4413827599e34803a320215bf4347fe9958826e9b6fc0bb2e292372c29065f0e510ca243d75317adfa2c001180067624a85078635a5737e89999e09ab524dcfe8bf9c40cab0e50bb96a0d62383b010d00c45d6601d965a46b5da957b61278396f3bcd8adbf91100eab299c56e8c8dcd6e0107000b865d50e9d261f269c7c18049ccf18957968961406fd90614aaf09ef8513e9d01020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "6822089119494f3bd7656281fd78cae631340ce8637568545faed32fd003708d",
			merklePath: "fecb7e0d0008023700f233839abdc3a4fdf4c354a4a9f74f62b12fb1e0b60beb7daa18a3ba0e425a123602f0869d4f33a07619a90057c1b9af6efc823248dc9d12053806f863e3602e3eda011a00b0141e269e41e140bc2b383f83144252001f73efd69d31498e339aa88abcd611010c00ad43c4dabf6ef080d151afb909d5a53c7b659b401b41c50e13ed7b4ab2ed4d680107000b865d50e9d261f269c7c18049ccf18957968961406fd90614aaf09ef8513e9d01020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "b8d6850ff429bcf940dbbf4d92021d4b9574f1bc484a8a51168d67f48241f231",
			merklePath: "fecb7e0d0008023e0003d34c90325207c9d2f97aced78ed3f43f14c60b5a5672f392a19715f69fde683f02c6eef165929146181a6816bdd4350a710f3e495fbaece86bc7af0efe9416d42a011e009c969f6d33466d1e7a3ebcceffd052fdcc91a4ec73c8ba5a3c2b5fd3a8a00dd8010e00ae408a1e7e2a50c82455598bf5584b39e1df59608a052ff03e6b9a7d0495588f0106002a8b6783d6184ddedd66b782a26725e272ce3f8840bfbbf4d65fa33f1a9f1ad801020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "ee09d7adcf261fb8b5497a3bf919825f919f08bbb0b6d777a6f0209796b02cd4",
			merklePath: "fecb7e0d0008024200e1f19fabe362b027f60ca0ca539a2f5aeab52eab7c8b9d9d54adad33f80333fe430287d7a1fe9108e0e2c3fea8aeb224adfb7c7a5ef45220019d7a61c0db0fa9ecb10120006d60998029ec663cc77484603efd0716d5a2cc33baf6bf33ef73056955337bd90111001f6bb660213452b7780436725814d94813eb6b52e385822b876599ec0192a056010900c25180cb837b1c74241bb26b02f188dc1422a9c077a5c9b9c6ad9c28eecb534e0105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "601921d2f7495c1dd096f23f38b4b299387e68d60d14d4c3ac3da6e27c3e4300",
			merklePath: "fecb7e0d00080249001eff5eaee310d8ac3247cb0d52cbb461e8420f3a48f378c376a5d96eae623b444802eafbdeb7c96f9c79e2e69bc07bca20cbdf105976bd3a0d8b1a4b738f0c5a7c800125001942844dd19e4648cd1942e2c7042a6200aaca668cabd0d04f397d7def9f41da0113003e9efc5a4bbb87c10d63f087cb799e0281ed900ac8d4d951ea5c938b99027afd0108000d285959d969da10d98b0ae7ab2ef6ebf71de6382eec4a70f2f8351907c0f5150105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "23b99d2fcc1ffd33ac9d369b4d68573c06e4afb44245a953db7388cbbe722d1e",
			merklePath: "fecb7e0d000802510058b9c9613fc7ee805a7f79206a93ec7371a995ce08e8875cf300d304af3c9a9a500293ddcdda57c18566e11f87495009839754aa837ee58a6630acd2227b8421fa22012900f32c5781fa141b57ec0e4c4f9eb45c18bce5685fcfe6ca6641f3736e9ae84842011500b2ad1617e367ac311209af46b981f09064fbc4b0ad183386ad5eedeb80cf7ab5010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "c26ad8679cb7cae897c99fe53b8e96253c87eb209e57465f6211307f1059bb40",
			merklePath: "fecb7e0d00080255003974c8bcb102d7baac525e3a5f9305bafea42b3b0eec56fc416ed7782da7204a54022256d29393f286725fe492d77c79b530b9a6a7e1623436e93744d454c9f335ba012b00be6c36fc1cbd20a011c4506f8a734a3082f9163a97fd3362bb6e281bca2fa8d5011400db8ef46ccd1ba67a7a66cb3869ec1805f0aca1a665c5440acbf22080a2f84194010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "4d6b678f068645520103c6254149b2ec62ad31593cb85eecbf82e5a09f6da67b",
			merklePath: "fecb7e0d0008025900e348f825b68fdc36cf7dafa6911ffdaf25687dae502cd39be7d61f5bc7649b415802be6ab9690e9488f430007629fbfa94162af8d8c7fa24a4b97b5eb2e88d68135c012d004da4440f82e2f96261f65b9c23d3d6f5dbeb2b94bbf11828d1953c0ee4cd5f7b01170093f8af668718997972594d254a7f3872e94cfc12c5a63be7cf8fb523afc1800d010a00ce78fc6a3ff6b03ee3ed3c1c59349afa18353cb69cc0fac7cc93a8dd8df42fd0010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "7c6a8081c06f3e319dc5e9bee86de0dbeb9db0e24929495a514b938f99a0fba2",
			merklePath: "fecb7e0d0008029300c2512ccad544f3132a946423eb0817132e131c84c4c6dfe84d3af05fd6f93bd19202ba91111907549ede8d95fe36f43b68269f661ba380785a9ff8ac811f11fdf6f10148008f0b3e0ed6e43b3bb6dc5812ae70531247b42d5611cbbe112c3b11e92a1059800125008318af79a5eaec2ac392b361dad40cc56de7fc4f8b0aa11c2224a4944ddcef94011300af5b9bd6628404a022b5c3a4ef8e641796663617d485523b44bd1da33548072901080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "2ad41694fe0eafc76be8ecba5f493e0f710a35d4bd16681a1846919265f1eec6",
			merklePath: "fecb7e0d00080299000c4f9d48bc38909744ba958c564907d99db04a0b444c081870d0003674857f6998023a80dd9b91349c9fb7c41b2379281ae8a3e6491641923c77a1b2e4c178c261af014d00e827c8635bde0c7691988a21a536dac279b9d09bcccb5b9c175710bdd387e3d8012700e7f4ee9d1c81908f8708c583b65726e8bf42be7c4980d10f81da0401a87d91bb0112007ef67cbfd4cd37679be1c49b234c7d9ebbab6449854dad3aba0e24605f1d241601080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "6e6bb4b23ccdf5a196b8c1172f678ca501145ad73dc316689fd25266c98f34ee",
			merklePath: "fecb7e0d0008029c00ea215cbc1a9218af80acdd014c5c5c3e649fe6f06d185381dbd21b172629590e9d02a9872b7360c26c0de25a22020b578d91a457c6bad7874323beea96d6f7763054014f0064339f3d26e03363fd2de523dc7fe9248c380c212230b3a8e74cb8023f9a91c2012600611ec412b4f6e9417734e978a17b62338f6f5e100726cd113380649d216ed0d80112007ef67cbfd4cd37679be1c49b234c7d9ebbab6449854dad3aba0e24605f1d241601080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "57607689b518a002d74f7ce62afb8c8b4179cebb20f38a66cf132e06863d87f4",
			merklePath: "fecb7e0d000802a200dfa92ef5b4defc86d6c9ea9f7e6dff2403dc728eefb046e12c4b53d16800b182a302eb10d50a72cae34d74a74db6edeaecd08a28a4862a3701a3aff8e85d22d3ddb0015000463dc1a3df8858d0e77832eba695e840ffd97b0eb48d5709519aacb744dc22c60129006f99522db6a78878e013d103891e6ee5013a8c970e455af1b0381bc30491bef9011500f971274b0ccd4b3ec62c4b21c8c3984f1ce55f306143da3acfb4428806e6a248010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "22fa21847b22d2ac30668ae57e83aa549783095049871fe16685c157dacddd93",
			merklePath: "fecb7e0d000802a8003a98a0fe3c7fd3bb8519e3bf1e405426adfbad98ef52d7cf55815791320df028a902c44824593b12f0828ed779a3ea6f8d5bdcf71ecc1b7537bd3bd7adac7110343e0155009ac4f6ed7e60d1cc68ff9abf0402cc9e3936eed9c6bfbfd270de4b2aa0ef1b19012b0080b17a6365be39783c0633c4d1831294509cc3ad1a725d504c2cf703a28969420114004d0833a489352699ccb68a8c426c6f24e4b907dc119ad2b8a0528ed885a7dd5b010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "4a20a72d78d76e41fc56ec0e3b2ba4feba05935f3a5e52acbad702b1bcc87439",
			merklePath: "fecb7e0d000802af008f851d1ef3461c68a5dd9d9c261431c0cbc9c49cef3e7b78fd7fd6e064e3b76cae028f13047da7809d813bb38008303155133847eadd46969b8bbd6386467b64aa6a01560022233a3241538260c93094f87498c9502d5fc950bf55d0bddf6af078f312dd7a012a004dc3e6a881507b58d1bae89181226e0ac74b2fe6b36efc259c7015df62a6a5350114004d0833a489352699ccb68a8c426c6f24e4b907dc119ad2b8a0528ed885a7dd5b010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "c528951581a47b8b875f1bc9c863a4fbd18051fcbb2750e45e524c5f6a93e548",
			merklePath: "fecb7e0d000802b20005e6bcafa6a9a594540dedc1cec622a888c67312bc64e63f9263c14804a3d0adb302f7349e0287a27f5e64fc679137fc8ff8b3ce5597d335d1b5e5df06e326c9e2c20158000f013dffff10f982d201b402b108faad4ea57f476041539e816474bad6157232012d000c66152e40c1fc29fa32c5c29ee61b37a324f219cbd8fb123817d920d9e606c6011700019c66fbd59b7813377e19d28acd935c1d580658dcfbc0ae9d7a13317b29b203010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "d964753f9b850d418cc7216b568b7d9691bb0b30369328a1b548e34cb8c23a81",
			merklePath: "fecb7e0d000802b80039d23c94f62c226214419c9edf67d060f23d7790fba3907ae48f95fdcc5c2d9fb90287474ae6f7d3c4392f07430a44c38e7f404bf6a00b3d1651e1e23a41cbdf7f68015d00136b66e87acd2c0ef587126911368403beca58787d784b2ad49ae5171c05d798012f00501dc278a7325ae98b0c27a5046725f5ca93b9d0e8a98949b6e2261327ef1ef501160090d25b33dfefc54116268def6a14ad1d57fd0eb33d656f182862ea0495a4f250010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "ce50ec49e9e8b571ed90b34338dba1c3e713c4a3bf54f593ef8e727a5464c9f4",
			merklePath: "fecb7e0d000802c3008d7003d02fd3ae5f54687563e80c3431e6ca78fd816265d73b4f491991082268c202d85cc5cbb29ad63b9285dcab4e61268853b03e6145a8e6df7ef414e65be0f787016000d505c051fa2a12e5c73a91cf567efe71d3660c127be04134058bf00f806a2493013100cadfbe0845c1e931e8ab0954e45f6120c71d963bf4d695fb9f7cbeef0dcdb3b10119007947528690e58942461c33193faec18b2b372bc848a9da9829ab946f1a50d795010d00f1ccb0f27a84a41e978e6acf0f8398bfad21a2290bd9b7e4543c5e3b274950a40107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "bd972512d40777ef3345a1682c7056bf149002f4646c2a40a0c910fe9139c7d7",
			merklePath: "fecb7e0d000802cb00569089b7c4c7299a09f1bafb8f1ae3762e1c135a9d96b1193d08106386ab47a5ca02067eae6324d953ae7a4a219832a581d93113ad60cc5b3372f1bc87696ae5f782016400e908302931617a1f772103b1d90740253600229bd1b6c29d06c918af29634d74013300b283333649ea77f2721caac90a0e9a605f6937534d7cc562954d5a2b1be43cf1011800243e574de078804478faa2fc8d0f2453857545ff3f875df7f92a234b2344d951010d00f1ccb0f27a84a41e978e6acf0f8398bfad21a2290bd9b7e4543c5e3b274950a40107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "49684d24297f2c56d998f169013c3bc36e348350725f1fedd21ed58c905c6eaf",
			merklePath: "fecb7e0d000802d50000433e7ce2a63dacc3d4140dd6687e3899b2b4383ff296d01d5c49f7d2211960d4021d88367c1795b6cb2f1d06ed6bd9236f4ae8a3f0333eaa39b64149d0cd4c6869016b0071ef7eede11b436428ee31025e369ae829ef61457ef436a559b0e2d4e16826620134007e418a484cabd45543029bbe0a219b0682de15bd662238251db56ba67be721c4011b00807de421eca8c9162eb6bccbe3dc58c50eb6e739621d8c372f0950a153231eb9010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "223cc19122d37c8a786c7b58839b94e80f3fac5a32e2da95582bf1f35264b454",
			merklePath: "fecb7e0d000802db002f49640eb87c68d90e88ee8dfca687997145a9b024b4afaa3ae5b305df312c11da02df75e702a14045aeb76c834ea6be4252fae5b2fe115125f41fbcc35651561a04016c009e3bb6a6446c1c256025993d0954d99c06ebeb80588177b9ca62c600c53fdb5b013700fe524238fd8a1315054acc81cbb82fd524fa60998cec4215ed7ef3edee25ee7f011a0065eda9cd45cbea9017d208b4b19495566730390c178dedaba817db963decacb1010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "f4c7dcaeb2c4e4d6713a80583a03a7d7975cd6976c5a803ce69502fd028b142b",
			merklePath: "fecb7e0d000802df00d7b6a1f91bb7b069900adfd01955db6cf83083f9c4ed5aec62a17cff675a5791de02c3a0b71901752567b3226b649dc988e077de745de9420c2e423e47c106f6de9b016e007b7baa9e72e04443831dec653e9315efe84090e4f9452031cca69c95ea7f6dc6013600ef31c0766f14bda2fac401f6addf31a6404515fd0eae897c803c92d996ab0423011a0065eda9cd45cbea9017d208b4b19495566730390c178dedaba817db963decacb1010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "42f055049462c30fee3c2d6159731826d4614a0028e06370a2058549356beb4b",
			merklePath: "fecb7e0d0008025b00e8d09d799e652ee06b6c92e6ab5ba83e3999d86210b6a6491acb577e0aa7a3ad5a0248e5936a5f4c525ee45027bbfc5180d1fba463c8c91b5f878b7ba481159528c5012c009519fc9971383b1489973cad796b0596fc7861d27bea15aff2be8e774017ceea01170093f8af668718997972594d254a7f3872e94cfc12c5a63be7cf8fb523afc1800d010a00ce78fc6a3ff6b03ee3ed3c1c59349afa18353cb69cc0fac7cc93a8dd8df42fd0010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "8cc3498c07cccbe336bc35c6471ee363b34f9437fa2bfbfb94b77d4609c52594",
			merklePath: "fecb7e0d0008025f007cd41a52493705c29b270fdfb27d8188a9028c9094c46a6879916f3b53f78c125e02f59cb7b1d1e33d94759b960dddaf9e6c5431216b2da05573c7225055b7cd5beb012e00ef671e52d8e6017c5be97c4b2f521e2f4733638d89048f8847905b4d4a805ba5011600afbfae2e2b2a34bf9eae914009c96a6bcbee6916f331ce52d5983edcc124d950010a00ce78fc6a3ff6b03ee3ed3c1c59349afa18353cb69cc0fac7cc93a8dd8df42fd0010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "9232872361126adb6401f30a7c165b418503affc7da41e81a8acce6cc015517f",
			merklePath: "fecb7e0d0008027000f1973d9e3492b6ebce9a4ee832b71f6a1d8a25ced41395366af5f2afba9e1a917102d7c73991fe10c9a0402a6c64f4029014bf56702c68a14533ef7707d4122597bd013900356f86d9eb31065cce2ddfc5c6dead1bfa8b4b26494411947e05bb923a6f26e2011d00e1ab21ac4968a983f36f0107edda345a4630a0511ffc33fe4c7073b18f9779dd010f00e339fc4fe0c19b8949ea362811d9a780a0d6e28a1b4fe754971437e4905c3a5e010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "08eb330e9c455c9302d2301d7b5a4b8a94870a7cdffacdfbb52bac9a063f96b8",
			merklePath: "fecb7e0d000802780019ece497a29993e70d8c2db0e4ffe41b85ba5577c18eb711e1da5f304efe76a97902f9361eca0b510687d5423c225f30645b497ccc2d643ec9eca304eec3e12f210d013d00a0a6023d4b48c397dc263508266be8b75b5f00ebe9956305c6a99166303dccc7011f00ef92776fbf694da29ec5fd8f9423a59e72dc3051148a9fee75319bab3975018c010e00800fbf16098dc0092875b4ed497b29c445bb84ff2dd24a1db24f4d48eb14e235010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "d049215258037715a622040cef3f834b86303e6c0095d08d66357ec4fa091338",
			merklePath: "fecb7e0d0008027c0075cfee3a8d32af24b51c77f42f725141fb454bc6aa091587847d9b96ed8f86cb7d025e5d1f62dbabeae9b6d5a05b2b34765cf591a5d330c54527acde7945ae30b601013f005d844d83fafea8dd6b51c7f64140c8444ac3c3122f986101dc58de3b1ecfdccd011e00ab323f5a7f849715b95c948e381ae28835d8e770c544ebac2f9ab4e03d51ab76010e00800fbf16098dc0092875b4ed497b29c445bb84ff2dd24a1db24f4d48eb14e235010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "409878632446e68a8e11e57964d0d9eb49a703ff786eb6cfc0f5f6d8736bf1e3",
			merklePath: "fecb7e0d000802820025ad0ad480573194bfa779f037cefd20c990488418e7b867d69abe107289162d83021e5c57da40abd5d0ccf396d51972b3ead21129304087ba3e39f41fa991d29d8e014000b9574ba7390b6b23bea7cdd9e177b69f243da6154956e726fcf30dd8c8c3f4e7012100d8117c8d26cb2813706a986d20ed46a783543b10d1f8c250b9945435734504e9011100b6a218a3f8a4ab9e60390aaa08543ffed64c033d3d24c43acb29b99a4d76aae801090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "3a80349e59273841d4af9f3f722cd79e64223c43eb9eb44ebdd97a8a15d499b1",
			merklePath: "fecb7e0d0008028d003804c02654bde43449a8332a0c937cfea7e3415b09e98a7e90ea1b2128ecbd718c028467927829aaa37a584fd86ead8857a3d97f51fc036c9b809365753fa6bfbd1a0147000f7aad7999f7bd292df4396e941312c901c46a4f5e17de928c1508ccb429725c012200900f719e9d10b2d498c5b9b4a4d3cddc2166ad9465dbd7f78010cc9f2622d0360110001526927b16c6d52f19e8e809cd0a1c5db2c7aa39fbcc9492dfd55c31dbf041b201090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "78260e5f5f45ed5cd55896f9a5dd0b3ee089e5452ffef48d6d271bc3596f939b",
			merklePath: "fecb7e0d0008020300e9aa8b893dd542babe39cf677dc62e3df0709f8efe71c1976f3ee4e921b29b090202bc0919a331d32caa671f4ae69abfa66ce67e849a118cb3f2ca82f112452db75401000085f1087c6567a40f90bfb16662e4021a5bd98c41e3202f917a43c5e4fd85415501010044354ba469cb20f1253474126fa7d958934163dbd28fb7aeeeccdfafe0af47af010100337868843989dedacfb1af353e0ebd18abb5d0d6a079d6222af2ebd731f9dee30101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "baa83f270907f38fdc008ea20917371d83f1c0e463d3cd7f1149ace259091e7f",
			merklePath: "fecb7e0d0008020b00c86dceb6011b7373619417bac459bb3f21ae123bbe3c3f957ec46f528ee538320a024d33e11eaa5f16acd0658ff9c9fef20c5d6fb3b1e039ad7a4ebdacb49277aaa6010400c552532a6079f184b8250e1badfd3c1975becd5b19b200e095376a9d2a89277901030074c77f1a2269b92cadc474b4c37b8b6c47ce58feb2bede50f1a206f32289e081010000a06f6d08de1b2c77f695c2f0d9647f803e6f24e9f42d8fb013a91a51dd887ecc0101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "c8c2e43dc559f3e38a0e2fbb37f8e1e4b7ef29df0632a775bdb75e80ddafd83f",
			merklePath: "fecb7e0d0008020f00583fc46aa870497e6933c216775ab908a9864effedb5a01aec83ebc40092c74c0e02f369b7bbdae9c69825346d829a4bf1f6a47fbb71045e8c0d7289923083b41b1c010600d8d68b290ec8ec1e3865d50da352c51fc13323c4a896e3f973ef06db3f7ec61901020074191d9882d39cd83ad39ff93ff5ad401e4427eb82581062bf9f3220bc076f96010000a06f6d08de1b2c77f695c2f0d9647f803e6f24e9f42d8fb013a91a51dd887ecc0101007eea0807cb778fd239abee1144fa87f5e711b2457f8b1970949adcbde261224e010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "7a19c9c4a95b0c15d64814d395e1981ed06f76129687968dcb0a9058608b8e16",
			merklePath: "fecb7e0d00080218007f5115c06cceaca8811ea47dfcaf0385415b167c0af30164db6a1261238732921902ddd7f72a3374e00143eaa5e15e51dc5419c54eb7a94d82f9980b98d9ad2d2271010d00ea144afe32c7abb54d82120a0ee9e48e93d105cf4284e597c7c6270e9192be390107004f99b8eaaee34ea659b6e78659aac43969511f09c91690110b2308cf2cc6b3bc01020083042726de64ea7ac92d003d018c35e05e12c25a7e79d0e391608d8bed53c2b10100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "4e4108ad498a6f425242237270a7b61588e29b8f2e538274e26e0148a169bcdd",
			merklePath: "fecb7e0d0008021f004ba255d7364c6e59a919a47504a5b59899b1264cc050f16711226ff1776773e71e02a08fc40f7394732c1927545bbf696c832adb70be9b3708bfd124515734e9e2af010e00dd8dd3f000fc1cb472a70fa3aef7157542dd466e9ff7f7f946b42d62e707be0901060021fc17fd706c9ae4d082f9156297beded60b3548b91bd2b884f3fd13bcd3d11f01020083042726de64ea7ac92d003d018c35e05e12c25a7e79d0e391608d8bed53c2b10100000730498528e95887a0222ac3d98b1fd5c4766a037128eff0f3af38174b0ea2e1010100f6895f3e77a4193c3aa7d4cf478fdfa8abe6c43c43bb28d6959084ec5447c20f010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "6cb7e364e0d67ffd787b3eef9cc4c9cbc03114269c9ddda5681c46f31e1d858f",
			merklePath: "fecb7e0d00080223008e8b821768a0d55b148360215c39b6a7807e9f3367a4127cfcc5de9051eb6b6522026ceac4ba02cf0149471a8e3a8d8141f9e707535531a358a8bc91f71452a89fea011000f37ea4a1b0286deff70c986574f77d57a2115966fe7ae4cdf2eafb61cfbdead70109006015bb213041656f26283becdb7282be7a9a6d7baa7aee045f8824ccf5c723850105002b08907dbf41d0e67d2547caba86111df0f806793c2eaeaa1c538f5b57f0a2900103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "bcd3dd4bff36c8d8de0ced66b15c43e1247d23af9ff28bf970040870c1ffd225",
			merklePath: "fecb7e0d0008022800688b95e887ce8460b4635e5181aa91812aff369f4baca73df166f489423f128729028a9371bc8d55e0f992e292c16fe6c30b9422c568f7b6137fbbbadef2683fa09a011500c5827a94481054c6322d3893c1c96a49d95addf027d543351657265970bc1b2a010b001fa937279508dd2d0866cb58e0599487d1eda25015004a6c4802560a87983d14010400705b50e71669191f10502ef1c092b1ee30146f9ee43ab80d230f9533ff3f129a0103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "a252aee671b44d2e29aa6ee2e9793522a775695bb069846db07daa162c06580b",
			merklePath: "fecb7e0d0008022f00700d9a5e875f1347350b7159b855992a54a3383e76878262a30b693ee115c0b72e02070f59a0e2aa3217e33bc9a9a6f3baa3cbc76d673bcd5fd02b06771e1bbd812601160009669cd23bec5ec8c5226c59082878b176ce57116bdf6f916daa2f7d16b7f987010a002b34a23129c9ea1233c3aea5439091195693f7fa7ad88de5edc88b9bd92eee32010400705b50e71669191f10502ef1c092b1ee30146f9ee43ab80d230f9533ff3f129a0103002d131434cae517e49d53242b578dfdd5fa82db2180588f164bddb487d0a20c7c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "fe91a9a7efcff63deda8aa80b48e164e710d234748a07b8c64a226ea75541daa",
			merklePath: "fecb7e0d000802320015bf4347fe9958826e9b6fc0bb2e292372c29065f0e510ca243d75317adfa2c03302b199d4158a7ad9bd4eb49eeb433c22649ed72c723f9fafd4413827599e34803a01180067624a85078635a5737e89999e09ab524dcfe8bf9c40cab0e50bb96a0d62383b010d00c45d6601d965a46b5da957b61278396f3bcd8adbf91100eab299c56e8c8dcd6e0107000b865d50e9d261f269c7c18049ccf18957968961406fd90614aaf09ef8513e9d01020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "e78a14bf220a180b131801b73d9c863ebdc880673ecd23067d8ccb89c075891b",
			merklePath: "fecb7e0d0008023b00c0aae0f25575fb1b6fec3d7c4a80f59eae83b72ecf20138b53126e67cb7e341e3a02e6d73a08df151af4763cd084b35db1195d405cf66ea6af0717332ad32586e91c011c0010cd9ade28003ec43db6cf540e649f7b055ea076d5cf734af512e80fc584929a010f00c86728279f7ca124718dfd1776935fdd62570ed6f301f025c0297aa5bae9404b0106002a8b6783d6184ddedd66b782a26725e272ce3f8840bfbbf4d65fa33f1a9f1ad801020093d4705058191cd7bb1fd6fb92809e3706d4417487c800c78e12da558fd0d31c0100008e2169df4aad591262f10bacf05d8d5007432f7a3469b1c0dff0e83c4af7732b010100291b058e87ed261f90963d5793d6bd26be747c49b46f09e10794444f841977a6010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "71b95cc8391cf493bb15ae5c9576fa7b573d4fc94d1356c09b552c36fcc05da9",
			merklePath: "fecb7e0d00080240006580d04a0f2c6027ee3dc04c24db0a036a016218c734cbefa2bf1c4946e534944102118a2a3d653106d5468db86989d84b5f85dfab6ff32b719ab012dce0286e3b32012100c66a1ef015d093d4e727096ae6df62afd139eac89475fb52fe2fe28b56c329bf0111001f6bb660213452b7780436725814d94813eb6b52e385822b876599ec0192a056010900c25180cb837b1c74241bb26b02f188dc1422a9c077a5c9b9c6ad9c28eecb534e0105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "bdb189576833617c49543cf070c21d0304039391e026e8ccaa472278ace7c7b0",
			merklePath: "fecb7e0d0008024400ee348fc96652d29f6816c33dd75a1401a58c672f17c1b896a1f5cd3cb2b46b6e45023cf91cba6e9a9e095bd834bb6ed6ed51380eb86306873cfa17e01ec87f772d8f012300dabd3482e26d3e733afb8e2ab4198c4167b7ec78d4dc68ec219624b0ed3e795a0110004484f7caeaa6f4a737698005d8fefed9d2ec0493a35e7688262c091ab9f139f9010900c25180cb837b1c74241bb26b02f188dc1422a9c077a5c9b9c6ad9c28eecb534e0105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "663915eac23f47cfec89807677ec6e0114fc651acdad60289cfd45a9b660e98f",
			merklePath: "fecb7e0d0008024800eafbdeb7c96f9c79e2e69bc07bca20cbdf105976bd3a0d8b1a4b738f0c5a7c8049021eff5eaee310d8ac3247cb0d52cbb461e8420f3a48f378c376a5d96eae623b440125001942844dd19e4648cd1942e2c7042a6200aaca668cabd0d04f397d7def9f41da0113003e9efc5a4bbb87c10d63f087cb799e0281ed900ac8d4d951ea5c938b99027afd0108000d285959d969da10d98b0ae7ab2ef6ebf71de6382eec4a70f2f8351907c0f5150105002b09ea6b61f8a6c0b5d56bcbee0485e85455afc1e9cae98b0ae4309796d27f190103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "9bdef606c1473e422e0c42e95d74de77e088c99d646b22b36725750119b7a0c3",
			merklePath: "fecb7e0d000802500093ddcdda57c18566e11f87495009839754aa837ee58a6630acd2227b8421fa22510258b9c9613fc7ee805a7f79206a93ec7371a995ce08e8875cf300d304af3c9a9a012900f32c5781fa141b57ec0e4c4f9eb45c18bce5685fcfe6ca6641f3736e9ae84842011500b2ad1617e367ac311209af46b981f09064fbc4b0ad183386ad5eedeb80cf7ab5010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "7e58a9785c28a9e687136a9fd52b96ec602557aebd7d10563949c4ad922172d8",
			merklePath: "fecb7e0d00080254002256d29393f286725fe492d77c79b530b9a6a7e1623436e93744d454c9f335ba55023974c8bcb102d7baac525e3a5f9305bafea42b3b0eec56fc416ed7782da7204a012b00be6c36fc1cbd20a011c4506f8a734a3082f9163a97fd3362bb6e281bca2fa8d5011400db8ef46ccd1ba67a7a66cb3869ec1805f0aca1a665c5440acbf22080a2f84194010b00d371b6e4d62f4341b4f15d6efe445c28de33a08d69dc8940e23fdf54ebf5ab72010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "da3e2e60e363f8063805129ddc483282fc6eafb9c15700a91976a0334f9d86f0",
			merklePath: "fecb7e0d0008028e004f2e9451cbb09dc700858f3758b9814a6265b4c994580176040075adca1e3eff8f029b936f59c31b276d8df4fe2f45e589e03e0bdda5f99658d55ced455f5f0e2678014600bb5935ea6465dcdf79b10e8be99e76411a94f4ad98c9c846ea93ac3d9dd07536012200900f719e9d10b2d498c5b9b4a4d3cddc2166ad9465dbd7f78010cc9f2622d0360110001526927b16c6d52f19e8e809cd0a1c5db2c7aa39fbcc9492dfd55c31dbf041b201090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "1e347ecb676e12538b1320cf2eb783ae9ef5804a7c3dec6f1bfb7555f2e0aac0",
			merklePath: "fecb7e0d0008029500ed8f557ab99607a3b46eee7d63830b29f475d1a2fdad43226ec94626dbb3cf489402519ad180d8c08d4719bed6549ef4f5ce97ef52364278d5347f2a3f9076146d65014b00496a19f69dac2e9b1564b2634e895628a64c661215c02cbf480c50fb5e23512c012400430960c35e8e5fc60ecf152603b769f61830a4b1a42b4cb292f47e55bcbfae3e011300af5b9bd6628404a022b5c3a4ef8e641796663617d485523b44bd1da33548072901080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "323b6e28e0dc12b09a712bf36fabdf855f4bd88969b88d46d50631653d2a8a11",
			merklePath: "fecb7e0d0008029b003fd8afdd805eb7bd75a73206df29efb7e4e1f837bb2f0e8ae3f359c53de4c2c89a0229cfd0c0bc05f0e3561e8ef71c7c5f3fd5e0189267b80fb01ec1a3c6bd231ebf014c008a7c073a6393bc50408928af30b631327e2c77d01f421dce772e7d6c2cd5ef1c012700e7f4ee9d1c81908f8708c583b65726e8bf42be7c4980d10f81da0401a87d91bb0112007ef67cbfd4cd37679be1c49b234c7d9ebbab6449854dad3aba0e24605f1d241601080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "8f2d777fc81ee017fa3c870663b80e3851edd66ebb34d85b099e9a6eba1cf93c",
			merklePath: "fecb7e0d0008029f00464ddf03467a82fa807ef4862f11cb0558c0f3ce1629a2390d8f73db6e80d0b69e022870f28f63868340fa4c0efa78a8ad2ac43c8523b5cc4935ae8fe6af8d8c6b47014e00e73c9066b7c6408f8485eb6da219e3a45144e9c12250e74dc8945879710b2823012600611ec412b4f6e9417734e978a17b62338f6f5e100726cd113380649d216ed0d80112007ef67cbfd4cd37679be1c49b234c7d9ebbab6449854dad3aba0e24605f1d241601080071f72f4944b92151a50ae1182e19847232e1f784f35ad97b96360cec1e412c33010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "80ef6c014ddc851f338e89809698ebd0bfa7874c6897f9e77a1a02cbb8fc4deb",
			merklePath: "fecb7e0d000802a700042f2ed9500d925d2cfb2f13f79c19dd3591ef92e5638457ff176387598b33efa602168e8b6058900acb8d96879612766fd01e98e195d31448d6150c5ba9c4c9197a0152009b48f41d8473dfa5a362aa5e480491005a910ff0ca6a93339032c9af3c781c5a012800fdc7244ee20db2d70a35b029d98fe130cf6934e5a9e9c0aa36093d072b799dc5011500f971274b0ccd4b3ec62c4b21c8c3984f1ce55f306143da3acfb4428806e6a248010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "9a9a3caf04d300f35c87e808ce95a97173ec936a20797f5a80eec73f61c9b958",
			merklePath: "fecb7e0d000802ab00ddbc69a148016ee27482532e8f9be28815b6a77072234252426f8a49ad08414eaa02ad0fa245c56c3f9a368b7625bb8503a198f33439929405fd76a9c699d48e9a0b0154007028f881cba42681ead83fd04dc9bb35dcef0c74b8fa55a2a6eb076e65c6931b012b0080b17a6365be39783c0633c4d1831294509cc3ad1a725d504c2cf703a28969420114004d0833a489352699ccb68a8c426c6f24e4b907dc119ad2b8a0528ed885a7dd5b010b00d197886e749699eb4f0382ac2d2607bd2c0ebd3ef424e87292018c30fa31cfed010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "5c13688de8b25e7bb9a424fac7d8f82a1694fafb29760030f488940e69b96abe",
			merklePath: "fecb7e0d000802b000d91b1737c2f562480f657322a5dfb525453f163ba28d441dec6ab4b84c6ad7e2b102615b1ff82d2ab14a8780dfb1ee1564c81d2b97e4403ea8b3846b6497b02c0e700159008338eda889ac080eb0f23e6869ba3b8eb25755f6fbb336d8ff69a850a0d9d811012d000c66152e40c1fc29fa32c5c29ee61b37a324f219cbd8fb123817d920d9e606c6011700019c66fbd59b7813377e19d28acd935c1d580658dcfbc0ae9d7a13317b29b203010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "91670f38139edf8f3813c930122da42f199364b8b59cddb67588ab962ce563b6",
			merklePath: "fecb7e0d000802b40030b6060dbc457e6c8a57239ae025d35968cef4c048fb169207666c447031b031b502a9457f554dae3c3a6a7e704b3530308117501d70e81aa8b5c52c2aa1a567c0a2015b00cab3adca669ee72024d40cd06756ae8ffa47cab64a1f6e5b0c9c556c817938bb012c000a36e47684819fcf8ec2840009337cc5235f4627f1cd194f064e725ada7404f1011700019c66fbd59b7813377e19d28acd935c1d580658dcfbc0ae9d7a13317b29b203010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "75144f5dd9be39c9df5eda639923b20495e2e3c326fd17c0eb20fdf24fa4bb29",
			merklePath: "fecb7e0d000802bb000b58062c16aa7db06d8469b05b6975a7223579e9e26eaa292e4db471e6ae52a2ba028b51fbb9452c8b46d0fe2fce26afecf5aae54ec4594c0caf71a9ca2517dca238015c0044db6afc80f5930c2652242a1511a24ac60650e4f66ddac72440df9eefba7abd012f00501dc278a7325ae98b0c27a5046725f5ca93b9d0e8a98949b6e2261327ef1ef501160090d25b33dfefc54116268def6a14ad1d57fd0eb33d656f182862ea0495a4f250010a00896dff8414e707331c71dcdf3ed5ec7100a9ab3fe9649027fc53d2fde7f7f85f010400d19849ccf0f4b2f4ea7e8dcf9a3512745b812baa0e453476784328c79b7ff5f1010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "40fbd46a116daa5924d86c83dd01ed2490ecda98af2d8f826161cfa135423e8a",
			merklePath: "fecb7e0d000802c4001f78e6c1107e672ed73b7e3d5c33c4a1b500e9f68ec3b1732669e1bafdb0d762c50240cddac125b166e4906c51b9f6da590179eba0417d09015c66267f748547e216016300ea6fc3e1ab5ccd4cb4a3a75c35aacc7900335b6e0622503ea565d8e456106eeb013000a7efe10a6a5705d20b75f903ad3d697f803226860b55201869b7ac979196ba0e0119007947528690e58942461c33193faec18b2b372bc848a9da9829ab946f1a50d795010d00f1ccb0f27a84a41e978e6acf0f8398bfad21a2290bd9b7e4543c5e3b274950a40107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "01b630ae4579deac2745c530d3a591f55c76342b5ba0d5b6e9eaabdb621f5d5e",
			merklePath: "fecb7e0d000802d7009a2d6ebfb1527f05b1072fce7d8fcdd9eefe6d9c3c2414e3839654b077ddfd47d6028fe960b6a945fd9c2860adcd1a65fc14016eec77768089eccf473fc2ea153966016a00ee31f845fe7378a3ef8bf34bf917e19b28b31454bfdc8158d58b1dc2b0b6ed1b0134007e418a484cabd45543029bbe0a219b0682de15bd662238251db56ba67be721c4011b00807de421eca8c9162eb6bccbe3dc58c50eb6e739621d8c372f0950a153231eb9010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "2d16897210be9ad667b8e718844890c920fdce37f079a7bf94315780d40aad25",
			merklePath: "fecb7e0d000802da00df75e702a14045aeb76c834ea6be4252fae5b2fe115125f41fbcc35651561a04db022f49640eb87c68d90e88ee8dfca687997145a9b024b4afaa3ae5b305df312c11016c009e3bb6a6446c1c256025993d0954d99c06ebeb80588177b9ca62c600c53fdb5b013700fe524238fd8a1315054acc81cbb82fd524fa60998cec4215ed7ef3edee25ee7f011a0065eda9cd45cbea9017d208b4b19495566730390c178dedaba817db963decacb1010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "ef349f5e78b71a5b6e77f478d2c2f207449c232784c5c0e3be5e3a0510fc40f5",
			merklePath: "fecb7e0d000802e10040bb59107f3011625f46579e20eb873c25968e3be59fc997e8cab79c67d86ac2e002ac17a939796fc7812102128fd76b444a654faaed6a35864a4f03ced1313be75401710005f585d9219304492fd69ae6bf8bfd54b1eea60dee783b3b6f5e34731316b0f901390036f01fc050e4b8f8b838ac42540dbbcf634cf62bce9ed0de14fdca6a821af99c011d01010f01010600ae56873a2ad6febbf8dd47a27e033c86bd4b51c1eeea678abfc2d4d734360d8c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "54b72d4512f182caf2b38c119a847ee66ca6bf9ae64a1f67aa2cd331a31909bc",
			merklePath: "fecb7e0d0008025a0048e5936a5f4c525ee45027bbfc5180d1fba463c8c91b5f878b7ba481159528c55b02e8d09d799e652ee06b6c92e6ab5ba83e3999d86210b6a6491acb577e0aa7a3ad012c009519fc9971383b1489973cad796b0596fc7861d27bea15aff2be8e774017ceea01170093f8af668718997972594d254a7f3872e94cfc12c5a63be7cf8fb523afc1800d010a00ce78fc6a3ff6b03ee3ed3c1c59349afa18353cb69cc0fac7cc93a8dd8df42fd0010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "b2cfd3040266eff6f8b2c68b43b644f1c0f7e40ebb88cf7ab8c99750f9a16a02",
			merklePath: "fecb7e0d0008026800e8bed717fa08cc1d6f25b0e2bf65c912a3a1a417c3e80a9eb5859a532b0f981d6902f4c964547a728eef93f554bfa3c413e7c3a1db3843b390ed71b5e8e949ec50ce0135009e34540d6269a8e3d54a3ce19a6525eff15e702b496ef80389c2530bdc7e5277011b0015e5786594f50f15b71492ce8cb7fd53690afb366af8d847442b7039cea1b3a4010c00735fb0f91d1cd77f66bbc8af11d89331a5dd91e3d954c7265677fed40a0f472b010700fc05e3aee9e1fdab595dac8a0a0f4a77c5f8ba8bb6ba212b84702869236c91be010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "a3a317262618a42a19d0a5d4e4bdf953b7128bfc39fabc82bc7ddac7bd7c127a",
			merklePath: "fecb7e0d000802720017bd15ca98cd0d1a6eba42b88542184cf3901a2ae29529ad6f60094f9abe37b1730227174b37d46673c4c1332111581f9fab1d4ee3bd5f5803f4141a125d9c94919001380028d7f7cad90072a603323733a215ad27f177ddd1560269f233dbb55e88a53780011d00e1ab21ac4968a983f36f0107edda345a4630a0511ffc33fe4c7073b18f9779dd010f00e339fc4fe0c19b8949ea362811d9a780a0d6e28a1b4fe754971437e4905c3a5e010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "81aa0579ac2866c916b0f8c082a212e0d2b848cfdaec9735c0e08cbdf450b621",
			merklePath: "fecb7e0d0008027b00af6e5c908cd51ed2ed1f5f725083346ec33b3c0169f198d9562c7f29244d68497a0258b022361e8c1255cc3c5c55c12c48f67536810219319f160dd46a98cea53cae013c00195335279d8c37ac28f508b163cb9c3b574f7c0a984d7bd457075555bd3d1e97011f00ef92776fbf694da29ec5fd8f9423a59e72dc3051148a9fee75319bab3975018c010e00800fbf16098dc0092875b4ed497b29c445bb84ff2dd24a1db24f4d48eb14e235010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "49910f1c0ea22d29094453a845c25e9404949e3fa2dd7017e9e61b01d2a2c11a",
			merklePath: "fecb7e0d000802810054b46452f3f12b5895dae2325aac3f0fe8949b83587b6c788a7cd32291c13c2280024e30a9e7073a07d1717a4215db0b4406559fe191b18445f94a5bab5c45431ce6014100bbb7a1df989298b83eb40d2077326947f36605641a6f59f8962fae0c92329491012100d8117c8d26cb2813706a986d20ed46a783543b10d1f8c250b9945435734504e9011100b6a218a3f8a4ab9e60390aaa08543ffed64c033d3d24c43acb29b99a4d76aae801090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "0c8ebe30988311447cad8a580bdce05282d12eb2b350b29719be03011fe1f647",
			merklePath: "fecb7e0d00080284004e8d0759b035fc6c48781b28ec28108f2b87f874b35e21b2bdd7df61e8d3bb6c85022b148b02fd0295e63c805a6c97d65c97d7a7033a58803a71d6e4c4b2aedcc7f4014300b79a21940c58b92a8cfe4b88213913444f60821b4773f35805deede016fbaedd012000f60ea7c86f97f4116ed04eb610ac521951d1e3bbf23d9e0e4d0b802b6a728677011100b6a218a3f8a4ab9e60390aaa08543ffed64c033d3d24c43acb29b99a4d76aae801090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "cca08fbef8d44feaae521bfa2bedab1f3a490b27088b1387c85197beed0cdadf",
			merklePath: "fecb7e0d0008028c008467927829aaa37a584fd86ead8857a3d97f51fc036c9b809365753fa6bfbd1a8d023804c02654bde43449a8332a0c937cfea7e3415b09e98a7e90ea1b2128ecbd710147000f7aad7999f7bd292df4396e941312c901c46a4f5e17de928c1508ccb429725c012200900f719e9d10b2d498c5b9b4a4d3cddc2166ad9465dbd7f78010cc9f2622d0360110001526927b16c6d52f19e8e809cd0a1c5db2c7aa39fbcc9492dfd55c31dbf041b201090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "e9cb5e02b7edd14e0a87e928a59347de1a1561da5cf66152deb99df0aa84897c",
			merklePath: "fecb7e0d000802c90053834928caf87532b213fa045480c3b48dc39930c5d6a19fe5efdb0186856ff6c802e16c32148b8796bfb2c5045961216eba2397d598bd4c7154218e0f7fb9699013016500d1145a86099aa2d63f5531d87b01d84659fa406d759ccb1f5459d885ad31a6c2013300b283333649ea77f2721caac90a0e9a605f6937534d7cc562954d5a2b1be43cf1011800243e574de078804478faa2fc8d0f2453857545ff3f875df7f92a234b2344d951010d00f1ccb0f27a84a41e978e6acf0f8398bfad21a2290bd9b7e4543c5e3b274950a40107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "a976fe4e305fdae111b78ec17755ba851be4ffe4b02d8c0de79399a297e4ec19",
			merklePath: "fecb7e0d000802d000d42cb0969720f0a677d7b6b0bb089f915f8219f93b7a49b5b81f26cfadd709eed1021fbde991fdf7193cc51afb732ae5afb39330f70d63b3e99cc69dd8e03c6bfb3b016900382852db8f1213004819b6b6b175061319fa0975895d977e222cdc59509da5160135003c2bae3ab85c7cbc318457363404ac85e0e5682defca11f60aa1d602e46d4158011b00807de421eca8c9162eb6bccbe3dc58c50eb6e739621d8c372f0950a153231eb9010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "662fb26006b96a9b5e3a64a48210019c60bddb47b824ea0ca61cf0b131624f13",
			merklePath: "fecb7e0d000802d900a6e1b8578b8a94204c0fe6fa9a965d13099496b24da9ad7f03a80fe9d20f1b5bd8021d4594198a45b135923d6c10f7fd0a7d89fbc22e21329018067aa61c113b66a8016d0054cdd92d96a8fcf9ca655b590945a999e9e9e8701231c19d9d9d7894d9689bda013700fe524238fd8a1315054acc81cbb82fd524fa60998cec4215ed7ef3edee25ee7f011a0065eda9cd45cbea9017d208b4b19495566730390c178dedaba817db963decacb1010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "6cbbd3e861dfd7bdb2215eb374f8872b8f1028ec281b78486cfc35b059078d4e",
			merklePath: "fecb7e0d000802dc00d4815169584f94412d59a2f71e0015badb4952efdc8adddb4268331d1084f1e5dd021e2d72becb8873db53a94542b4afe4063c57684d9b369dac33fd1fcc2f9db923016f00976a331a69a56d7ca4a93b1e5488a21b061e8ad43fd18615c6038b6ce3ae02b0013600ef31c0766f14bda2fac401f6addf31a6404515fd0eae897c803c92d996ab0423011a0065eda9cd45cbea9017d208b4b19495566730390c178dedaba817db963decacb1010c0075f8850493221941d3e18cd6384555a25bdc0040b1b09d187d6e9a84a74567060107006cfa14aa43d4e0a032d7dce82272505ba41f31e755bc058303bdb7f219d9977c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "6b8e0e1ced9bfc6a5e1136acaf77d01094399482af9bfb47c991a19254133c0e",
			merklePath: "fecb7e0d000802e200d8722192adc4493956107dbdae572560ec962bd59f6a1387e6a9285c78a9587ee30211b83f49b60a1bc4a8259d2bee1e26f6083cac75be4187c3c7fa476657f7814d0170004638b457be498ea8bf2cb2f40718c0af355c777d0e6c5af88ea0f8cf1bf984df01390036f01fc050e4b8f8b838ac42540dbbcf634cf62bce9ed0de14fdca6a821af99c011d01010f01010600ae56873a2ad6febbf8dd47a27e033c86bd4b51c1eeea678abfc2d4d734360d8c010200983537772b32227402b1ae500db341d9360b5848584793f10021d4141faf5f44010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "f3d29a4ddf1a0ea3b257cd12f996fa43b47296f91da221a99def8f13a79c64aa",
			merklePath: "fecb7e0d0008025c00b663e52c96ab8875b6dd9cb5b86493192fa42d1230c913388fdf9e13380f67915d02e4f198353e6cdd5f178bc70c55f0d08a5712cc1a80f68f2312a6e7ca6365862b012f0005a09772ce3d94e9e640695357a84c9508dfb91a82ec4e9a0bcbd64881d2e75c011600afbfae2e2b2a34bf9eae914009c96a6bcbee6916f331ce52d5983edcc124d950010a00ce78fc6a3ff6b03ee3ed3c1c59349afa18353cb69cc0fac7cc93a8dd8df42fd0010400cd0a28f725e5227e819d775a11f746f17fc9d4f8e46ea4487ee14d296aa1431d0103003ee979af6676e9d6d06526806c0e16d76bc4ea4879c102dfee20924c3aba6a35010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "e444cdd301c4e8b75c9abae69ee36718f0556315e19c8838301edbb71c9af9a2",
			merklePath: "fecb7e0d0008027100d7c73991fe10c9a0402a6c64f4029014bf56702c68a14533ef7707d4122597bd7002f1973d9e3492b6ebce9a4ee832b71f6a1d8a25ced41395366af5f2afba9e1a91013900356f86d9eb31065cce2ddfc5c6dead1bfa8b4b26494411947e05bb923a6f26e2011d00e1ab21ac4968a983f36f0107edda345a4630a0511ffc33fe4c7073b18f9779dd010f00e339fc4fe0c19b8949ea362811d9a780a0d6e28a1b4fe754971437e4905c3a5e010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "3bfa975df7b16f282f3de15f01bb248212cb9d2cf1acf1016d6bba6ecc15f793",
			merklePath: "fecb7e0d0008027400dec701a90cd8426345d6e70d1d2baf6972982272fe70c003af885f4090b19ee4750232a094efb49e3f4a11bc1f5ec9aadd526fa39d08f2612e552f756d0eefb23901013b00c4ef568105b8c0871aff97cce8d4ebe70a9e1c79c68cd33c085454df930d9400011c0024f6ae12db9729ae37d5082f9b585e99f1d88a091d9efad507e45f272605ff96010f00e339fc4fe0c19b8949ea362811d9a780a0d6e28a1b4fe754971437e4905c3a5e010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "656beb5190dec5fc7c12a467339f7e80a7b6395c216083145bd5a06817828b8e",
			merklePath: "fecb7e0d0008027d005e5d1f62dbabeae9b6d5a05b2b34765cf591a5d330c54527acde7945ae30b6017c0275cfee3a8d32af24b51c77f42f725141fb454bc6aa091587847d9b96ed8f86cb013f005d844d83fafea8dd6b51c7f64140c8444ac3c3122f986101dc58de3b1ecfdccd011e00ab323f5a7f849715b95c948e381ae28835d8e770c544ebac2f9ab4e03d51ab76010e00800fbf16098dc0092875b4ed497b29c445bb84ff2dd24a1db24f4d48eb14e235010600083801634b8f55f668dd424bac68d3d1a325cebcf7d2ce4078f6c58ca19baab5010200e6744e4dd20cb9b388e677208f6fae83546608def9f854297b39da41f4817891010000492a49b6a79b75b5aea4b903f6ba8d3b230052e53040fe4c4a58413728d32e03010100b697a87879ca39e791bd824c9678da3628215aa9b50c7825ebc7404689a2e529",
		},
		{
			txID:       "87123f4289f466f13da7ac4b9f36ff2a8191aa81515e63b46084ce87e8958b68",
			merklePath: "fecb7e0d00080280004e30a9e7073a07d1717a4215db0b4406559fe191b18445f94a5bab5c45431ce6810254b46452f3f12b5895dae2325aac3f0fe8949b83587b6c788a7cd32291c13c22014100bbb7a1df989298b83eb40d2077326947f36605641a6f59f8962fae0c92329491012100d8117c8d26cb2813706a986d20ed46a783543b10d1f8c250b9945435734504e9011100b6a218a3f8a4ab9e60390aaa08543ffed64c033d3d24c43acb29b99a4d76aae801090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "c0a2df7a31753d24ca10e5f06590c27223292ebbc06f9b6e825899fe4743bf15",
			merklePath: "fecb7e0d0008028a000e3c135492a191c947fb9baf8294399410d077afac36115e6afc9bed1c0e8e6b8b024b1a49aeedf45cef024dced18262124d196f6609d3e93245e2f3b3ffa5ea8ac6014400f49046445137224b66ef789d247b308870bb980ffb7a5406ea828466bf106e41012300a069f81e94877263116b39b919be3f0f89c22acaabb31becf6f6c46998ccb8ce0110001526927b16c6d52f19e8e809cd0a1c5db2c7aa39fbcc9492dfd55c31dbf041b201090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
		{
			txID:       "365ac2080e3deadd057a663343a242879c1706b3cdc94e4bfada6e96e5803b91",
			merklePath: "fecb7e0d0008028f009b936f59c31b276d8df4fe2f45e589e03e0bdda5f99658d55ced455f5f0e26788e024f2e9451cbb09dc700858f3758b9814a6265b4c994580176040075adca1e3eff014600bb5935ea6465dcdf79b10e8be99e76411a94f4ad98c9c846ea93ac3d9dd07536012200900f719e9d10b2d498c5b9b4a4d3cddc2166ad9465dbd7f78010cc9f2622d0360110001526927b16c6d52f19e8e809cd0a1c5db2c7aa39fbcc9492dfd55c31dbf041b201090018939a4548f9d4d95f2c2a17c3f8c7f41ee19401a4a9b01c9ca2838c6f5fa717010500996fbd524c3f8ebc762317eafbf9806b5eea013176ca90d6fa4a6d8ab7c014e3010300b47ee6b676e94060b7b776e9e1857caa7a36ae511708bd3365b438f16422cf6c010000eb92cb22704a94e78966f7256df5ffbcd63de41d4a092ae0a4cc507576caba79",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			path, err := sdkTx.NewMerklePathFromHex(tc.merklePath)
			require.NoError(t, err)

			_, err = path.ComputeRootHex(&tc.txID)
			require.NoError(t, err)
		})
	}
}
