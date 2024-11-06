package integrationtest

// Components of this test:
// 		Blocktx Processor
// 		Postgresql Store - running on docker
// 		PeerHandler - mocked
// 		Message queue sending txs to metamorph - mocked
//
// Flow of this test:
// 		1. Blocks at heights 822014-822017, 822019-822020 and 822022-822023 are added to db from fixtures
// 		2. A hardcoded msg with competing block at height 822015 is being sent through the mocked PeerHandler
// 		3. This block has a chainwork lower than the current tip of chain - becomes STALE
// 		4. Registered transactions from this block that are not in the longest chain are published to metamorph message queue with blockstatus = STALE
// 		5. Next competing block, at height 822016 is being send through the mocked PeerHandler
// 		6. This block has a greater chainwork than the current tip of longest chain - it becomes LONGEST despite not being the highest
// 		7. Verification of reorg - checking if statuses are correctly switched
// 		8. Verification of transactions
// 			- transactions from the stale chain becoming the longest are published
// 			- transactions that were previously in the longest chain are published with udpated block data
// 			- transactions that were previously in the longest chain, but are not in the stale chain are published with blockstatus = STALE
// 		9. A new block at height 822018 is being sent through the mocked PeerHandler
// 		10. This block is extending the previously LONGEST but now STALE chain and finds orphaned chain at heights 822019, 822020
// 		11. The tip of the orphaned chain does not have a greater chainwork than the current longest chain - entire orphaned chain becomes STALE
// 		12. A new block at height 822021 is being sent through the mocked PeerHandler
// 		13. This block extends the STALE chain and finds orphaned chain at height 822022, 822023
// 		14. The tip of the orphaned chain has a greater chainwork than the current tip of longest chain
// 			- entire STALE chain at heights 822015 - 822023 becomes LONGEST
// 			- entire LONGEST chain at height 822015 - 822016 becomes STALE
// 		15. Verification of reorg - checking if statuses are correctly switched (for blocks and for transactions)

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	blockchain "github.com/bitcoin-sv/arc/internal/blocktx/blockchain_communication"
	blocktx_p2p "github.com/bitcoin-sv/arc/internal/blocktx/blockchain_communication/p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	nats_mock "github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core/mocks"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	postgresPort   = "5432"
	migrationsPath = "file://../store/postgresql/migrations"
	dbName         = "main_test"
	dbUsername     = "arcuser"
	dbPassword     = "arcpass"
)

var (
	dbInfo string
	dbConn *sql.DB
)

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
		return 1
	}

	port := "5437"
	resource, connStr, err := testutils.RunAndMigratePostgresql(pool, port, "blocktx", migrationsPath)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	dbInfo = connStr

	dbConn, err = sql.Open("postgres", dbInfo)
	if err != nil {
		log.Fatalf("failed to create db connection: %v", err)
		return 1
	}

	return m.Run()
}

const (
	blockHash822011 = "bf9be09b345cc2d904b59951cc8a2ed452d8d143e2e25cde64058270fb3a667a"

	blockHash822014_startOfChain = "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000"
	blockHash822015              = "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000"
	blockHash822016              = "e1df1273e6e7270f96b508545d7aa80aebda7d758dc82e080000000000000000"
	blockHash822017              = "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000"

	blockHash822015_fork = "82471bbf045ab13825a245b37de71d77ec12513b37e2524ec11551d18c19f7c3"
	blockHash822016_fork = "032c3688bc7536b2d787f3a196b1145a09bf33183cd1448ff6b1a9dfbb022db8"

	blockHash822018        = "212a7598a62295f1a520ef525a34f657bc636d9da9bda74acdf6f051cd84c353"
	blockHash822019_orphan = "00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656"
	blockHash822020_orphan = "00000000000000000a5c4d27edc0178e953a5bb0ab0081e66cb30c8890484076"
	blockHash822021        = "743c7dc491ae5fddd37ebf63058f9574b4db9f6a89f483a4baec31820e5df61d"
	blockHash822022_orphan = "0000000000000000059d6add76e3ddb8ec4f5ffd6efecd4c8b8c577bd32aed6c"
	blockHash822023_orphan = "0000000000000000082131979a4e25a5101912a5f8461e18f306d23e158161cd"

	txhash822015   = "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
	txhash822015_2 = "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"
	txhash822016   = "2ff4430eb883c6f6c0640a5d716b2d107bbc0efa5aeaa237aec796d4686b0a8f"
	txhash822017   = "ece2b7e40d98749c03c551b783420d6e3fdc3c958244bbf275437839585829a6"
)

func TestReorg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	defer pruneTables(t, dbConn)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var blockRequestCh chan blocktx_p2p.BlockRequest = nil // nolint: revive
	blockProcessCh := make(chan *blockchain.BlockMessage, 10)

	blocktxStore, err := postgresql.New(dbInfo, 10, 80)
	require.NoError(t, err)

	publishedTxs := make([]*blocktx_api.TransactionBlock, 0)

	mockNatsConn := &nats_mock.NatsConnectionMock{
		PublishFunc: func(subj string, data []byte) error {
			serialized := &blocktx_api.TransactionBlock{}
			err := proto.Unmarshal(data, serialized)
			require.NoError(t, err)

			publishedTxs = append(publishedTxs, serialized)
			return nil
		},
	}
	mqClient := nats_core.New(mockNatsConn, nats_core.WithLogger(logger))

	p2pMsgHandler := blocktx_p2p.NewMsgHandler(logger, blockRequestCh, blockProcessCh)
	processor, err := blocktx.NewProcessor(
		logger,
		blocktxStore,
		blockRequestCh,
		blockProcessCh,
		blocktx.WithMessageQueueClient(mqClient),
	)
	require.NoError(t, err)

	processor.StartBlockProcessing()

	testHandleBlockOnEmptyDatabase(t, p2pMsgHandler, blocktxStore)
	publishedTxs = publishedTxs[:0] // clear slice for the next test

	// only load fixtures at this point
	testutils.LoadFixtures(t, dbConn, "fixtures")

	expectedTxs := testHandleStaleBlock(t, p2pMsgHandler, blocktxStore)
	// verify the transaction was correctly published to metamorph
	verifyTxs(t, expectedTxs, publishedTxs)
	// clear slice for the next test
	publishedTxs = publishedTxs[:0]

	expectedTxs = testHandleReorg(t, p2pMsgHandler, blocktxStore)
	verifyTxs(t, expectedTxs, publishedTxs)
	publishedTxs = publishedTxs[:0]

	testHandleStaleOrphans(t, p2pMsgHandler, blocktxStore)

	expectedTxs = testHandleOrphansReorg(t, p2pMsgHandler, blocktxStore)
	verifyTxs(t, expectedTxs, publishedTxs)
}

func testHandleBlockOnEmptyDatabase(t *testing.T, p2pMsgHandler *blocktx_p2p.MsgHandler, store *postgresql.PostgreSQL) {
	// test for empty database edge case before inserting fixtures
	prevBlockHash := testutils.RevChainhash(t, "00000000000000000a00c377b260a3219b0c314763f486bc363df7aa7e22ad72")
	txHash, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)
	merkleRoot, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)

	// should become LONGEST
	blockMessage := &blockchain.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash, // NON-existent in the db
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff,
		},
		Height:            uint64(822011),
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	p2pMsgHandler.OnReceive(blockMessage, nil)

	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	verifyBlock(t, store, blockHash822011, 822011, blocktx_api.Status_LONGEST)
}

func testHandleStaleBlock(t *testing.T, p2pMsgHandler *blocktx_p2p.MsgHandler, store *postgresql.PostgreSQL) []*blocktx_api.TransactionBlock {
	prevBlockHash := testutils.RevChainhash(t, blockHash822014_startOfChain)
	txHash := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
	txHash2 := testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430") // should not be published - is already in the longest chain
	treeStore := bc.BuildMerkleTreeStoreChainHash([]*chainhash.Hash{txHash, txHash2})
	merkleRoot := treeStore[len(treeStore)-1]

	// should become STALE
	blockMessage := &blockchain.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash, // block with status LONGEST at height 822014
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing block
		},
		Height:            uint64(822015), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash, txHash2},
	}
	blockHash := blockMessage.Header.BlockHash()

	p2pMsgHandler.OnReceive(blockMessage, nil)
	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	verifyBlock(t, store, blockHash822015_fork, 822015, blocktx_api.Status_STALE)

	// transactions expected to be published to metamorph
	expectedTxs := []*blocktx_api.TransactionBlock{
		{
			BlockHash:       blockHash[:],
			BlockHeight:     822015,
			TransactionHash: txHash[:],
			BlockStatus:     blocktx_api.Status_STALE,
		},
	}

	return expectedTxs
}

func testHandleReorg(t *testing.T, p2pMsgHandler *blocktx_p2p.MsgHandler, store *postgresql.PostgreSQL) []*blocktx_api.TransactionBlock {
	txHash := testutils.RevChainhash(t, txhash822016)
	txHash2 := testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")
	treeStore := bc.BuildMerkleTreeStoreChainHash([]*chainhash.Hash{txHash, txHash2})
	merkleRoot := treeStore[len(treeStore)-1]
	prevhash := testutils.RevChainhash(t, blockHash822015_fork)

	// should become LONGEST
	// reorg should happen
	blockMessage := &blockchain.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevhash, // block with status STALE at height 822015
			MerkleRoot: *merkleRoot,
			Bits:       0x1a05db8b, // chainwork: "12301577519373468" higher than the competing chain
		},
		Height:            uint64(822016), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash, txHash2},
	}
	blockHash := blockMessage.Header.BlockHash()

	p2pMsgHandler.OnReceive(blockMessage, nil)
	// Allow DB to process the block and perform reorg
	time.Sleep(1 * time.Second)

	// verify that reorg happened
	verifyBlock(t, store, blockHash822016_fork, 822016, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822015_fork, 822015, blocktx_api.Status_LONGEST)

	verifyBlock(t, store, blockHash822015, 822015, blocktx_api.Status_STALE)
	verifyBlock(t, store, blockHash822016, 822016, blocktx_api.Status_STALE)
	verifyBlock(t, store, blockHash822017, 822017, blocktx_api.Status_STALE)

	verifyBlock(t, store, blockHash822014_startOfChain, 822014, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822019_orphan, 822019, blocktx_api.Status_ORPHANED)

	previouslyLongestBlockHash := testutils.RevChainhash(t, blockHash822017)

	expectedTxs := []*blocktx_api.TransactionBlock{
		{ // previously in stale chain
			BlockHash:       prevhash[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, txhash822015)[:],
			BlockStatus:     blocktx_api.Status_LONGEST,
		},
		{ // previously in longest chain - also in stale - should have blockdata updated
			BlockHash:       prevhash[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, txhash822015_2)[:],
			BlockStatus:     blocktx_api.Status_LONGEST,
		},
		{ // newly mined from stale block that became longest after reorg
			BlockHash:       blockHash[:],
			BlockHeight:     822016,
			TransactionHash: txHash[:],
			BlockStatus:     blocktx_api.Status_LONGEST,
		},
		{ // previously longest chain - not found in the new longest chain
			BlockHash:       previouslyLongestBlockHash[:],
			BlockHeight:     822017,
			TransactionHash: testutils.RevChainhash(t, txhash822017)[:],
			BlockStatus:     blocktx_api.Status_STALE,
		},
	}

	return expectedTxs
}

func testHandleStaleOrphans(t *testing.T, p2pMsgHandler *blocktx_p2p.MsgHandler, store *postgresql.PostgreSQL) {
	txHash := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
	merkleRoot := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
	prevhash := testutils.RevChainhash(t, blockHash822017)

	// should become STALE
	blockMessage := &blockchain.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevhash, // block with status STALE at height 822017
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing chain
		},
		Height:            uint64(822018),
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	p2pMsgHandler.OnReceive(blockMessage, nil)
	// Allow DB to process the block and find orphans
	time.Sleep(1 * time.Second)

	// verify that the block and orphans have STALE status
	verifyBlock(t, store, blockHash822018, 822018, blocktx_api.Status_STALE)
	verifyBlock(t, store, blockHash822019_orphan, 822019, blocktx_api.Status_STALE)
	verifyBlock(t, store, blockHash822020_orphan, 822020, blocktx_api.Status_STALE)

	// verify that the blocks after the next gap are still orphans
	verifyBlock(t, store, blockHash822022_orphan, 822022, blocktx_api.Status_ORPHANED)
	verifyBlock(t, store, blockHash822023_orphan, 822023, blocktx_api.Status_ORPHANED)
}

func testHandleOrphansReorg(t *testing.T, p2pMsgHandler *blocktx_p2p.MsgHandler, store *postgresql.PostgreSQL) []*blocktx_api.TransactionBlock {
	txHash := testutils.RevChainhash(t, "3e15f823a7de25c26ce9001d4814a6f0ebc915a1ca4f1ba9cfac720bd941c39c")
	merkleRoot := testutils.RevChainhash(t, "3e15f823a7de25c26ce9001d4814a6f0ebc915a1ca4f1ba9cfac720bd941c39c")
	prevhash := testutils.RevChainhash(t, blockHash822020_orphan)

	// should become LONGEST
	// reorg should happen
	blockMessage := &blockchain.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevhash, // block with status STALE at height 822020
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing chain
			// but the sum of orphan chain has a higher chainwork and should cause a reorg
		},
		Height:            uint64(822021),
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	p2pMsgHandler.OnReceive(blockMessage, nil)
	// Allow DB to process the block, find orphans and perform reorg
	time.Sleep(2 * time.Second)

	// verify that the reorg happened
	verifyBlock(t, store, blockHash822014_startOfChain, 822014, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822015, 822015, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822016, 822016, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822017, 822017, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822018, 822018, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822019_orphan, 822019, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822020_orphan, 822020, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822021, 822021, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822022_orphan, 822022, blocktx_api.Status_LONGEST)
	verifyBlock(t, store, blockHash822023_orphan, 822023, blocktx_api.Status_LONGEST)

	verifyBlock(t, store, blockHash822015_fork, 822015, blocktx_api.Status_STALE)
	verifyBlock(t, store, blockHash822016_fork, 822016, blocktx_api.Status_STALE)

	bh822015 := testutils.RevChainhash(t, blockHash822015)
	bh822015_fork := testutils.RevChainhash(t, blockHash822015_fork)
	bh822016_fork := testutils.RevChainhash(t, blockHash822016_fork)
	bh822017 := testutils.RevChainhash(t, blockHash822017)

	expectedTxs := []*blocktx_api.TransactionBlock{
		{ // in stale chain
			BlockHash:       bh822015_fork[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, txhash822015)[:],
			BlockStatus:     blocktx_api.Status_STALE,
		},
		{ // in both chains - should have blockdata updated
			BlockHash:       bh822015[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, txhash822015_2)[:],
			BlockStatus:     blocktx_api.Status_LONGEST,
		},
		{ // in stale chain
			BlockHash:       bh822016_fork[:],
			BlockHeight:     822016,
			TransactionHash: testutils.RevChainhash(t, txhash822016)[:],
			BlockStatus:     blocktx_api.Status_STALE,
		},
		{ // in now longest chain
			BlockHash:       bh822017[:],
			BlockHeight:     822017,
			TransactionHash: testutils.RevChainhash(t, txhash822017)[:],
			BlockStatus:     blocktx_api.Status_LONGEST,
		},
	}

	return expectedTxs
}

func verifyBlock(t *testing.T, store *postgresql.PostgreSQL, hashStr string, height uint64, status blocktx_api.Status) {
	hash := testutils.RevChainhash(t, hashStr)
	block, err := store.GetBlock(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, height, block.Height)
	require.Equal(t, status, block.Status)
}

func verifyTxs(t *testing.T, expectedTxs []*blocktx_api.TransactionBlock, publishedTxs []*blocktx_api.TransactionBlock) {
	strippedTxs := make([]*blocktx_api.TransactionBlock, len(publishedTxs))
	for i, tx := range publishedTxs {
		strippedTxs[i] = &blocktx_api.TransactionBlock{
			BlockHash:       tx.BlockHash,
			BlockHeight:     tx.BlockHeight,
			TransactionHash: tx.TransactionHash,
			BlockStatus:     tx.BlockStatus,
		}
	}

	require.ElementsMatch(t, expectedTxs, strippedTxs)
}
