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
		name        string
		registerErr error

		expectedRegisterTxsCalls int
	}{
		{
			name: "success",

			expectedRegisterTxsCalls: 2,
		},
		{
			name:        "error",
			registerErr: errors.New("failed to register"),

			expectedRegisterTxsCalls: 2,
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
				GetBlockTransactionsHashesFunc: func(_ context.Context, _ []byte) ([]*chainhash.Hash, error) {
					return nil, nil
				},
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte, _ bool) ([]store.BlockTransaction, error) {
					return nil, nil
				},
			}

			txChan := make(chan []byte, 10)

			txChan <- testdata.TX1Hash[:]
			txChan <- testdata.TX2Hash[:]
			txChan <- testdata.TX3Hash[:]
			txChan <- testdata.TX4Hash[:]

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			// when
			sut, err := blocktx.NewProcessor(
				logger,
				storeMock,
				nil,
				nil,
				blocktx.WithRegisterTxsInterval(time.Millisecond*20),
				blocktx.WithRegisterTxsChan(txChan),
				blocktx.WithRegisterTxsBatchSize(3),
			)
			require.NoError(t, err)

			sut.StartProcessRegisterTxs()

			time.Sleep(120 * time.Millisecond)
			sut.Shutdown()

			// then
			require.Equal(t, tc.expectedRegisterTxsCalls, len(storeMock.RegisterTransactionsCalls()))
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
			txID:       "d7ce87a63fabea133786f3f32bd5e57be2ce48fcd07dfc5e0fe3eddcdee26941",
			merklePath: "fe6c7c0d000a02fd4403007a90e2ede094bcda4fc627829774c469487ad5c540247ffeba2174a975489c20fd4503024169e2dedcede30f5efc7dd0fc48cee27be5d52bf3f3863713eaab3fa687ced701fda301009c0a942c48aa3ec8253da601e82855583fc740cf275fdefeb8346e3bc76f77d201d00004e91e9c1c1958ced1b69393943c939728042fcf8acec4f3f409cb5b5c64f06d016900d14bc03cf9ff45c13a58457bafd741580d8d2ce48c4618a30e456d73412e92b6013500aa6e35f938dba8d61f4e1dde9c0fa11e168f75b548ffc1be721cfa83d2ba6d37011b00f3c27e7a3bf5964dae016db606f47b381035bc8206614e52137066f9abcd8da5010c00447080a4fff9f7bce604c91a5d9786f2903529f0b5d085c6a4205d7c44d212a70107004a17c0eaf7d9a4df40d704aaaddbee0eb5697215b56d579b328690e83a3b918e010200d5ef9a424802d1a20029f4eb7ca1ebba0083d8e52041cde947523724dc5c9ef40100006255729c72c029fb9871324bd77e0102c8355bd271f5359ac88de73d1c60040a",
		},
		{
			txID:       "b80d96de0138f87741846789dd8dd42524060e6e1423270039c2ddec1fa3b6bb",
			merklePath: "fe6f7c0d000b02fd10060018b099af92fd1e37332049d84a22245dcbdb0de9f0f8127d9d0c13fbf1d79231fd110602bbb6a31fecddc239002723146e0e062425d48ddd8967844177f83801de960db801fd090300d1f62a586a58003d6221b0fdf22d9cf38b3efc92cb1493bcff70fdc0d1b99c0901fd8501001091197b96c6a066a83bd8a74bda96fc9bd86dcff0c2268af4fb9c1fb3b139df01c30001c10f90a1c15775b72775546b2417c44564417f5a0d8e48ada2fc7590f72fb501600011406671a8f9260e123e00d0f8930d67a67bf32e59d3f88202c94563be3c68dd0131005175f92cd7b66d5520b9dca31645d9dee889856c43467bc419a89ca56220c54b0119005e032aa9c36219c8029af7fec7c7d2a838bee91f0967c302ab22858b49ce150a010d00c77a409589e6b8d799ed7adcdd54a5232b46487e15b0b7fd0373d67b6af818d40107002e7df3259bf10716525adcf75e9b7432e0df4c513b866b9e2bebd07dfd9cb91b01020043f73f6a63b55c32109dc437944b8534b3f53954fffbe748606ec22551c95dbf010000c5a623cc0ffa573f391c7092b1b1564603d82da42c86effd3a8ea489fbd33357",
		},
		{
			txID:       "48a9aceb354297cbb8ecb365f39586d1cd89c50d00d3a67a33677ef769f499ed",
			merklePath: "fe707c0d000b02fd9d0100ece880078ffe7326d25daf12bd36d8e49941672caec9a5280eaf85ded7a29707fd9c0102ed99f469f77e67337aa6d3000dc589cdd18695f365b3ecb8cb974235ebaca94801cf00533d2d9339d2c311883d5e98b988c99152394627844aecf9289d2ae93eb57eb1016600f53ff37b64a72ddc427a68e30bcb7e8d2a0e95fbd33ad900f5a60c73643dab58013200c5b6e9f4db354795723bcdd1293cfdaae97cf402da7ed76c840449082e761e790118009e1e1c6a5ff8f4c9cc245a13eeee7a927fec6472346df5d3616ed03d6fa664dc010d00b6253ea130cf34707f7ac3c3aca9f531b3e9fdbd8a6e351ee27036fcaa220b75010700b4509ce1a58e1bb9047ee5c355b0d882e74d4530c73872045f7798d4245ce68c0102006732baae87d6bf45b86d2811139fb27e0ccfc243c07f8a64d561232b74091a73010000cdb12b30efd3f6a06274f2aaf5f0da101ec7d0d82e296ff02fd5e3195fe6d514010100098a909ab7c63c674d58aa2537ae97d74bfa2d2ea7df3dbcaf0b59dad94265c901010021a05ad1e5cc785f56af3e9bcbd234a0320d5d0ebecfe8cf0deaa535d3d9a68d",
		},
		{
			txID:       "9ce54cd3f2c09baf840621283bbbfa266f2de1a79a743677915a54b3a3588a0e",
			merklePath: "fe767c0d000b026500a5f882966a26a63b9c0ee3701941bdae2a204dab7e05c7739582a747ad7c95a464020e8a58a3b3545a917736749aa7e12d6f26fabb3b28210684af9bc0f2d34ce59c013300a8cca5a323fa8f8d5f682c478cb8099f97b5f5ad0a9b05ff3005a4d372370384011800d9bfac16032cb718f5132017f6fa6028bdd5f4583c4696ad85e4758e5f0338c2010d001fe4169c3c41cc07d5ee349086c6f87200a0a416b2b77e4e2f7b86866a09af580107003aec83e1ccbfeb2a0561eca33773cca30551b4b15765a671eae933eddb8f026201020039f2cebe413c58237e9e3e8fa70c8edc42e8b6dacfbb87b93e14fe2c1362a169010000e5c375956fbd316a970d9784153efca18984a6535d80987244d2246220733e23010100746c671540530a543181c03f2a8d2690908229919523f97cc080ac3cb0758c4001010021de804398ee9b922918780b2d0456892d60281d309b26a95e9853034bf4177b010100f5a50b2c0b9cd5572dc0a231773f9095cecb227559c8485ae14bf87ea9fa81c6010100373eac58d47d07951b3390016d50029df212790ad2f4018f6745762371b44b9d",
		},
		{
			name:       "4",
			txID:       "e02a43a9dbb285d4946ad640b39822dbe91442e84de55769e53a938107814205",
			merklePath: "fe737c0d000e02fd0334000ad2e88b78bcb8d54600ccf983f68a775e88af526ac75c08982ac8d06a23446cfd02340264b58e9f5b1c48038d86f8050e240053ea2b21e49da23f3892355987ddc2eb7301fd001a00e74f7280e5240f50812d278f3c78409e6b30538893eac18d01951dea3804592f01fd010d0089bde9324fc5a9ef7a2296e658a5cec949935cffa0e8c8c5375895a84331a90e01fd8106007f0d1a9453df73f803e97827431676afbf57f50cc9138b6d45b6aba29a21355901fd41030086adbcdecefa570d63803ffbe91318cc3b2fe2863e1a053cb11a57f3146d977a01fda1010052d8d725ae4ea51918aca6fa981f1da71aef9d11427e9af68860e80458386db301d1002d3092e308c9c3547b3cc407775cbbe605afb00bf9b6b895c49bb8ebacf4092d016900915bf704908421fcba19d19c73b791ddbcba489d6069cdb4fbe5aba714f309aa01350086fbb327fb4bfb001c80b9fc9cce94f52f630cb194ef4f216e740ece7eee4f50011b01010c00ded6533343a346f6d83d4eb85c2d0c3ebf714b0912ac8acff2e1d1c4d6d5e6a8010701010200ef8a9d965919a1c1314d0c70e2684bf3d2504142d2e3b3ceede952c8a71f4d2e010000ec76222d6c569bd2e87daa2805a9a3a47125ac54be0e984b8b7b7055e1664402",
		},
		{
			txID:       "998a686855c0560eb3529a864d3c62aba7d4a614e50b32211abfbb7e2b20f89f",
			merklePath: "fe707c0d000b02fde60500a176643d790fcaf7b821cb6a0784d6a86899b3bed4ef371a1d6c63d5bc32593bfde705029ff8202b7ebbbf1a21320be514a6d4a7ab623c4d869a52b30e56c05568688a9901fdf2020026d5237030c03792a47b22dff34f9735f781e8550a22084c101b22b3d3e82f6201fd780100121e274bfa47a30ea573418712e7da3af2abe36bc2eceb88fad6a0198ea69dec01bd00d697b503a670288165426a1967b9e36eef55216d4ac73e567e728c5c0d08959e015f0073c5523076d8951f0c7f44904dccf4e7f33758ccb4259609355fd38fa4efbc82012e006287fd23656d2b8a00774afb2dfe911132b752d8b0c5fffc8e938e34d7f490140116001fddf90ed221b9f1f47bb87781d9343c2e74b747ebac37c2a32673ceb1c9a23b010a00bbbacceb48915a125cb48bec517f5b31a9d9e75744df14282e97a5a706123268010400e3ea99748b425826bfaebd7711176fc10a791a789ea008b8708b133794a718bd0103005b5edeab6c67734e6f8e6a8d9c154d852d2e2f5bcc89150961e22b34ba334fde010000d2b9ec57b200941e909eaaccd66c99be452e058b92259d9adb54d05b9dc1c119",
		},
		{
			txID:       "b9072a57f07099e79b8713f94b7dab682954913c634cd5b9e35af5bcd383527d",
			merklePath: "fe717c0d000b02110017d4eb8b3028faf5709f2bb93f66712df008bd82fbd4c5d1406413084e073f6c10027d5283d3bcf55ae3b9d54c633c91542968ab7d4bf913879be79970f0572a07b9010900d40968f8ebf3869e8f783e6833b12d2221f7634101d540e25ee193a5cd609ee5010500efdb95888ad3cd0ed67e9361126909de33efabac221a334e3964a483ab326d9a0103000b6959ce2a6c4dd1f15caddd8187bb7fdae8b03dea132d759a2880712ccf1950010000ca95aac2f6b352e266d56437f039770cf0b302206a33642d403371a7a65aaf4901010094370e6e947d9ce3b52c16cd4f85437c4b36b92d28fc2f8ad37fa8eade57e850010100620766a36702f9dd24a05152c010d9b35d3caff5ab56da157057434343359cee0101009132c126c8fe6d33a9dcf83f3301572f0dbfa22f08b0454b9bde04b4504c839d01010053de106cd1524a7f52a45ee2d3c88f275f15b7ecbecbca113b21988d7bc98d1201010019f7d07a26734ffd13f77c59c92f51535047d6be1186e70819833b6b1f4214930101002868897692645526b0d27b32eb4d332af6203e622a5842a58b317ab660157586",
		},
		{
			txID:       "15b932fba18e9541b69205954a9ee1f00875ea2bdf00dc8d1d22cdb761457383",
			merklePath: "fe737c0d000e02fd92030047814833d5a3c9159b8f994ad188aaae06e64a629a7371f521bcac22ab9c3497fd9303022953d6a13ceb68c85a04a11dca67ea13001630e9d72f58959ffabf1a70771ee101fdc801007cc611f2199c7aba6684e35120b52954a326c3d0652281632b69fc90950320dd01e500b56a16c709fdb24bd56f88ddb093efb5c8580fce8b7554fcbd69d4da3882aadf017300791a7587c52f6a6c5bd0f21635640d5204caa074f0a9d09d554db884dd2367c0013800504c26dcc7800b6ae21f122754c6246d0725e141f5d345015294c20909e9c451011d00a8c766d0d05889952630a4085ede93f0e98f34778f65670b8e28094866bdb924010f0033bc1b958449a476ac56426ecfd50a51f317718cd7dbdee4b61c112f458ae666010600d552d6ac02c28a8c9ec4d73151be0ab79e4e13c2a7efa31ce3c0dd75039c1ee401020035606e57ff80a8fb70280bc8b5b31e4f39ec94e68e8ba5eabe221dad5280df3801000060b8cc25871f3173090352c3a0e4529e91275253b1750113dae6441d2b381bc7010100df48a9dd6546a54be835993d6a7c30543ae5360bef76262bb078236d3f0225d5010100e04f384715b49643de713b4519565b288e6dc0244647cff030c7ed44136742120101008d618a2dd5929cd2a2a130a2d88d8bc8404fa7cefaccab366256112a9b8d3add0101005a37a1ab63894d1741be55cad1e9a3c74d2996f2d97a9812437d213de1b6190c",
		},
		{
			txID:       "dbee7d0f6eb19263b78bebc75fad0d53cb50f8ecd9a6d5f3b1f5e956c3be90cc",
			merklePath: "fe707c0d000b02fd1a04009d6b413ea34a73e8ee49c9efb073f68893d71a5efd8a448f3eacedd495385c6bfd1b0402cc90bec356e9f5b1f3d5a6d9ecf850cb530dad5fc7eb8bb76392b16e0f7deedb01fd0c020015dabf89c3547ee245e5b89a368719b76d150586c6a2cea059abc45cb15d67b801fd070100af5e575642a65e5840dddfe3bbcd2ba23f886dd12989ea3a1a4c1b2dc85a7195018200747673502f6b18c1c7d4185314617c8bbb35f30c893e7e1fb32b30fe65785e840140002b6de1e06d1f80a9915a03ee3d562815ed0597935b10ac8e626cd2d46e389335012100c6973d3ee073c2b92bfc33c5c3e8fbd7d60959245214f0eeada56247f997b2410111003230b46af2ab8203aed950f583f820fd75494cad8c7f335b1942cc90c20b87220109000ae5a40fe75df174dc39e99c55b4720c022595b3c4aee3dc8514801412408c2c010500b73818a995d825c4c16a14bc30570388abe2975d4967e00fe7779bd41c8145650103005b5edeab6c67734e6f8e6a8d9c154d852d2e2f5bcc89150961e22b34ba334fde010000d2b9ec57b200941e909eaaccd66c99be452e058b92259d9adb54d05b9dc1c119",
		},
		{
			txID:       "3aa7206a80aac354beccd1babbaf7e2872db25f0b594f44341544d14dd534442",
			merklePath: "fe737c0d000e02fda4090058f267e1f97edc0b5ed7cda51ec1a2ed9b1a4e8d52a214e92be5ebacf4ac2beafda509022bd391ae6522e1455b5fd627ae8b5eba3c777016ba2ca837760852f8f86cf00501fdd30400f564822a9d5d4dc429fae8fa4ff7abad66d5e29bc2c072427c2d3f7b211dbc6b01fd680200e312bdeb0ebea64d36bc10350bf198e1151ad50c4edc2020588ca0bb0631665701fd35010084077e1393ecbc63e48ac760bc85f8cdc20799f055b58f75dbb57cd6c2bb2bb1019b00bb362b92486613f2b21521e34670b375740bbf6ff2215a23146bcc9bebcf1535014c00ded9dacbf223b44cda87a0398fb3f868e7c55e2492e0f64868d3b2a2d3c0bcbc012700c7858f9e2146e68fb257ad9cd1a56dced67bddd747ef9ed695564d58b4e8c06a011200fd635ca6dbc04bfc6b1c7a237a3b26bcf49edbbb94928b7eaf4a2f0caf6c49da01080077477918f4fad72829f9d11bdd296ae67ac2f9053a298f19ec99ee70c71957980105009aaaf25295c6417a6bfb7525e6a0860224b8671edb35ed8afc734e34a87d9c36010300baf8b2ab2122cc13b0b7658543acec6421ccaceb29cf8b877bf3b9d9c70e02d90100000e5b06d63ba8b59509093aeead1f9781c9b36f03626f86e1f1c6efc3cb5b8110010100a6580281dad52f604b7cc558001ac7453856d4ada51cb87cb522407a06a0f8f20101005a37a1ab63894d1741be55cad1e9a3c74d2996f2d97a9812437d213de1b6190c",
		},
		{
			txID:       "b2a545d709b75b6816c21ffedd42c0724732ff08cccf5eb7f3e054e3d4467e9d",
			merklePath: "fe717c0d000b02fda30400df092d58ccefaa68b84080ebd9c6f9be0575dceeaef722cb5f2a566845fa1486fda204029d7e46d4e354e0f3b75ecfcc08ff324772c042ddfe1fc216685bb709d745a5b201fd5002004305b49fb76f593fc18cb3a6fdb71285bf21dbc56a18e3aba54835ed7f0a158101fd2901008116fda9df53b0520742312133a5ed26d25c2cf993b4c7e4e161d2b976a8a761019500fa0ef6508d4cdf85b2084e753ab6dac92ed623efd32c33b097af2cf2f08bfac1014b00b731c951d85d21a19d2b3f4b762a7474ee298e4ad7fb17e7f617f5201fe24bb9012400aea3070be0e38e604859d2cf14868df3452ffd474dfaf38cc410c306c53b14a8011300d995a70b2bd2cb7448c2595669a765bd4161f6d3393ac5cb47f7a5f880caa86b01080038d120bef909e8e445720d9b72196a11a010da74b4cb270c6effe3a50ea693e40105000e652639164cfff231bd2b99a054e9728bbbfba548bbc84a06abd07287ad3db1010301010000e3d317bff6377ac13aafb1b1d205c735740b97101c033a2b58e7fbee74af68a3",
		},
		{
			txID:       "79e4a154cd87f425e2a04357d0d224485e645613acb5531c78512efdb9fe7de6",
			merklePath: "fe727c0d000c02fd340c0031cdf9906d751da820db27f4aec37e97324db6a011c7af60f0de1d724d83593ffd350c02c9de547fde541b8a301e53905e72968f67d09c9ebf8380764d4a2ffe2c163f0401fd1b0600b2ac4a95530250d0380ee8987ff225b7bcb6bb518cbefa92ced0d339a56f540601fd0c030091b9d86f7931105839a86e088b5fdc2ae503375065c75da651ca80e8d0467dc501fd87010064c804de680a3e0d0cd9a3d560e717a3f2b108d06d7c6a3336074882cb786d8b01c200e85cae6784fec2737ec0432042258a8a3d1f51e2f2324ef5e4b6a93d3a9ab94a016000016d1dcdb793c64dcc3b71d6cde9b1493c2d3f0543bfb7b6be656f4d49e6aa9c01310028ec85f319b1ca4cc49cd385a2891bc053eb7cfea3c4bf1153f2f81f2d995487011901010d0101070101020046b36aecf8805aca15d08adbd07c90d2db696e197d713c729c9a63f071a2b276010000f4f8c78fa299561bf543d4a67494976ad4b2f0f823c1f2cdb6983018c9a4fa53",
		},
		{
			txID:       "bc8f206623ecf9c21494f81a17b79242e1a18d9bd39ae0bc1a7f585cfc12b040",
			merklePath: "fe707c0d000b02fd50040091de324ab5d668dad4b52584bc877711492d5034afb93450e33eebe78823cce7fd51040240b012fc5c587f1abce09ad39b8da1e14292b7171af89414c2f9ec2366208fbc01fd2902008d684da7ddc943f2f25e9ca2c157aed3e3891e73846766742467a7617608b3cf01fd150100f2806e213f5698723396722b0c6b75a4ef27f7499d24271ff992335abb04c546018b0008c860875d4d9234adece2a834881b8f342dd5240c30004127432a8ac1a779630144004908df3c9c9f61d5430d407d2889bbdf4df5cccfa41dfbc6cb751cce71238d73012300a417b60fe133ea00199bd8613574b425b2d5c2b116446244f2cda93357e990aa0110004f88d9d6265984e8bf47229b62932d340c6a23fb274b858a27af676930ffce770109000ae5a40fe75df174dc39e99c55b4720c022595b3c4aee3dc8514801412408c2c010500b73818a995d825c4c16a14bc30570388abe2975d4967e00fe7779bd41c8145650103005b5edeab6c67734e6f8e6a8d9c154d852d2e2f5bcc89150961e22b34ba334fde010000d2b9ec57b200941e909eaaccd66c99be452e058b92259d9adb54d05b9dc1c119",
		},
		{
			txID:       "92dfd05c4cb99685adc9a08cc48c471ea9625d2301a047d540d354ad7a2d6642",
			merklePath: "fe737c0d000e02fd540700fe0576da2811247f26e254f5b94a736d2a90c7d9808b4b7b61f623e69becc87efd550702825f4d1fb74e143a046aa899e2ee11fd613175ad07f173521feab47d05292fb301fdab0300df51f177dc454b3d8029e36e3a085a7fd35df542aacb1d68b435d71f2cc58f5e01fdd40100657985f63e3e311038604883b3e512942e14b23c6b2325513bd9f47c060076bc01eb00b70ed32ecbd8dc2fcc1524f8fd67ee475c28ad5b8d55542c62bbfe6c31febff90174005094f99f7bf5bb4167e4fe0d8a85fbb38b4935b08d7d30a1d55d3132afbdc173013b00e169a54a72038473f1253da7850bdb585a317ea03b8e6222d691ef965189bfc5011c006158a4a03688a0861b2cc9061bd8d377f1924fe1fc82afaf6710fb28fd433d41010f0072557bec7e9dec62b0706d0692097086d367ff54fbedfa77f4ab887f4ed94958010600b40256f98450c25bafa3d4ffd84d6230926b523ae990f338ac4cb0ef559237270102009473b03e411bf6ed0c8dd026158833d889c5a6c212f75c0d0b7229be780b6e7d010000c7493d109640d45143577896b5218e138d6b79ea7cefaf1b6dda892abbf0db47010100e04f384715b49643de713b4519565b288e6dc0244647cff030c7ed44136742120101008d618a2dd5929cd2a2a130a2d88d8bc8404fa7cefaccab366256112a9b8d3add0101005a37a1ab63894d1741be55cad1e9a3c74d2996f2d97a9812437d213de1b6190c",
		},
		{
			txID:       "6012036e0740f2c30c0681732ee68e2d56b02332de678a3a83e5659045b13660",
			merklePath: "fe737c0d000e02fd500d00fb762b1c766c6b367f5af32494642829a352ff7ace0424727d833ffa5c95a938fd510d02f6aebc4e0448f934b9202b81a0bd4e1c1cac76f895cfe3050d10a4439ab56a1f01fda90600a957bc0ba14a155649057d1fa5cf88df520d52c9cfa859a24c55ba1a07b539bd01fd5503007bd95b9083642269d324325d80656c4eaec95c47e39d5c6fbe21c30bda2dd4d901fdab0100f5c45cefcbfbf70469872b7b6ff164ec05cd17334798271688f8e8354a54898601d4005df3d7c70a2e3337508c1ea32c922e53a168634e04e4b41a71bafee89caa1f47016b0013a829344a0be6f51bade86a586cb26ec38da5493437abe4d408e9df536236b6013400250c3bf9fcfe58d1ba6f27f6ea2c0f6da7bff922697512c606024b4122c2e1f7011b00a75634f454c1ee954c6273475702ef59d396f1c2fd06974185518e004d89aa69010c0048dbadfcf10010265cd0b441a094ada6d3eee66b30fcb04b00a01dd086dab0e10107002c91719c0664fef37c15a9d05326f4abe667dc6f93017654a0299a5f413495fb010200e89805e9afa3eb9da9973ea6edb70ad7b0d4f54fab82689c54c7653c7b9d763d010000b9f4ef4cd8bed01ed706fc3f3a57aa5edee53024ffb406e3f0141ba273ffed630101008d618a2dd5929cd2a2a130a2d88d8bc8404fa7cefaccab366256112a9b8d3add0101005a37a1ab63894d1741be55cad1e9a3c74d2996f2d97a9812437d213de1b6190c",
		},
		{
			txID:       "530fac16d68de2481c0530b41328a98cfa7418ea1975af865bac3edbeba859e1",
			merklePath: "fe737c0d000e02f20050f175a49ae31daa45df3816feee6da8c7a9175f2d1647e25b90e2db8011ffc8f3028ba0a57733651e936d5c1f592fb16c55e6849874c1b418cf114b22f4fbef7eb6017800ee16a49f29648d8e5ecf69c9a4019ce9392c298b893d86bfa2c8a34eb9922ea0013d0095f160e0c728d1caca25c1b1b05009ea709be422e2a3bdb451db8c18938f51fe011f002d26a0115a4101d3c65d6e37144a4669cddd5efc9c3c0b172123b41f8b1ff880010e001d4b5516e65131601a0c40fb2280313306e72e7d518b2591c84f51a57deccb390106009f0253356134f05d834ede4c97b06d023818aa5f04d1d421cf48d223af9010f301020054607e62e22e4c34b169a7cea6a6d452f88bd781a7b7a6774bcc5068db618984010000a9eaa824ecbfc681166231bcbe795cc96e0ce4e69332af78dc382f60a1f2a8570101009c75bd18682b730a1f850c3bd755e14c5a512afbec48c64260e130d3f30287d901010050b49365be38d924fa839b6aa07127ec38872baea126c46b02fa841c0e92952b0101003e86684385500c0bfc6efdfd247d87c4cd6b68f8e0d2c40591e82ce3ae57b693010100f0134e478fe124b42439e2a44fc8c8e1a6611e7053b65b93610e64898548d40f01010019ef2b35e5f6dea5998b098e1c193fccc2a77e399ab01497479fa49080a56bc501010086934872ec0debe53824ea37ee6fcc1c9ddc4b6980444f5723469a4e19910f6c",
		},
		{
			txID:       "6de454e41765b71439475dd4522e1ce648f9c88c97e8d245794a9be4a626d9a5",
			merklePath: "fe727c0d000c02fd780900cfadef294ddeb23e27e0b70cd5d62bef29b2a9bd40fc6b0104b5127a55094b9ffd7909022f86416a17aba49ef0944e8b20944e45c2bfa8d9c47f3b3a4975cdcc0098a4eb01fdbd04008f79ffa5f8f09fa8006d667228e861ea18d1ec8ee829c6285241462d9c4ce97301fd5f02008896be455ec345d91a36948896a82affb9cc8e9afcca27a6dc1fa95b9052a9a701fd2e01003d987e926a04deb796236bcbd6dc9b2086e2313c0338361f06304f4d1b2072e80196002fc0fb26043a69297ba63b7d4843b22c6989a675408ab6a8ab488a7d51c6923a014a0061f58868fd3cc9a83a273c5bf0daabb90e087432c05e68b4e5de492eca68b8df012400264dc7de5790401ede14e5abd61bd13a35f150a80de9d9ad17281af9de7706a1011300863e15295285064176bda455536b2a4153be79ece57ffcf6debb6132bef159eb01080070e862545a1ce8b0af401415688e9aa20e9f1313e1881b4388a6fc2e5336edaf01050009d26230e7db6bea2730f39339b2123b909f72619111b26dae84f23cf7db561201030060f15a67a2a380d422fc7a8373d0b7d692a346a24a43f0357242ac06ff5048a1010000f4f8c78fa299561bf543d4a67494976ad4b2f0f823c1f2cdb6983018c9a4fa53",
		},
		{
			txID:       "210901c2af430a534b42de1efb46a42408430d71d8b154504c82ed896b905858",
			merklePath: "fe6f7c0d000b02fd480700000934891d3d9960c90f6b9fc7696ad23a16ea3d90f173aca76074b62157ededfd4907025858906b89ed824c5054b1d8710d430824a446fb1ede424b530a43afc201092101fda50300e1374982e50f509fb5f41e61f9ab042d49f2580eaf260f156530fba47382b56801fdd301004717f780bb987d9a08b99c76d54e1206fc54eb82616b527cb671d2366c4d63a401e800c94d6de159b0d193e3e9bbc09eddb31d133e0836c63073197380062137302a42017500881c9b4eb5042a72aa2079315b2b720d51e40502db47962c189546393f2a8cf8013b00618cd3e593e20ad855772da715eedb91cd6caaae5fb9a055c3687f56d1981e59011c002929705d6b78894bceb2257bc1c2d8b45705c4dcecd7dfac126384019f1f4f10010f00bcf76dff8c10be145c35d9b31721bf053543838f8a872a3aa8714c9d5e4aa6e7010600c6bbd38b948d2c3ed0e0ab2c913abad1fb2b3932079c71370752f8668de723c001020043f73f6a63b55c32109dc437944b8534b3f53954fffbe748606ec22551c95dbf010000c5a623cc0ffa573f391c7092b1b1564603d82da42c86effd3a8ea489fbd33357",
		},
		{
			txID:       "8c563c65e323095270b0adfcabba6486ed058f432aa0dc438f0f14e6c6857587",
			merklePath: "fe727c0d000c02fd4504007f01301ce48ac067d3b7b1709478e01f0dca624544707cc49167d5d2cbf32571fd440402877585c6e6140f8f43dca02a438f05ed8664baabfcadb070520923e3653c568c01fd230200b729a5cee3cc14d086c8891f49676a332d26b0e266f1a5fe879cca40cbdade3e01fd100100cf74033c5c61fa4604003dd35d7928d391407263800948450c82eabcd29ca9760189004922b2faeb85c37a41ce9bc5430ce7a28bc707de32cad0e3f8b4f6f4e4c48a23014500c000f4137d0ce168eccb23730acb998bf029fda177b0ac2d593a78e8efdffc000123007d447b3b2648eef8fc9c8ee9286ed6b3dc89c44bc0f285ef7a068ed910e9119a0110001a6e63d87d488931e1a53b9f9878a718fc38cf5865c20a429ef5358ebc52b42b0109000ab2a281f0984234eefa12a636e3b5b0fc45a8ee0a0094d22a404942a89cc9cc010500a57942e85bbaba488d58c187bd4bb944b7822eb6e7fb9aa7ac23587f5615336e010300091ad38b6ec1a97ebf66a9b326abcf3ee737da3eec423144b64a92dd30dfbe54010000df5088c9ada3c91a1d0137342122073f929adbb354dba7e39dddaa6b040fedc30101004582213a73884fbaf6ea9141cf5b1fd6defce00ed2d97e95e571845a3ad459ac",
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
