package postgresql

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/testdata"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
)

type Block struct {
	ID           int64     `db:"id"`
	Hash         string    `db:"hash"`
	PreviousHash string    `db:"prevhash"`
	MerkleRoot   string    `db:"merkleroot"`
	MerklePath   *string   `db:"merkle_path"`
	Height       int64     `db:"height"`
	Orphaned     bool      `db:"orphanedyn"`
	Status       int       `db:"status"`
	Chainwork    string    `db:"chainwork"`
	Size         *int64    `db:"size"`
	TxCount      *int64    `db:"tx_count"`
	Processed    bool      `db:"processed"`
	ProcessedAt  time.Time `db:"processed_at"`
	InsertedAt   time.Time `db:"inserted_at"`
}

type Transaction struct {
	ID           int64     `db:"id"`
	Hash         []byte    `db:"hash"`
	IsRegistered bool      `db:"is_registered"`
	InsertedAt   time.Time `db:"inserted_at"`
}

type BlockTransactionMap struct {
	BlockID       int64     `db:"blockid"`
	TransactionID int64     `db:"txid"`
	MerklePath    string    `db:"merkle_path"`
	InsertedAt    time.Time `db:"inserted_at"`
}

const (
	migrationsPath = "file://migrations"
)

var dbInfo string

func TestMain(m *testing.M) {
	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	port := "5434"
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
	return m.Run()
}

func prepareDb(t *testing.T, db *sql.DB, fixture string) {
	t.Helper()

	testutils.PruneTables(t, db,
		"blocktx.blocks",
		"blocktx.transactions",
		"blocktx.block_transactions_map",
	)

	if fixture != "" {
		testutils.LoadFixtures(t, db, fixture)
	}
}

func setupPostgresTest(t testing.TB) (ctx context.Context, now time.Time, db *PostgreSQL) {
	t.Helper()

	now = time.Date(2023, 12, 22, 12, 0, 0, 0, time.UTC)
	db, err := New(dbInfo, 10, 10, WithNow(func() time.Time {
		return now
	}))
	if err != nil {
		t.Errorf("error setup tests %s", err.Error())
	}

	ctx = context.TODO()

	return
}

func TestPostgresDB(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// common setup for test cases
	ctx, now, postgresDB := setupPostgresTest(t)
	defer postgresDB.Close()

	var err error

	t.Run("insert block / get block", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "")

		blockHash1 := testutils.RevChainhash(t, "000000000000000001b8adefc1eb98896c80e30e517b9e2655f1f929d9958a48")
		blockHash2 := testutils.RevChainhash(t, "00000000000000000a081a539601645abe977946f8f6466a3c9e0c34d50be4a8")
		merkleRoot := testutils.RevChainhash(t, "31e25c5ac7c143687f55fc49caf0f552ba6a16d4f785e4c9a9a842179a085f0c")
		expectedBlock := &blocktx_api.Block{
			Hash:         blockHash2[:],
			PreviousHash: blockHash1[:],
			MerkleRoot:   merkleRoot[:],
			Height:       100,
		}

		// when -> then
		id, err := postgresDB.UpsertBlock(ctx, expectedBlock)
		require.NoError(t, err)
		require.Equal(t, uint64(1), id)

		actualBlockResp, err := postgresDB.GetBlock(ctx, blockHash2)
		require.NoError(t, err)
		require.Equal(t, expectedBlock, actualBlockResp)
	})

	t.Run("get block by height / get chain tip", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/get_block_by_height")

		height := uint64(822015)
		expectedHashAtHeightLongest := testutils.RevChainhash(t, "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000")
		expectedHashAtHeightStale := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")

		heightNotFound := uint64(812222)

		expectedTipHeight := uint64(822020)
		hashAtTip := testutils.RevChainhash(t, "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000")

		// when -> then
		actualBlock, err := postgresDB.GetBlockByHeight(context.Background(), height, blocktx_api.Status_LONGEST)
		require.NoError(t, err)
		require.Equal(t, expectedHashAtHeightLongest[:], actualBlock.Hash)

		actualBlock, err = postgresDB.GetBlockByHeight(context.Background(), height, blocktx_api.Status_STALE)
		require.NoError(t, err)
		require.Equal(t, expectedHashAtHeightStale[:], actualBlock.Hash)

		actualBlock, err = postgresDB.GetBlockByHeight(context.Background(), heightNotFound, blocktx_api.Status_LONGEST)
		require.Nil(t, actualBlock)
		require.Equal(t, store.ErrBlockNotFound, err)

		actualBlock, err = postgresDB.GetChainTip(context.Background())
		require.NoError(t, err)
		require.Equal(t, hashAtTip[:], actualBlock.Hash)
		require.Equal(t, expectedTipHeight, actualBlock.Height)
	})

	t.Run("get block gaps", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/get_block_gaps")

		hash822014 := testutils.RevChainhash(t, "67708796ef57464ed9eaf2a663d3da32372e4c2fb65558020000000000000000")
		hash822019 := testutils.RevChainhash(t, "5696fc6e504b6aa2ae5d9c46b9418192dc61bd1b2e3364030000000000000000")
		hash822020 := testutils.RevChainhash(t, "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000")
		hash822009 := testutils.RevChainhash(t, "4ad773b1a464129a0ed8c7a8c71bb98175f0f01da1793f0e0000000000000000")

		expectedBlockGaps := []*store.BlockGap{
			{ // gap
				Height: 822019,
				Hash:   hash822019,
			},
			{ // gap
				Height: 822014,
				Hash:   hash822014,
			},
			{ // processing not finished
				Height: 822020,
				Hash:   hash822020,
			},
			{ // gap
				Height: 822009,
				Hash:   hash822009,
			},
		}

		// when
		actualBlockGaps, err := postgresDB.GetBlockGaps(ctx, 12)

		// then
		require.NoError(t, err)
		require.Equal(t, 4, len(actualBlockGaps))
		require.ElementsMatch(t, expectedBlockGaps, actualBlockGaps)
	})

	t.Run("get longest chain from height", func(t *testing.T) {
		//given
		prepareDb(t, postgresDB.db, "fixtures/get_longest_chain")

		startingHeight := uint64(822014)
		hash0Longest := testutils.RevChainhash(t, "0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")
		hash1Longest := testutils.RevChainhash(t, "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde")

		expectedStaleHashes := []*chainhash.Hash{
			hash0Longest,
			hash1Longest,
		}

		// when
		longestBlocks, err := postgresDB.GetLongestChainFromHeight(ctx, startingHeight)
		require.NoError(t, err)

		// then
		require.Equal(t, len(expectedStaleHashes), len(longestBlocks))
		for i, b := range longestBlocks {
			require.Equal(t, expectedStaleHashes[i][:], b.Hash)
		}
	})

	t.Run("get stale chain back from hash", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/get_stale_chain")

		hash2Stale := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")
		hash3Stale := testutils.RevChainhash(t, "0000000000000000082ec88d757ddaeb0aa87a5d5408b5960f27e7e67312dfe1")
		hash4Stale := testutils.RevChainhash(t, "000000000000000004bf3e68405b31650559ff28d38a42b5e4f1440a865611ca")

		expectedStaleHashes := [][]byte{
			hash4Stale[:],
			hash3Stale[:],
			hash2Stale[:],
		}

		// when
		actualStaleBlocks, err := postgresDB.GetStaleChainBackFromHash(ctx, hash4Stale[:])
		require.NoError(t, err)

		// then
		require.Equal(t, len(expectedStaleHashes), len(actualStaleBlocks))
		for i, b := range actualStaleBlocks {
			require.Equal(t, expectedStaleHashes[i], b.Hash)
		}
	})

	t.Run("update blocks statuses", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/update_blocks_statuses")

		hash1Longest := testutils.RevChainhash(t, "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde")
		hash2Stale := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")
		hash3Stale := testutils.RevChainhash(t, "0000000000000000082ec88d757ddaeb0aa87a5d5408b5960f27e7e67312dfe1")
		hash4Stale := testutils.RevChainhash(t, "000000000000000004bf3e68405b31650559ff28d38a42b5e4f1440a865611ca")

		blockStatusUpdates := []store.BlockStatusUpdate{
			{Hash: hash1Longest[:], Status: blocktx_api.Status_STALE},
			{Hash: hash2Stale[:], Status: blocktx_api.Status_LONGEST},
			{Hash: hash3Stale[:], Status: blocktx_api.Status_LONGEST},
			{Hash: hash4Stale[:], Status: blocktx_api.Status_LONGEST},
		}

		// when
		err := postgresDB.UpdateBlocksStatuses(ctx, blockStatusUpdates)
		require.NoError(t, err)

		// then
		longest1, err := postgresDB.GetBlock(ctx, hash1Longest)
		require.NoError(t, err)
		require.Equal(t, blocktx_api.Status_STALE, longest1.Status)

		stale2, err := postgresDB.GetBlock(ctx, hash2Stale)
		require.NoError(t, err)
		require.Equal(t, blocktx_api.Status_LONGEST, stale2.Status)

		stale3, err := postgresDB.GetBlock(ctx, hash3Stale)
		require.NoError(t, err)
		require.Equal(t, blocktx_api.Status_LONGEST, stale3.Status)

		stale4, err := postgresDB.GetBlock(ctx, hash4Stale)
		require.NoError(t, err)
		require.Equal(t, blocktx_api.Status_LONGEST, stale4.Status)
	})

	t.Run("test getting mined txs", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/get_mined_transactions")

		txHash1 := testutils.RevChainhash(t, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")
		txHash2 := testutils.RevChainhash(t, "164e85a5d5bc2b2372e8feaa266e5e4b7d0808f8d2b784fb1f7349c4726392b0")
		txHash3 := testutils.RevChainhash(t, "dbbd24251b9bb824566412395bb76a579bca3477c2d0b4cbc210a769d3bb4177")
		txHash4 := testutils.RevChainhash(t, "0d60dd6dc1f2649efb2847f801dfaa61361a438deb526da2de5b6875e0016514")

		blockHash := testutils.RevChainhash(t, "6258b02da70a3e367e4c993b049fa9b76ef8f090ef9fd2010000000000000000")

		// when
		// get mined transaction and corresponding block
		minedTxs, err := postgresDB.GetMinedTransactions(ctx, []*chainhash.Hash{txHash1, txHash2, txHash3, txHash4})
		require.NoError(t, err)

		// then
		require.Len(t, minedTxs, 3)

		for _, tx := range minedTxs {
			require.True(t, bytes.Equal(tx.TxHash, txHash2[:]) || bytes.Equal(tx.TxHash, txHash3[:]) || bytes.Equal(tx.TxHash, txHash4[:]))
			require.Equal(t, tx.BlockHash, blockHash[:])
			require.Equal(t, uint64(826481), tx.BlockHeight)
		}
	})

	t.Run("clear data", func(t *testing.T) {
		prepareDb(t, postgresDB.db, "fixtures/clear_data")

		resp, err := postgresDB.ClearBlocktxTable(context.Background(), 10, "blocks")
		require.NoError(t, err)
		require.Equal(t, int64(1), resp.Rows)

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)
		var blocks []Block

		require.NoError(t, d.Select(&blocks, "SELECT id FROM blocktx.blocks"))

		require.Len(t, blocks, 1)

		resp, err = postgresDB.ClearBlocktxTable(context.Background(), 10, "block_transactions_map")
		require.NoError(t, err)
		require.Equal(t, int64(5), resp.Rows)

		var mps []BlockTransactionMap
		require.NoError(t, d.Select(&mps, "SELECT blockid FROM blocktx.block_transactions_map"))

		require.Len(t, mps, 5)

		resp, err = postgresDB.ClearBlocktxTable(context.Background(), 10, "transactions")
		require.NoError(t, err)
		require.Equal(t, int64(5), resp.Rows)

		var txs []Transaction
		require.NoError(t, d.Select(&txs, "SELECT hash FROM blocktx.transactions"))

		require.Len(t, txs, 5)
	})

	t.Run("set/get/del block processing", func(t *testing.T) {
		prepareDb(t, postgresDB.db, "fixtures/block_processing")

		bh1 := testutils.RevChainhash(t, "747468cf7e6639ba9aa277ade1cf27639b0f214cec5719020000000000000000")

		processedBy, err := postgresDB.SetBlockProcessing(ctx, bh1, "pod-1")
		require.NoError(t, err)
		require.Equal(t, "pod-1", processedBy)

		// set a second time, expect error
		processedBy, err = postgresDB.SetBlockProcessing(ctx, bh1, "pod-1")
		require.ErrorIs(t, err, store.ErrBlockProcessingDuplicateKey)
		require.Equal(t, "pod-1", processedBy)

		bhInProgress := testutils.RevChainhash(t, "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000")

		blockHashes, err := postgresDB.GetBlockHashesProcessingInProgress(ctx, "pod-2")
		require.NoError(t, err)
		require.Len(t, blockHashes, 1)
		require.True(t, bhInProgress.IsEqual(blockHashes[0]))

		_, err = postgresDB.DelBlockProcessing(ctx, bhInProgress, "pod-1")
		require.ErrorIs(t, err, store.ErrBlockNotFound)

		_, err = postgresDB.DelBlockProcessing(ctx, bhInProgress, "pod-2")
		require.NoError(t, err)

		blockHashes, err = postgresDB.GetBlockHashesProcessingInProgress(ctx, "pod-2")
		require.NoError(t, err)
		require.Len(t, blockHashes, 0)
	})

	t.Run("mark block as done", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/mark_block_as_done")

		bh1 := testutils.RevChainhash(t, "b71ab063c5f96cad71cdc59dcc94182a20a69cbd7eed2d070000000000000000")

		// when
		err = postgresDB.MarkBlockAsDone(ctx, bh1, 500, 75)
		require.NoError(t, err)

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)
		var block Block

		require.NoError(t, d.Get(&block, "SELECT * FROM blocktx.blocks WHERE hash=$1", bh1[:]))

		// then
		require.NotNil(t, block.Size)
		require.Equal(t, int64(500), *block.Size)

		require.NotNil(t, block.TxCount)
		require.Equal(t, int64(75), *block.TxCount)
		require.Equal(t, now.UTC(), block.ProcessedAt.UTC())
	})

	t.Run("verify merkle roots", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB.db, "fixtures/verify_merkle_roots")

		merkleRequests := []*blocktx_api.MerkleRootVerificationRequest{
			{
				// correct merkleroot - should not return its height
				MerkleRoot:  "80d8a6a8306626fa25bd7e1bdae587805fc1f7d41e8aba0353488f8094153d4f",
				BlockHeight: 812010,
			},
			{
				// correct merkleroot but incorrect height - should return its height
				MerkleRoot:  "80d8a6a8306626fa25bd7e1bdae587805fc1f7d41e8aba0353488f8094153d4f",
				BlockHeight: 812011,
			},
			{
				// incorrect merkleroot - should return its height
				MerkleRoot:  "incorrect_merkle_root",
				BlockHeight: 822010,
			},
			{
				// merkleroot below lowest height - should not return its height
				MerkleRoot:  "fc377968f64fdcb6386ca7fcb6e6f8693a988663a07f552269823099909ea790",
				BlockHeight: 800000,
			},
			{
				// merkleroot above top height, but within limits - should not return its height
				MerkleRoot:  "fc377968f64fdcb6386ca7fcb6e6f8693a988663a07f552269823099909ea790",
				BlockHeight: 822030,
			},
			{
				// merkleroot above top height and above limits - should return its height
				MerkleRoot:  "fc377968f64fdcb6386ca7fcb6e6f8693a988663a07f552269823099909ea790",
				BlockHeight: 822032,
			},
		}
		maxAllowedBlockHeightMismatch := 10
		expectedUnverifiedBlockHeights := []uint64{812011, 822010, 822032}

		// when
		res, err := postgresDB.VerifyMerkleRoots(ctx, merkleRequests, maxAllowedBlockHeightMismatch)
		require.NoError(t, err)

		// then
		assert.Equal(t, expectedUnverifiedBlockHeights, res.UnverifiedBlockHeights)
	})
}

func TestPostgresStore_UpsertBlockTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tcs := []struct {
		name               string
		txsWithMerklePaths []store.TxWithMerklePath

		expectedErr           error
		expectedUpdatedResLen int
	}{
		{
			name: "upsert all registered transactions (updates only)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:       testutils.RevChainhash(t, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")[:],
					MerklePath: "test1",
				},
				{
					Hash:       testutils.RevChainhash(t, "164e85a5d5bc2b2372e8feaa266e5e4b7d0808f8d2b784fb1f7349c4726392b0")[:],
					MerklePath: "test2",
				},
			},
			expectedUpdatedResLen: 2,
		},
		{
			name: "upsert all non-registered transactions (inserts only)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:       testutils.RevChainhash(t, "edd33fdcdfa68444d227780e2b62a4437c00120c5320d2026aeb24a781f4c3f1")[:],
					MerklePath: "test1",
				},
			},
			expectedUpdatedResLen: 0,
		},
		{
			name: "update exceeds max batch size (more txs than 5)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:       testutils.RevChainhash(t, "b4201cc6fc5768abff14adf75042ace6061da9176ee5bb943291b9ba7d7f5743")[:],
					MerklePath: "test1",
				},
				{
					Hash:       testutils.RevChainhash(t, "37bd6c87927e75faeb3b3c939f64721cda48e1bb98742676eebe83aceee1a669")[:],
					MerklePath: "test2",
				},
				{
					Hash:       testutils.RevChainhash(t, "952f80e20a0330f3b9c2dfd1586960064e797218b5c5df665cada221452c17eb")[:],
					MerklePath: "test3",
				},
				{
					Hash:       testutils.RevChainhash(t, "861a281b27de016e50887288de87eab5ca56a1bb172cdff6dba965474ce0f608")[:],
					MerklePath: "test4",
				},
				{
					Hash:       testutils.RevChainhash(t, "9421cc760c5405af950a76dc3e4345eaefd4e7322f172a3aee5e0ddc7b4f8313")[:],
					MerklePath: "test5",
				},
				{
					Hash:       testutils.RevChainhash(t, "8b7d038db4518ac4c665abfc5aeaacbd2124ad8ca70daa8465ed2c4427c41b9b")[:],
					MerklePath: "test6",
				},
			},
			expectedUpdatedResLen: 6,
		},
	}

	// common setup for test cases
	ctx, _, sut := setupPostgresTest(t)
	defer sut.Close()
	sut.maxPostgresBulkInsertRows = 5

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prepareDb(t, sut.db, "fixtures/upsert_block_transactions")

			testBlockID := uint64(9736)

			// when
			res, err := sut.UpsertBlockTransactions(ctx, testBlockID, tc.txsWithMerklePaths)

			// then
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tc.expectedUpdatedResLen, len(res))

			// assert correctness of returned values
			// assume registered transactions are at the beginning of tc.txs
			for i := 0; i < tc.expectedUpdatedResLen; i++ {
				require.True(t, bytes.Equal(tc.txsWithMerklePaths[i].Hash, res[i].Hash))
				require.Equal(t, tc.txsWithMerklePaths[i].MerklePath, res[i].MerklePath)
			}

			// assert data are correctly saved in the store
			d, err := sqlx.Open("postgres", dbInfo)
			require.NoError(t, err)

			for i, tx := range tc.txsWithMerklePaths {
				var storedtx Transaction

				err = d.Get(&storedtx, "SELECT id, hash, is_registered from blocktx.transactions WHERE hash=$1", tx.Hash[:])
				require.NoError(t, err, "error during getting transaction")

				require.Equal(t, i < tc.expectedUpdatedResLen, storedtx.IsRegistered)

				var mp BlockTransactionMap
				err = d.Get(&mp, "SELECT blockid, txid, merkle_path from blocktx.block_transactions_map WHERE txid=$1", storedtx.ID)
				require.NoError(t, err, "error during getting block transactions map")

				require.Equal(t, tx.MerklePath, mp.MerklePath)
				require.Equal(t, testBlockID, uint64(mp.BlockID))
			}
		})
	}
}

func TestPostgresStore_UpsertBlockTransactions_CompetingBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// given
	ctx, _, sut := setupPostgresTest(t)
	defer sut.Close()
	sut.maxPostgresBulkInsertRows = 5

	prepareDb(t, sut.db, "fixtures/upsert_block_transactions")

	testBlockID := uint64(9736)
	competingBlockID := uint64(9737)

	txHash := testutils.RevChainhash(t, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")

	txsWithMerklePaths := []store.TxWithMerklePath{
		{
			Hash:       txHash[:],
			MerklePath: "merkle-path-1",
		},
	}

	competingTxsWithMerklePaths := []store.TxWithMerklePath{
		{
			Hash:       txHash[:],
			MerklePath: "merkle-path-2",
		},
	}

	expected := []store.GetMinedTransactionResult{
		{
			TxHash:      txHash[:],
			BlockHash:   testutils.RevChainhash(t, "6258b02da70a3e367e4c993b049fa9b76ef8f090ef9fd2010000000000000000")[:],
			BlockHeight: uint64(826481),
			MerklePath:  "merkle-path-1",
		},
		{
			TxHash:      txHash[:],
			BlockHash:   testutils.RevChainhash(t, "7258b02da70a3e367e4c993b049fa9b76ef8f090ef9fd2010000000000000000")[:],
			BlockHeight: uint64(826481),
			MerklePath:  "merkle-path-2",
		},
	}

	// when
	_, err := sut.UpsertBlockTransactions(ctx, testBlockID, txsWithMerklePaths)
	require.NoError(t, err)

	_, err = sut.UpsertBlockTransactions(ctx, competingBlockID, competingTxsWithMerklePaths)
	require.NoError(t, err)

	// then
	actual, err := sut.GetMinedTransactions(ctx, []*chainhash.Hash{txHash})
	require.NoError(t, err)

	require.ElementsMatch(t, expected, actual)
}

func TestPostgresStore_RegisterTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tcs := []struct {
		name string
		txs  [][]byte
	}{
		{
			name: "register new transactions",
			txs: [][]byte{
				testdata.TX1Hash[:],
				testdata.TX2Hash[:],
				testdata.TX3Hash[:],
				testdata.TX4Hash[:],
			},
		},
		{
			name: "register already known, not registered transactions",
			txs: [][]byte{
				testutils.RevChainhash(t, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")[:],
				testutils.RevChainhash(t, "164e85a5d5bc2b2372e8feaa266e5e4b7d0808f8d2b784fb1f7349c4726392b0")[:],
				testutils.RevChainhash(t, "8b7d038db4518ac4c665abfc5aeaacbd2124ad8ca70daa8465ed2c4427c41b9b")[:],
				testutils.RevChainhash(t, "9421cc760c5405af950a76dc3e4345eaefd4e7322f172a3aee5e0ddc7b4f8313")[:],
			},
		},
		{
			name: "register already registered transactions",
			txs: [][]byte{
				testutils.RevChainhash(t, "b4201cc6fc5768abff14adf75042ace6061da9176ee5bb943291b9ba7d7f5743")[:],
				testutils.RevChainhash(t, "37bd6c87927e75faeb3b3c939f64721cda48e1bb98742676eebe83aceee1a669")[:],
				testutils.RevChainhash(t, "952f80e20a0330f3b9c2dfd1586960064e797218b5c5df665cada221452c17eb")[:],
				testutils.RevChainhash(t, "861a281b27de016e50887288de87eab5ca56a1bb172cdff6dba965474ce0f608")[:],
			},
		},
	}

	// common setup for test cases
	ctx, now, sut := setupPostgresTest(t)
	defer sut.Close()

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prepareDb(t, sut.db, "fixtures/register_transactions")

			// when
			result, err := sut.RegisterTransactions(ctx, tc.txs)
			require.NoError(t, err)
			require.NotNil(t, result)

			resultmap := make(map[chainhash.Hash]bool)
			for _, h := range result {
				resultmap[*h] = false
			}

			// then
			// assert data are correctly saved in the store
			d, err := sqlx.Open("postgres", dbInfo)
			require.NoError(t, err)

			updatedCounter := 0
			for _, hash := range tc.txs {
				var storedtx Transaction
				err = d.Get(&storedtx, "SELECT hash, is_registered from blocktx.transactions WHERE hash=$1", hash)
				require.NoError(t, err)

				require.NotNil(t, storedtx)
				require.True(t, storedtx.IsRegistered)

				if _, found := resultmap[chainhash.Hash(storedtx.Hash)]; found {
					require.Greater(t, storedtx.InsertedAt, now)
					updatedCounter++
				} else {
					require.Less(t, storedtx.InsertedAt, now)
				}
			}

			require.Equal(t, len(result), updatedCounter)
		})
	}
}
