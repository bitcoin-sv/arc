package blocktx_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestExtractHeight(t *testing.T) {
	coinbase, _ := hex.DecodeString("01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff570350cc0b041547b5630cfabe6d6d0000000000000000000000000000000000000000000000000000000000000000010000000000000047ed20542096bd0000000000143362663865373833636662643732306431383436000000000140be4025000000001976a914c9b0abe09b7dd8e9d1e8c1e3502d32ab0d7119e488ac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := sdkTx.NewTransactionFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := blocktx.ExtractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(773200), height, "height should be 773200, got %d", height)
}

func TestExtractHeightForRegtest(t *testing.T) {
	coinbase, _ := hex.DecodeString("02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0502dc070101ffffffff012f500900000000002321032efe256e14fd77eea05d0453374f8920e0a7a4a573bb3937ef3f567f3937129cac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := sdkTx.NewTransactionFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := blocktx.ExtractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(2012), height, "height should be 2012, got %d", height)
}

func TestHandleBlock(t *testing.T) {
	// define HandleBlock function parameters (BlockMessage and p2p.PeerI)

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
		setBlockProcessingErr error
		bhsProcInProg         []*chainhash.Hash
	}{
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
			batchSize := 4
			storeMock := &storeMocks.BlocktxStoreMock{
				InsertBlockFunc: func(ctx context.Context, block *blocktx_api.Block) (uint64, error) {
					return 0, nil
				},
				MarkBlockAsDoneFunc: func(ctx context.Context, hash *chainhash.Hash, size uint64, txCount uint64) error {
					return nil
				},
			}

			mq := &mocks.MessageQueueClientMock{
				PublishMarshalFunc: func(topic string, m protoreflect.ProtoMessage) error {
					return nil
				},
			}

			// build peer manager and processor
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			var blockRequestCh chan blocktx.BlockRequest = nil
			blockProcessCh := make(chan *p2p.BlockMessage, 10)

			peerHandler := blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)
			processor, err := blocktx.NewProcessor(logger, storeMock, blockRequestCh, blockProcessCh, blocktx.WithTransactionBatchSize(batchSize), blocktx.WithMessageQueueClient(mq))
			require.NoError(t, err)
			defer processor.Shutdown()

			processor.StartBlockProcessing()

			var expectedInsertedTransactions []*blocktx_api.TransactionAndSource
			transactionHashes := make([]*chainhash.Hash, len(tc.txHashes))
			for i, hash := range tc.txHashes {
				txHash, err := chainhash.NewHashFromStr(hash)
				require.NoError(t, err)
				transactionHashes[i] = txHash

				expectedInsertedTransactions = append(expectedInsertedTransactions, &blocktx_api.TransactionAndSource{Hash: txHash[:]})
			}

			var insertedBlockTransactions []*blocktx_api.TransactionAndSource

			storeMock.UpsertBlockTransactionsFunc = func(ctx context.Context, blockId uint64, transactions []*blocktx_api.TransactionAndSource, merklePaths []string) ([]store.UpsertBlockTransactionsResult, error) {
				require.True(t, len(merklePaths) <= batchSize)
				require.True(t, len(transactions) <= batchSize)

				for i, path := range merklePaths {
					bump, err := bc.NewBUMPFromStr(path)
					require.NoError(t, err)
					tx, err := chainhash.NewHash(transactions[i].GetHash())
					require.NoError(t, err)
					root, err := bump.CalculateRootGivenTxid(tx.String())
					require.NoError(t, err)

					require.Equal(t, root, tc.merkleRoot.String())
				}

				insertedBlockTransactions = append(insertedBlockTransactions, transactions...)

				result := make([]store.UpsertBlockTransactionsResult, len(transactions))
				for i, tx := range transactions {
					result[i] = store.UpsertBlockTransactionsResult{TxHash: tx.Hash}
				}

				return result, nil
			}

			peer := &mocks.PeerMock{
				StringFunc: func() string {
					return ""
				},
			}

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
			err = peerHandler.HandleBlock(blockMessage, peer)
			require.NoError(t, err)
			time.Sleep(20 * time.Millisecond)
			require.ElementsMatch(t, expectedInsertedTransactions, insertedBlockTransactions)
		})
	}
}

func TestStartFillGaps(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	tt := []struct {
		name            string
		hostname        string
		getBlockGapsErr error
		blockGaps       []*store.BlockGap

		minExpectedGetBlockCapsCalls int
	}{
		{
			name:     "success",
			hostname: hostname,
			blockGaps: []*store.BlockGap{
				{
					Height: 822014,
					Hash:   testdata.Block1Hash,
				},
				{
					Height: 822015,
					Hash:   testdata.Block2Hash,
				},
			},

			minExpectedGetBlockCapsCalls: 1,
		},
		{
			name:            "error getting block gaps",
			hostname:        hostname,
			getBlockGapsErr: errors.New("failed to get block gaps"),

			minExpectedGetBlockCapsCalls: 1,
		},
		{
			name:      "no block gaps",
			hostname:  hostname,
			blockGaps: make([]*store.BlockGap, 0),

			minExpectedGetBlockCapsCalls: 4,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			getBlockErrCh := make(chan error)

			getBlockGapTestErr := tc.getBlockGapsErr
			storeMock := &storeMocks.BlocktxStoreMock{
				GetBlockGapsFunc: func(ctx context.Context, heightRange int) ([]*store.BlockGap, error) {
					if getBlockGapTestErr != nil {
						getBlockErrCh <- getBlockGapTestErr
						return nil, getBlockGapTestErr
					}

					return tc.blockGaps, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			blockRequestCh := make(chan blocktx.BlockRequest, 10)
			var blockProcessCh chan *p2p.BlockMessage = nil

			_ = blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)
			processor, err := blocktx.NewProcessor(logger, storeMock, blockRequestCh, blockProcessCh, blocktx.WithFillGapsInterval(time.Millisecond*20))
			require.NoError(t, err)

			peerMock := &mocks.PeerMock{
				StringFunc: func() string {
					return ""
				},
			}
			peers := []p2p.PeerI{peerMock}

			processor.StartFillGaps(peers)

			select {
			case hashPeer := <-processor.GetBlockRequestCh():
				require.True(t, testdata.Block1Hash.IsEqual(hashPeer.Hash))
			case err = <-getBlockErrCh:
				require.ErrorIs(t, err, tc.getBlockGapsErr)
			case <-time.NewTimer(100 * time.Millisecond).C:
			}

			processor.Shutdown()

			require.GreaterOrEqual(t, len(storeMock.GetBlockGapsCalls()), tc.minExpectedGetBlockCapsCalls)
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
			registerErrTest := tc.registerErr
			storeMock := &storeMocks.BlocktxStoreMock{
				RegisterTransactionsFunc: func(ctx context.Context, transaction []*blocktx_api.TransactionAndSource) ([]*chainhash.Hash, error) {
					return nil, registerErrTest
				},
			}

			txChan := make(chan []byte, 10)

			txChan <- testdata.TX1Hash[:]
			txChan <- testdata.TX2Hash[:]
			txChan <- testdata.TX3Hash[:]
			txChan <- testdata.TX4Hash[:]

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			processor, err := blocktx.NewProcessor(
				logger,
				storeMock,
				nil,
				nil,
				blocktx.WithRegisterTxsInterval(time.Millisecond*20),
				blocktx.WithRegisterTxsChan(txChan),
				blocktx.WithRegisterTxsBatchSize(3),
			)
			require.NoError(t, err)

			processor.StartProcessRegisterTxs()

			time.Sleep(120 * time.Millisecond)
			processor.Shutdown()

			require.Equal(t, tc.expectedRegisterTxsCalls, len(storeMock.RegisterTransactionsCalls()))
		})
	}
}

func TestStartBlockRequesting(t *testing.T) {
	// define HandleBlock function parameters (BlockMessage and p2p.PeerI)

	blockHash, err := chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
	require.NoError(t, err)

	tt := []struct {
		name                  string
		setBlockProcessingErr error
		writeMsgErr           error
		bhsProcInProg         []*chainhash.Hash

		expectedSetBlockProcessingCalls                 int
		expectedGetBlockHashesProcessingInProgressCalls int
		expectedPeerWriteMessageCalls                   int
	}{
		{
			name: "process block",

			expectedSetBlockProcessingCalls:                 1,
			expectedGetBlockHashesProcessingInProgressCalls: 1,
			expectedPeerWriteMessageCalls:                   1,
		},
		{
			name:                  "block already processed",
			setBlockProcessingErr: store.ErrBlockProcessingDuplicateKey,

			expectedSetBlockProcessingCalls:                 1,
			expectedGetBlockHashesProcessingInProgressCalls: 1,
			expectedPeerWriteMessageCalls:                   0,
		},
		{
			name:                  "failed to set block processing",
			setBlockProcessingErr: errors.New("failed to set block processing"),

			expectedSetBlockProcessingCalls:                 1,
			expectedGetBlockHashesProcessingInProgressCalls: 1,
			expectedPeerWriteMessageCalls:                   0,
		},
		{
			name:          "max blocks being processed reached",
			bhsProcInProg: []*chainhash.Hash{testdata.Block1Hash, testdata.Block2Hash},

			expectedSetBlockProcessingCalls:                 0,
			expectedGetBlockHashesProcessingInProgressCalls: 1,
			expectedPeerWriteMessageCalls:                   0,
		},
		{
			name:        "write message error",
			writeMsgErr: errors.New("failed to write message"),

			expectedSetBlockProcessingCalls:                 1,
			expectedGetBlockHashesProcessingInProgressCalls: 1,
			expectedPeerWriteMessageCalls:                   1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			setBlockProcessingErrTest := tc.setBlockProcessingErr
			bhsProcInProgErr := tc.bhsProcInProg
			storeMock := &storeMocks.BlocktxStoreMock{
				SetBlockProcessingFunc: func(ctx context.Context, hash *chainhash.Hash, processedBy string) (string, error) {
					return "abc", setBlockProcessingErrTest
				},
				GetBlockHashesProcessingInProgressFunc: func(ctx context.Context, processedBy string) ([]*chainhash.Hash, error) {
					return bhsProcInProgErr, nil
				},
			}

			writeMsgErrTest := tc.writeMsgErr
			peerMock := &mocks.PeerMock{
				WriteMsgFunc: func(msg wire.Message) error {
					return writeMsgErrTest
				},
				StringFunc: func() string {
					return ""
				},
			}

			// build peer manager
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			blockRequestCh := make(chan blocktx.BlockRequest, 10)
			blockProcessCh := make(chan *p2p.BlockMessage, 10)

			peerHandler := blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)
			processor, err := blocktx.NewProcessor(logger, storeMock, blockRequestCh, blockProcessCh)
			require.NoError(t, err)

			// send msg Inv to blockRequest channel
			err = peerHandler.HandleBlockAnnouncement(wire.NewInvVect(wire.InvTypeBlock, blockHash), peerMock)
			require.NoError(t, err)

			processor.StartBlockRequesting()

			// call tested function
			require.NoError(t, err)
			time.Sleep(20 * time.Millisecond)
			processor.Shutdown()

			require.Equal(t, tc.expectedGetBlockHashesProcessingInProgressCalls, len(storeMock.GetBlockHashesProcessingInProgressCalls()))
			require.Equal(t, tc.expectedSetBlockProcessingCalls, len(storeMock.SetBlockProcessingCalls()))
			require.Equal(t, tc.expectedPeerWriteMessageCalls, len(peerMock.WriteMsgCalls()))
		})
	}
}

func TestStartProcessRequestTxs(t *testing.T) {
	tt := []struct {
		name            string
		requests        int
		getMinedErr     error
		publishMinedErr error
		requestedTx     []byte

		expectedGetMinedCalls     int
		expectedPublishMinedCalls int
	}{
		{
			name:        "success - 5 requests",
			requests:    5,
			requestedTx: testdata.TX1Hash[:],

			expectedGetMinedCalls:     2,
			expectedPublishMinedCalls: 2,
		},
		{
			name:        "5 requests, error - get mined",
			requests:    5,
			getMinedErr: errors.New("get mined error"),
			requestedTx: testdata.TX1Hash[:],

			expectedGetMinedCalls:     4,
			expectedPublishMinedCalls: 0,
		},
		{
			name:            "5 requests, error - publish mined",
			requests:        5,
			publishMinedErr: errors.New("publish mined error"),
			requestedTx:     testdata.TX1Hash[:],

			expectedGetMinedCalls:     4,
			expectedPublishMinedCalls: 4,
		},
		{
			name:        "success - 2 requests",
			requests:    2,
			requestedTx: testdata.TX1Hash[:],

			expectedGetMinedCalls:     1,
			expectedPublishMinedCalls: 1,
		},
		{
			name:        "success - 0 requests",
			requests:    0,
			requestedTx: testdata.TX1Hash[:],

			expectedGetMinedCalls:     0,
			expectedPublishMinedCalls: 0,
		},
		{
			name:        "error - not a tx",
			requests:    1,
			requestedTx: []byte("not a tx"),

			expectedGetMinedCalls:     0,
			expectedPublishMinedCalls: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			publishMinedErrTest := tc.publishMinedErr
			getMinedErrTest := tc.getMinedErr
			storeMock := &storeMocks.BlocktxStoreMock{
				GetMinedTransactionsFunc: func(ctx context.Context, hashes []*chainhash.Hash) ([]store.GetMinedTransactionResult, error) {
					for _, hash := range hashes {
						require.Equal(t, testdata.TX1Hash, hash)
					}

					return []store.GetMinedTransactionResult{{
						TxHash:      testdata.TX1Hash[:],
						BlockHash:   testdata.Block1Hash[:],
						BlockHeight: 1,
					}}, getMinedErrTest
				},
			}

			mq := &mocks.MessageQueueClientMock{
				PublishMarshalFunc: func(topic string, m protoreflect.ProtoMessage) error {
					return publishMinedErrTest
				},
			}

			requestTxChannel := make(chan []byte, 5)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			processor, err := blocktx.NewProcessor(logger, storeMock,
				nil, nil,
				blocktx.WithRegisterRequestTxsInterval(20*time.Millisecond),
				blocktx.WithRegisterRequestTxsBatchSize(3),
				blocktx.WithRequestTxChan(requestTxChannel),
				blocktx.WithMessageQueueClient(mq))
			require.NoError(t, err)

			for i := 0; i < tc.requests; i++ {
				requestTxChannel <- tc.requestedTx
			}

			processor.StartProcessRequestTxs()

			// call tested function
			require.NoError(t, err)
			time.Sleep(20 * time.Millisecond)
			processor.Shutdown()

			require.Equal(t, tc.expectedGetMinedCalls, len(storeMock.GetMinedTransactionsCalls()))
			require.Equal(t, tc.expectedPublishMinedCalls, len(mq.PublishMarshalCalls()))
		})
	}
}

func TestStart(t *testing.T) {
	tt := []struct {
		name     string
		topicErr map[string]error

		expectedErrorStr string
	}{
		{
			name: "success",
		},
		{
			name:     "error - subscribe mined txs",
			topicErr: map[string]error{blocktx.RegisterTxTopic: errors.New("failed to subscribe")},

			expectedErrorStr: "failed to subscribe to register-tx topic: failed to subscribe",
		},
		{
			name:     "error - subscribe submit txs",
			topicErr: map[string]error{blocktx.RequestTxTopic: errors.New("failed to subscribe")},

			expectedErrorStr: "failed to subscribe to request-tx topic: failed to subscribe",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			storeMock := &storeMocks.BlocktxStoreMock{}

			mqClient := &mocks.MessageQueueClientMock{
				SubscribeFunc: func(topic string, msgFunc func([]byte) error) error {
					err, ok := tc.topicErr[topic]
					if ok {
						return err
					}
					return nil
				},
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			processor, err := blocktx.NewProcessor(logger, storeMock, nil, nil, blocktx.WithMessageQueueClient(mqClient))
			require.NoError(t, err)
			err = processor.Start()
			if tc.expectedErrorStr != "" || err != nil {
				require.ErrorContains(t, err, tc.expectedErrorStr)
				return
			} else {
				require.NoError(t, err)
			}

			processor.Shutdown()
		})
	}
}
