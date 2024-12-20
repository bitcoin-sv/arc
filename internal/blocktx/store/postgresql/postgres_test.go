package postgresql

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/testdata"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"

	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
)

type Block struct {
	ID           int64     `db:"id"`
	Hash         string    `db:"hash"`
	PreviousHash string    `db:"prevhash"`
	MerkleRoot   string    `db:"merkleroot"`
	MerklePath   *string   `db:"merkle_path"`
	Height       int64     `db:"height"`
	Status       int       `db:"status"`
	Chainwork    string    `db:"chainwork"`
	IsLongest    bool      `db:"is_longest"`
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
	BlockID         int64     `db:"blockid"`
	TransctionHash  []byte    `db:"txhash"`
	MerklePath      string    `db:"merkle_path"`
	MerkleTreeIndex int64     `db:"merkle_tree_index"`
	InsertedAt      time.Time `db:"inserted_at"`
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

func prepareDb(t testing.TB, postgres *PostgreSQL, fixture string) {
	t.Helper()

	testutils.PruneTables(t, postgres.db,
		"blocktx.blocks",
		"blocktx.transactions",
		"blocktx.block_transactions_map",
	)

	if fixture != "" {
		testutils.LoadFixtures(t, postgres.db, fixture)
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

	t.Run("upsert block / get block", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "")

		blockHash1 := testutils.RevChainhash(t, "000000000000000001b8adefc1eb98896c80e30e517b9e2655f1f929d9958a48")
		blockHash2 := testutils.RevChainhash(t, "00000000000000000a081a539601645abe977946f8f6466a3c9e0c34d50be4a8")
		// TODO: uncomment when all block gaps are filled
		// blockHashViolating := testutils.RevChainhash(t, "00000000b69bd8e4dc60580117617a466d5c76ada85fb7b87e9baea01f9d9984")
		merkleRoot := testutils.RevChainhash(t, "31e25c5ac7c143687f55fc49caf0f552ba6a16d4f785e4c9a9a842179a085f0c")
		expectedBlock := &blocktx_api.Block{
			Hash:         blockHash2[:],
			PreviousHash: blockHash1[:],
			MerkleRoot:   merkleRoot[:],
			Height:       100,
			Status:       blocktx_api.Status_LONGEST,
			Processed:    true,
		}
		// TODO: uncomment when all block gaps are filled
		// expectedBlockViolatingUniqueIndex := &blocktx_api.Block{
		// 	Hash:         blockHashViolating[:],
		// 	PreviousHash: blockHash1[:],
		// 	MerkleRoot:   merkleRoot[:],
		// 	Height:       100,
		// 	Status:       blocktx_api.Status_LONGEST,
		// }
		expectedBlockOverrideStatus := &blocktx_api.Block{
			Hash:         blockHash2[:],
			PreviousHash: blockHash1[:],
			MerkleRoot:   merkleRoot[:],
			Height:       100,
			Status:       blocktx_api.Status_ORPHANED,
			Processed:    true,
		}

		// when
		id, err := postgresDB.UpsertBlock(ctx, expectedBlock)
		require.NoError(t, err)
		require.Equal(t, uint64(1), id)

		err = postgresDB.MarkBlockAsDone(ctx, blockHash2, uint64(1000), uint64(1))
		require.NoError(t, err)

		// then
		actualBlockResp, err := postgresDB.GetBlock(ctx, blockHash2)
		require.NoError(t, err)
		require.Equal(t, expectedBlock, actualBlockResp)

		// TODO: uncomment when all block gaps are filled
		// when
		// _, err = postgresDB.UpsertBlock(ctx, expectedBlockViolatingUniqueIndex)
		// then
		// require.ErrorIs(t, err, store.ErrFailedToInsertBlock)

		// when
		id, err = postgresDB.UpsertBlock(ctx, expectedBlockOverrideStatus)
		require.NoError(t, err)
		require.Equal(t, uint64(1), id) // this should only update the status and retain the same ID

		// then
		actualBlockResp, err = postgresDB.GetBlock(ctx, blockHash2)
		require.NoError(t, err)
		require.Equal(t, expectedBlockOverrideStatus, actualBlockResp)
	})

	t.Run("get block by height / get chain tip", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_block_by_height")

		height := uint64(822015)
		expectedHashAtHeightLongest := testutils.RevChainhash(t, "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000")

		heightNotFound := uint64(812222)

		expectedTipHeight := uint64(822020)
		hashAtTip := testutils.RevChainhash(t, "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000")

		// when -> then
		actualBlock, err := postgresDB.GetLongestBlockByHeight(context.Background(), height)
		require.NoError(t, err)
		require.Equal(t, expectedHashAtHeightLongest[:], actualBlock.Hash)

		actualBlock, err = postgresDB.GetLongestBlockByHeight(context.Background(), heightNotFound)
		require.Nil(t, actualBlock)
		require.Equal(t, store.ErrBlockNotFound, err)

		actualBlock, err = postgresDB.GetChainTip(context.Background())
		require.NoError(t, err)
		require.Equal(t, hashAtTip[:], actualBlock.Hash)
		require.Equal(t, expectedTipHeight, actualBlock.Height)
	})

	t.Run("get block gaps", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_block_gaps")

		hash822014 := testutils.RevChainhash(t, "67708796ef57464ed9eaf2a663d3da32372e4c2fb65558020000000000000000")
		hash822019 := testutils.RevChainhash(t, "5696fc6e504b6aa2ae5d9c46b9418192dc61bd1b2e3364030000000000000000")
		hash822020 := testutils.RevChainhash(t, "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000")
		hash822009 := testutils.RevChainhash(t, "4ad773b1a464129a0ed8c7a8c71bb98175f0f01da1793f0e0000000000000000")
		hash822017competing := testutils.RevChainhash(t, "00000000000000000d840fb91c0df3b057db04a0250c6d88b2f25aadcfc8410b")

		expectedBlockGaps := []*store.BlockGap{
			{ // gap
				Height: 822009,
				Hash:   hash822009,
			},
			// block 11 is being processed
			{ // gap
				Height: 822014,
				Hash:   hash822014,
			},
			{ // gap from competing chain
				Height: 822017,
				Hash:   hash822017competing,
			},
			{ // gap
				Height: 822019,
				Hash:   hash822019,
			},
			{ // processing not finished
				Height: 822020,
				Hash:   hash822020,
			},
		}

		// when
		actualBlockGaps, err := postgresDB.GetBlockGaps(ctx, 12)

		// then
		require.NoError(t, err)
		require.Equal(t, len(expectedBlockGaps), len(actualBlockGaps))
		require.ElementsMatch(t, expectedBlockGaps, actualBlockGaps)
	})

	t.Run("get stats for block geps", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_block_gaps")
		const expectedGaps = int64(2007) // range (2016) - already existing blocks (9) = gaps (2007)

		// when
		actualBlockGaps, err := postgresDB.GetStats(ctx)

		// then
		require.NoError(t, err)
		// as we have 6 blocks in the range only it should return remaining 2006 (BlockDistance) - 6
		require.Equal(t, expectedGaps, actualBlockGaps.CurrentNumOfBlockGaps)
	})

	t.Run("get longest chain from height", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_longest_chain")

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
		prepareDb(t, postgresDB, "fixtures/get_stale_chain")

		hash2Stale := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")
		hash3Stale := testutils.RevChainhash(t, "0000000000000000082ec88d757ddaeb0aa87a5d5408b5960f27e7e67312dfe1")
		hash4Stale := testutils.RevChainhash(t, "000000000000000004bf3e68405b31650559ff28d38a42b5e4f1440a865611ca")

		expectedStaleHashes := [][]byte{
			hash2Stale[:],
			hash3Stale[:],
			hash4Stale[:],
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

	t.Run("get orphans back to non-orphaned ancestor", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_orphaned_chain")

		newHash := testutils.RevChainhash(t, "00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656")
		hash2Orphaned := testutils.RevChainhash(t, "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde")
		hash3Orphaned := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")
		hash4Orphaned := testutils.RevChainhash(t, "0000000000000000082ec88d757ddaeb0aa87a5d5408b5960f27e7e67312dfe1")

		noAncestorHash := testutils.RevChainhash(t, "0000000000000000082131979a4e25a5101912a5f8461e18f306d23e158161cd")
		hash6Orphaned := testutils.RevChainhash(t, "0000000000000000059d6add76e3ddb8ec4f5ffd6efecd4c8b8c577bd32aed6c")

		expectedOrphanedHashes := [][]byte{
			hash2Orphaned[:],
			hash3Orphaned[:],
			hash4Orphaned[:],
			newHash[:],
		}
		expectedAncestorHash := testutils.RevChainhash(t, "0000000000000000025855b62f4c2e3732dad363a6f2ead94e4657ef96877067")

		expectedNoAncestorOrphanedHashes := [][]byte{
			noAncestorHash[:],
			hash6Orphaned[:],
		}

		// when
		actualOrphanedBlocks, actualAncestor, err := postgresDB.GetOrphansBackToNonOrphanAncestor(ctx, newHash[:])
		require.NoError(t, err)

		// then
		require.Equal(t, len(expectedOrphanedHashes), len(actualOrphanedBlocks))
		for i, b := range actualOrphanedBlocks {
			require.Equal(t, expectedOrphanedHashes[i], b.Hash)
		}
		require.Equal(t, expectedAncestorHash[:], actualAncestor.Hash)

		// when
		actualOrphanedBlocks, actualAncestor, err = postgresDB.GetOrphansBackToNonOrphanAncestor(ctx, noAncestorHash[:])
		require.NoError(t, err)

		// then
		require.Equal(t, len(expectedNoAncestorOrphanedHashes), len(actualOrphanedBlocks))
		require.Nil(t, actualAncestor)
	})

	t.Run("update blocks statuses", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/update_blocks_statuses")

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

		// TODO: uncomment when all block gaps are filled
		// blockStatusUpdatesViolating := []store.BlockStatusUpdate{
		// 	// there is already a LONGEST block at that height
		// 	{Hash: hash1Longest[:], Status: blocktx_api.Status_LONGEST},
		// }

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

		// TODO: uncomment when all block gaps are filled
		// when
		// err = postgresDB.UpdateBlocksStatuses(ctx, blockStatusUpdatesViolating)
		// require.ErrorIs(t, err, store.ErrFailedToUpdateBlockStatuses)
	})

	t.Run("get mined txs", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_transactions")

		txHash1 := testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853")
		txHash2 := testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		txHash3 := testutils.RevChainhash(t, "213a8c87c5460e82b5ae529212956b853c7ce6bf06e56b2e040eb063cf9a49f0") // from STALE block

		blockHash := testutils.RevChainhash(t, "000000000000000005aa39a25e7e8bf440c270ec9a1bd30e99ab026f39207ef9")
		blockHash2 := testutils.RevChainhash(t, "0000000000000000072ded7ebd9ca6202a1894cc9dc5cd71ad6cf9c563b01ab7")

		expectedTxs := []store.TransactionBlock{
			{
				TxHash:          txHash1[:],
				BlockHash:       blockHash[:],
				BlockHeight:     822013,
				MerklePath:      "merkle-path-1",
				MerkleTreeIndex: int64(1),
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				TxHash:          txHash2[:],
				BlockHash:       blockHash[:],
				BlockHeight:     822013,
				MerklePath:      "merkle-path-2",
				MerkleTreeIndex: int64(2),
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				TxHash:          txHash3[:],
				BlockHash:       blockHash2[:],
				BlockHeight:     822012,
				MerklePath:      "merkle-path-6",
				MerkleTreeIndex: int64(6),
				BlockStatus:     blocktx_api.Status_STALE,
			},
		}

		// when
		onlyLongestChain := true
		actualTxs, err := postgresDB.GetMinedTransactions(ctx, [][]byte{txHash1[:], txHash2[:], txHash3[:]}, onlyLongestChain)

		// then
		require.NoError(t, err)
		require.ElementsMatch(t, expectedTxs[:2], actualTxs)

		// when
		onlyLongestChain = false
		actualTxs, err = postgresDB.GetMinedTransactions(ctx, [][]byte{txHash1[:], txHash2[:], txHash3[:]}, onlyLongestChain)

		// then
		require.NoError(t, err)
		require.Equal(t, expectedTxs, actualTxs)
	})

	t.Run("get registered txs by block hashes", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_transactions")

		blockHashLongest := testutils.RevChainhash(t, "000000000000000005aa39a25e7e8bf440c270ec9a1bd30e99ab026f39207ef9")
		blockHashStale := testutils.RevChainhash(t, "0000000000000000072ded7ebd9ca6202a1894cc9dc5cd71ad6cf9c563b01ab7")

		blockHashes := [][]byte{
			blockHashLongest[:],
			blockHashStale[:],
		}

		expectedTxs := []store.TransactionBlock{
			{
				TxHash:          testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")[:],
				BlockHash:       blockHashLongest[:],
				BlockHeight:     822013,
				MerklePath:      "merkle-path-2",
				MerkleTreeIndex: int64(2),
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{
				TxHash:          testutils.RevChainhash(t, "213a8c87c5460e82b5ae529212956b853c7ce6bf06e56b2e040eb063cf9a49f0")[:],
				BlockHash:       blockHashStale[:],
				BlockHeight:     822012,
				MerklePath:      "merkle-path-6",
				MerkleTreeIndex: int64(6),
				BlockStatus:     blocktx_api.Status_STALE,
			},
			{
				TxHash:          testutils.RevChainhash(t, "12c04cfc5643f1cd25639ad42d6f8f0489557699d92071d7e0a5b940438c4357")[:],
				BlockHash:       blockHashStale[:],
				BlockHeight:     822012,
				MerklePath:      "merkle-path-7",
				MerkleTreeIndex: int64(7),
				BlockStatus:     blocktx_api.Status_STALE,
			},
		}

		// when
		actualTxs, err := postgresDB.GetRegisteredTxsByBlockHashes(ctx, blockHashes)

		// then
		require.NoError(t, err)
		require.Equal(t, expectedTxs, actualTxs)
	})

	t.Run("get unmined registered txs", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_unmined_registered_txs")

		expectedTxs := [][]byte{
			testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430")[:],
			testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")[:],
		}

		// when
		txHashes, err := postgresDB.GetUnminedRegisteredTxsHashes(ctx)
		require.NoError(t, err)

		// then
		require.ElementsMatch(t, expectedTxs, txHashes)
	})

	t.Run("get block transactions hashes", func(t *testing.T) {
		// given
		prepareDb(t, postgresDB, "fixtures/get_block_transactions")

		blockHash := testutils.RevChainhash(t, "000000000000000005aa39a25e7e8bf440c270ec9a1bd30e99ab026f39207ef9")[:]

		expectedTxsHashes := []*chainhash.Hash{
			testutils.RevChainhash(t, "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"),
			testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e"),
			testutils.RevChainhash(t, "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"),
			testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa"),
			testutils.RevChainhash(t, "3e0b5b218c344110f09bf485bc58de4ea5378e55744185edf9c1dafa40068ecd"),
		}

		// when
		actualTxsHashes, err := postgresDB.GetBlockTransactionsHashes(ctx, blockHash)
		require.NoError(t, err)

		// then
		require.ElementsMatch(t, expectedTxsHashes, actualTxsHashes)
	})

	t.Run("clear data", func(t *testing.T) {
		prepareDb(t, postgresDB, "fixtures/clear_data")

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
		prepareDb(t, postgresDB, "fixtures/block_processing")

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
		prepareDb(t, postgresDB, "fixtures/mark_block_as_done")

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
		prepareDb(t, postgresDB, "fixtures/verify_merkle_roots")

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

		expectedUpdatedResLen int
		upsertRepeat          bool
	}{
		{
			name: "upsert all registered transactions (updates only)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:            testutils.RevChainhash(t, "76732b80598326a18d3bf0a86518adbdf95d0ddc6ff6693004440f4776168c3b")[:],
					MerklePath:      "test1",
					MerkleTreeIndex: int64(1),
				},
				{
					Hash:            testutils.RevChainhash(t, "164e85a5d5bc2b2372e8feaa266e5e4b7d0808f8d2b784fb1f7349c4726392b0")[:],
					MerklePath:      "test2",
					MerkleTreeIndex: int64(2),
				},
			},
			expectedUpdatedResLen: 2,
		},
		{
			name: "upsert all non-registered transactions (inserts only)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:            testutils.RevChainhash(t, "edd33fdcdfa68444d227780e2b62a4437c00120c5320d2026aeb24a781f4c3f1")[:],
					MerklePath:      "test1",
					MerkleTreeIndex: int64(1),
				},
			},
			expectedUpdatedResLen: 0,
		},
		{
			name: "update exceeds max batch size (more txs than 5)",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:            testutils.RevChainhash(t, "b4201cc6fc5768abff14adf75042ace6061da9176ee5bb943291b9ba7d7f5743")[:],
					MerklePath:      "test1",
					MerkleTreeIndex: int64(1),
				},
				{
					Hash:            testutils.RevChainhash(t, "37bd6c87927e75faeb3b3c939f64721cda48e1bb98742676eebe83aceee1a669")[:],
					MerklePath:      "test2",
					MerkleTreeIndex: int64(2),
				},
				{
					Hash:            testutils.RevChainhash(t, "952f80e20a0330f3b9c2dfd1586960064e797218b5c5df665cada221452c17eb")[:],
					MerklePath:      "test3",
					MerkleTreeIndex: int64(3),
				},
				{
					Hash:            testutils.RevChainhash(t, "861a281b27de016e50887288de87eab5ca56a1bb172cdff6dba965474ce0f608")[:],
					MerklePath:      "test4",
					MerkleTreeIndex: int64(4),
				},
				{
					Hash:            testutils.RevChainhash(t, "9421cc760c5405af950a76dc3e4345eaefd4e7322f172a3aee5e0ddc7b4f8313")[:],
					MerklePath:      "test5",
					MerkleTreeIndex: int64(5),
				},
				{
					Hash:            testutils.RevChainhash(t, "8b7d038db4518ac4c665abfc5aeaacbd2124ad8ca70daa8465ed2c4427c41b9b")[:],
					MerklePath:      "test6",
					MerkleTreeIndex: int64(6),
				},
			},
			expectedUpdatedResLen: 6,
		},
		{
			name: "upsert all registered transactions cause conflict with txid and blockid",
			txsWithMerklePaths: []store.TxWithMerklePath{
				{
					Hash:            testutils.RevChainhash(t, "8b7d038db4518ac4c665abfc5aeaacbd2124ad8ca70daa8465ed2c4427c41b9b")[:],
					MerklePath:      "test7",
					MerkleTreeIndex: int64(7),
				},
			},
			upsertRepeat:          true,
			expectedUpdatedResLen: 1,
		},
	}

	// common setup for test cases
	ctx, _, sut := setupPostgresTest(t)
	defer sut.Close()
	sut.maxPostgresBulkInsertRows = 5

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prepareDb(t, sut, "fixtures/upsert_block_transactions")

			testBlockID := uint64(9736)
			testBlockHash := testutils.RevChainhash(t, "6258b02da70a3e367e4c993b049fa9b76ef8f090ef9fd2010000000000000000")

			// when
			err := sut.UpsertBlockTransactions(ctx, testBlockID, tc.txsWithMerklePaths)
			if tc.upsertRepeat {
				err = sut.UpsertBlockTransactions(ctx, testBlockID, tc.txsWithMerklePaths)
				require.NoError(t, err)
			}

			// then
			require.NoError(t, err)

			res, err := sut.GetRegisteredTxsByBlockHashes(ctx, [][]byte{testBlockHash[:]})
			require.NoError(t, err)

			require.Equal(t, tc.expectedUpdatedResLen, len(res))

			// assert correctness of returned values
			// assume registered transactions are at the beginning of tc.txs
			for i := 0; i < tc.expectedUpdatedResLen; i++ {
				require.True(t, bytes.Equal(tc.txsWithMerklePaths[i].Hash, res[i].TxHash))
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
				err = d.Get(&mp, "SELECT blockid, txhash, merkle_path, merkle_tree_index from blocktx.block_transactions_map WHERE txhash=$1", storedtx.Hash)
				require.NoError(t, err, "error during getting block transactions map")

				require.Equal(t, tx.MerklePath, mp.MerklePath)
				require.Equal(t, tx.MerkleTreeIndex, mp.MerkleTreeIndex)
				require.Equal(t, testBlockID, uint64(mp.BlockID))
			}
		})
	}
}

func NewHash(length int) ([]byte, error) {
	hash := make([]byte, length)
	_, err := rand.Read(hash)
	if err != nil {
		return nil, err
	}

	return hash, nil
}

func BenchmarkUpsertBlockTransactions(b *testing.B) {
	ctx, _, sut := setupPostgresTest(b)
	defer sut.Close()

	const totalRows = 80000
	tt := []struct {
		name       string
		batch      int
		iterations int
	}{
		{
			name:       "UpsertBlockTransactions - 80000, 1 batch",
			batch:      totalRows,
			iterations: 1,
		},
		{
			name:       "UpsertBlockTransactions - 20000, 4 batches",
			batch:      20000,
			iterations: 4,
		},
		{
			name:       "UpsertBlockTransactions - 10000, 8 batches",
			batch:      10000,
			iterations: 8,
		},
		{
			name:       "UpsertBlockTransactions - 8000, 10 batches",
			batch:      8000,
			iterations: 10,
		},
		{
			name:       "UpsertBlockTransactions - 5000, 16 batches",
			batch:      5000,
			iterations: 16,
		},
		{
			name:       "UpsertBlockTransactions - 2000, 40 batches",
			batch:      2000,
			iterations: 40,
		},
	}

	for _, tc := range tt {
		b.Run(tc.name, func(b *testing.B) {
			b.StopTimer()

			testBlockID := uint64(9736)
			txsWithMerklePaths := make([]store.TxWithMerklePath, totalRows)

			counter := 0
			for range totalRows {
				hash, err := NewHash(32)
				require.NoError(b, err)
				merklePath, err := NewHash(100)
				require.NoError(b, err)
				hex.EncodeToString(merklePath)

				txsWithMerklePaths[counter] = store.TxWithMerklePath{
					Hash:       hash,
					MerklePath: hex.EncodeToString(merklePath),
				}

				counter++
			}

			b.StartTimer()

			for i := 0; i < tc.iterations; i++ {
				err := sut.UpsertBlockTransactions(ctx, testBlockID, txsWithMerklePaths[i*tc.batch:(i+1)*tc.batch-1])
				require.NoError(b, err)
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

	prepareDb(t, sut, "fixtures/upsert_block_transactions")

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

	expected := []store.TransactionBlock{
		{
			TxHash:      txHash[:],
			BlockHash:   testutils.RevChainhash(t, "6258b02da70a3e367e4c993b049fa9b76ef8f090ef9fd2010000000000000000")[:],
			BlockHeight: uint64(826481),
			MerklePath:  "merkle-path-1",
			BlockStatus: blocktx_api.Status_LONGEST,
		},
	}

	// when
	err := sut.UpsertBlockTransactions(ctx, testBlockID, txsWithMerklePaths)
	require.NoError(t, err)

	err = sut.UpsertBlockTransactions(ctx, competingBlockID, competingTxsWithMerklePaths)
	require.NoError(t, err)

	// then
	actual, err := sut.GetMinedTransactions(ctx, [][]byte{txHash[:]}, true)
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
			prepareDb(t, sut, "fixtures/register_transactions")

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

func TestUpsertBlockConditions(t *testing.T) {
	tt := []struct {
		name            string
		blockStatus     blocktx_api.Status
		prevBlockExists bool
		prevBlockStatus blocktx_api.Status

		shouldSucceed bool
	}{
		{
			name:            "extend longest chain - success",
			blockStatus:     blocktx_api.Status_LONGEST,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_LONGEST,
			shouldSucceed:   true,
		},
		{
			name:            "extend stale chain - sucsess",
			blockStatus:     blocktx_api.Status_STALE,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_STALE,
			shouldSucceed:   true,
		},
		{
			name:            "extend orphaned chain - success",
			blockStatus:     blocktx_api.Status_ORPHANED,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_ORPHANED,
			shouldSucceed:   true,
		},
		{
			name:            "stale block extends longest - success",
			blockStatus:     blocktx_api.Status_STALE,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_LONGEST,
			shouldSucceed:   true,
		},
		{
			name:            "orphan block - success",
			blockStatus:     blocktx_api.Status_ORPHANED,
			prevBlockExists: false,
			shouldSucceed:   true,
		},
		{
			name:            "stale block with no prevBlock - fail",
			blockStatus:     blocktx_api.Status_STALE,
			prevBlockExists: false,
			shouldSucceed:   false,
		},
		{
			name:            "orphan block extending longest chain - fail",
			blockStatus:     blocktx_api.Status_ORPHANED,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_LONGEST,
			shouldSucceed:   false,
		},
		{
			name:            "orphan block extending stale chain - fail",
			blockStatus:     blocktx_api.Status_ORPHANED,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_STALE,
			shouldSucceed:   false,
		},
		{
			name:            "longest block extending stale chain - fail",
			blockStatus:     blocktx_api.Status_LONGEST,
			prevBlockExists: true,
			prevBlockStatus: blocktx_api.Status_STALE,
			shouldSucceed:   false,
		},
	}

	// common setup for test cases
	ctx, _, sut := setupPostgresTest(t)
	defer sut.Close()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prepareDb(t, sut, "fixtures/insert_block")

			blockHashLongest := testutils.RevChainhash(t, "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde")
			blockHashStale := testutils.RevChainhash(t, "00000000000000000659df0d3cf98ebe46931b67117502168418f9dce4e1b4c9")
			blockHashOrphaned := testutils.RevChainhash(t, "0000000000000000072ded7ebd9ca6202a1894cc9dc5cd71ad6cf9c563b01ab7")
			randomPrevBlockHash := testutils.RevChainhash(t, "0000000000000000099da871f74c55a6305e6a37ef8bf955ad7d29ca4b44fda9")

			var prevBlockHash []byte

			if tc.prevBlockExists {
				switch tc.prevBlockStatus {
				case blocktx_api.Status_LONGEST:
					prevBlockHash = blockHashLongest[:]
				case blocktx_api.Status_STALE:
					prevBlockHash = blockHashStale[:]
				case blocktx_api.Status_ORPHANED:
					prevBlockHash = blockHashOrphaned[:]
				}
			} else {
				prevBlockHash = randomPrevBlockHash[:]
			}

			blockHash := testutils.RevChainhash(t, "0000000000000000082ec88d757ddaeb0aa87a5d5408b5960f27e7e67312dfe1")
			merkleRoot := testutils.RevChainhash(t, "7382df1b717287ab87e5e3e25759697c4c45eea428f701cdd0c77ad3fc707257")

			block := &blocktx_api.Block{
				Hash:         blockHash[:],
				PreviousHash: prevBlockHash,
				MerkleRoot:   merkleRoot[:],
				Height:       822016,
				Processed:    true,
				Status:       tc.blockStatus,
				Chainwork:    "123",
			}

			// when
			blockID, err := sut.UpsertBlock(ctx, block)

			// then
			if tc.shouldSucceed {
				require.NotEqual(t, uint64(0), blockID)
				require.NoError(t, err)
			} else {
				require.Equal(t, uint64(0), blockID)
				require.ErrorIs(t, err, store.ErrFailedToInsertBlock)
			}
		})
	}
}
