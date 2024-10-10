package integrationtest

// Components of this test:
// 		Postgresql Store - running on docker
// 		Blocktx Processor
// 		PeerHandler - mocked
//
// Flow of this test:
// 		1. A list of blocks from height 822014 to 822017 is added to db from fixtures
// 		2. A hardcoded msg with competing block at height 822015 is being sent through the mocked PeerHandler
// 		3. This block has a chainwork lower than the current tip of chain - becomes STALE
// 		4. Next competing block, at height 822016 is being send through the mocked PeerHandler
// 		5. This block has a greater chainwork than the current tip of longest chain - it becomes LONGEST despite not being the highest
// 		6. Verification of reorg - checking if statuses are correctly switched
//
// Todo: Next tasks:
// 		- Verify that transactions are properly updated in blocktx store
// 		- Include mock metamorph in this test and verify that transactions statuses are properly updated

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	nats_mock "github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core/mocks"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p"
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

func TestReorg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	defer pruneTables(t, dbConn)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var blockRequestCh chan blocktx.BlockRequest = nil // nolint: revive
	blockProcessCh := make(chan *p2p.BlockMessage, 10)

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

	peerHandler := blocktx.NewPeerHandler(logger, blockRequestCh, blockProcessCh)
	processor, err := blocktx.NewProcessor(
		logger,
		blocktxStore,
		blockRequestCh,
		blockProcessCh,
		blocktx.WithMessageQueueClient(mqClient),
	)
	require.NoError(t, err)

	processor.StartBlockProcessing()

	testHandleBlockOnEmptyDatabase(t, peerHandler, blocktxStore)
	publishedTxs = make([]*blocktx_api.TransactionBlock, 0) // clear slice for the next test

	// only load fixtures at this point
	testutils.LoadFixtures(t, dbConn, "fixtures")

	staleBlockHash, expectedTxs := testHandleStaleBlock(t, peerHandler, blocktxStore, publishedTxs)
	// verify the transaction was correctly published to metamorph
	verifyTxs(t, expectedTxs, publishedTxs)
	// clear slice for the next test
	publishedTxs = publishedTxs[:0]

	expectedTxs = testHandleReorg(t, peerHandler, blocktxStore, publishedTxs, staleBlockHash)
	verifyTxs(t, expectedTxs, publishedTxs)
}

func testHandleBlockOnEmptyDatabase(t *testing.T, peerHandler *blocktx.PeerHandler, store *postgresql.PostgreSQL) {
	// test for empty database edge case before inserting fixtures
	prevBlockHash := testutils.RevChainhash(t, "00000000000000000a00c377b260a3219b0c314763f486bc363df7aa7e22ad72")
	txHash, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)
	merkleRoot, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
	require.NoError(t, err)

	// should become LONGEST
	blockMessage := &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash, // NON-existant in the db
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff,
		},
		Height:            uint64(822011),
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	err = peerHandler.HandleBlock(blockMessage, nil)
	require.NoError(t, err)
	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	blockHashZero := blockMessage.Header.BlockHash()

	block, err := store.GetBlock(context.Background(), &blockHashZero)
	require.NoError(t, err)
	require.Equal(t, uint64(822011), block.Height)
	require.Equal(t, blocktx_api.Status_LONGEST, block.Status)
}

func testHandleStaleBlock(t *testing.T, peerHandler *blocktx.PeerHandler, store *postgresql.PostgreSQL, publishedTxs []*blocktx_api.TransactionBlock) (*chainhash.Hash, []*blocktx_api.TransactionBlock) {
	prevBlockHash := testutils.RevChainhash(t, "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000")
	txHash := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
	txHash2 := testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430") // should not be published - is already in the longest chain
	treeStore := bc.BuildMerkleTreeStoreChainHash([]*chainhash.Hash{txHash, txHash2})
	merkleRoot := treeStore[len(treeStore)-1]

	// should become STALE
	blockMessage := &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *prevBlockHash, // block with status LONGEST at height 822014
			MerkleRoot: *merkleRoot,
			Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing block
		},
		Height:            uint64(822015), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash, txHash2},
	}

	err := peerHandler.HandleBlock(blockMessage, nil)
	require.NoError(t, err)
	// Allow DB to process the block
	time.Sleep(200 * time.Millisecond)

	blockHashStale := blockMessage.Header.BlockHash()

	block, err := store.GetBlock(context.Background(), &blockHashStale)
	require.NoError(t, err)
	require.Equal(t, uint64(822015), block.Height)
	require.Equal(t, blocktx_api.Status_STALE, block.Status)

	// transactions expected to be published to metamorph
	expectedTxs := []*blocktx_api.TransactionBlock{
		{
			BlockHash:       blockHashStale[:],
			BlockHeight:     822015,
			TransactionHash: txHash[:],
		},
	}

	return &blockHashStale, expectedTxs
}

func testHandleReorg(t *testing.T, peerHandler *blocktx.PeerHandler, store *postgresql.PostgreSQL, publishedTxs []*blocktx_api.TransactionBlock, staleBlockHash *chainhash.Hash) []*blocktx_api.TransactionBlock {
	txHash := testutils.RevChainhash(t, "2ff4430eb883c6f6c0640a5d716b2d107bbc0efa5aeaa237aec796d4686b0a8f")
	merkleRoot := testutils.RevChainhash(t, "2ff4430eb883c6f6c0640a5d716b2d107bbc0efa5aeaa237aec796d4686b0a8f")

	// should become LONGEST
	// reorg should happen
	blockMessage := &p2p.BlockMessage{
		Header: &wire.BlockHeader{
			Version:    541065216,
			PrevBlock:  *staleBlockHash, // block with status STALE at height 822015
			MerkleRoot: *merkleRoot,
			Bits:       0x1a05db8b, // chainwork: "12301577519373468" higher than the competing block
		},
		Height:            uint64(822016), // competing block already exists at this height
		TransactionHashes: []*chainhash.Hash{txHash},
	}

	err := peerHandler.HandleBlock(blockMessage, nil)
	require.NoError(t, err)
	// Allow DB to process the block and perform reorg
	time.Sleep(1 * time.Second)

	// verify that reorg happened
	blockHashLongest := blockMessage.Header.BlockHash()

	block, err := store.GetBlock(context.Background(), &blockHashLongest)
	require.NoError(t, err)
	require.Equal(t, uint64(822016), block.Height)
	require.Equal(t, blocktx_api.Status_LONGEST, block.Status)

	block, err = store.GetBlock(context.Background(), staleBlockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822015), block.Height)
	require.Equal(t, blocktx_api.Status_LONGEST, block.Status)

	previouslyLongestBlockHash := testutils.RevChainhash(t, "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000")
	block, err = store.GetBlock(context.Background(), previouslyLongestBlockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822015), block.Height)
	require.Equal(t, blocktx_api.Status_STALE, block.Status)

	previouslyLongestBlockHash = testutils.RevChainhash(t, "e1df1273e6e7270f96b508545d7aa80aebda7d758dc82e080000000000000000")
	block, err = store.GetBlock(context.Background(), previouslyLongestBlockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822016), block.Height)
	require.Equal(t, blocktx_api.Status_STALE, block.Status)

	previouslyLongestBlockHash = testutils.RevChainhash(t, "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000")
	block, err = store.GetBlock(context.Background(), previouslyLongestBlockHash)
	require.NoError(t, err)
	require.Equal(t, uint64(822017), block.Height)
	require.Equal(t, blocktx_api.Status_STALE, block.Status)

	beginningOfChain := testutils.RevChainhash(t, "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000")
	block, err = store.GetBlock(context.Background(), beginningOfChain)
	require.NoError(t, err)
	require.Equal(t, uint64(822014), block.Height)
	require.Equal(t, blocktx_api.Status_LONGEST, block.Status)

	expectedTxs := []*blocktx_api.TransactionBlock{
		{
			BlockHash:       staleBlockHash[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")[:],
		},
		{
			BlockHash:       staleBlockHash[:],
			BlockHeight:     822015,
			TransactionHash: testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430")[:],
		},
		// {
		// 	BlockHash:       blockHashLongest[:],
		// 	BlockHeight:     822016,
		// 	TransactionHash: txHash[:],
		// },
	}

	return expectedTxs
}

func verifyTxs(t *testing.T, expectedTxs []*blocktx_api.TransactionBlock, publishedTxs []*blocktx_api.TransactionBlock) {
	strippedTxs := make([]*blocktx_api.TransactionBlock, len(publishedTxs))
	for i, tx := range publishedTxs {
		chash, err := chainhash.NewHash(tx.TransactionHash)
		require.NoError(t, err)
		t.Logf("published tx hash: %s", chash.String())
		strippedTxs[i] = &blocktx_api.TransactionBlock{
			BlockHash:       tx.BlockHash,
			BlockHeight:     tx.BlockHeight,
			TransactionHash: tx.TransactionHash,
			// TODO: add block status
		}
	}

	require.ElementsMatch(t, expectedTxs, strippedTxs)
}
