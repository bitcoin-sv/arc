package integrationtest

// Components of this test:
// 		Blocktx Processor
// 		Postgresql Store - running on docker
// 		PeerHandler - mocked
// 		Message queue sending txs to metamorph - mocked
//
// Flow of this test:
// 		1. Blocks at heights 822014-822017 (LONGEST), 822018-822020 (ORPHANED) and 822022-822023 (ORPHANED) are added to db from fixtures
// 		2. A hardcoded msg with competing block at height 822015 is being sent through the mocked PeerHandler
// 		3. This block has a chainwork lower than the current tip of chain - becomes STALE
// 		4. Registered transactions from this block are ignored
// 		5. Next competing block, at height 822016 is being sent through the mocked PeerHandler
// 		6. This block has a greater chainwork than the current tip of longest chain - it becomes LONGEST despite not being the highest
// 		7. Verification of reorg - checking if statuses are correctly switched
// 		8. Verification of transactions
// 			- transactions from the stale chain becoming the longest are published
// 			- transactions that were previously in the longest chain are published with updated block data
// 			- transactions that were previously in the longest chain, but are not in the stale chain are published with blockstatus = STALE
// 		9. A new block at height 822021 is being sent through the mocked PeerHandler
// 		10. This block is extending the orphaned chain and finds that it's connected to the stale chain - orphans get updated to STALE
// 		11. The new stale chain does not have a greater chainwork than the current longest chain - entire orphaned chain becomes STALE
// 		12. A new block at height 822024 is being sent through the mocked PeerHandler
// 		13. This block extends the orphaned chain and finds that it's connected to the stale chain - orphans get updated to STALE
// 		14. The new stale chain has a greater chainwork than the current longest chain
// 			- entire STALE chain at heights 822015 - 822024 becomes LONGEST
// 			- entire LONGEST chain at height 822015 - 822016 becomes STALE
// 		15. Verification of reorg - checking if statuses are correctly switched (for blocks and for transactions)

import (
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/pkg/test_utils"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
)

func TestReorg(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("block on empty database", func(t *testing.T) {
		defer pruneTables(t, dbConn)

		processor, p2pMsgHandler, store, _, _ := setupSut(t, dbInfo)

		const blockHash822011 = "bf9be09b345cc2d904b59951cc8a2ed452d8d143e2e25cde64058270fb3a667a"

		blockHash := testutils.RevChainhash(t, blockHash822011)
		prevBlockHash := testutils.RevChainhash(t, "00000000000000000a00c377b260a3219b0c314763f486bc363df7aa7e22ad72")
		txHash, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
		require.NoError(t, err)
		merkleRoot, err := chainhash.NewHashFromStr("be181e91217d5f802f695e52144078f8dfbe51b8a815c3d6fb48c0d853ec683b")
		require.NoError(t, err)

		// should become LONGEST
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevBlockHash, // NON-existent in the db
				MerkleRoot: *merkleRoot,
				Bits:       0x1d00ffff,
			},
			Height:            uint64(822011),
			TransactionHashes: []*chainhash.Hash{txHash},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block
		time.Sleep(200 * time.Millisecond)

		verifyBlock(t, store, blockHash822011, 822011, blocktx_api.Status_LONGEST)
	})

	t.Run("stale block", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/stale_block")

		processor, p2pMsgHandler, store, _, publishedTxsCh := setupSut(t, dbInfo)

		const (
			blockHash822014StartOfChain = "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000"
			blockHash822015             = "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000"
			blockHash822015Fork         = "82471bbf045ab13825a245b37de71d77ec12513b37e2524ec11551d18c19f7c3"
			txhash822015                = "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
			txhash822015Competing       = "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"
		)

		blockHash := testutils.RevChainhash(t, blockHash822015Fork)
		prevBlockHash := testutils.RevChainhash(t, blockHash822014StartOfChain)
		txHash := testutils.RevChainhash(t, txhash822015)
		txHash2 := testutils.RevChainhash(t, txhash822015Competing) // should not be published - is already in the longest chain
		treeStore := bc.BuildMerkleTreeStoreChainHash([]*chainhash.Hash{txHash, txHash2})
		merkleRoot := treeStore[len(treeStore)-1]

		// should become STALE
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevBlockHash, // block with status LONGEST at height 822014
				MerkleRoot: *merkleRoot,
				Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing block
			},
			Height:            uint64(822015), // competing block already exists at this height
			TransactionHashes: []*chainhash.Hash{txHash, txHash2},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block
		time.Sleep(200 * time.Millisecond)

		verifyBlock(t, store, blockHash822015Fork, 822015, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822015, 822015, blocktx_api.Status_LONGEST)

		publishedTxs := getPublishedTxs(publishedTxsCh)

		// verify the no transaction was published to metamorph
		require.Len(t, publishedTxs, 0)
	})

	t.Run("reorg", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/reorg")

		processor, p2pMsgHandler, store, _, publishedTxsCh := setupSut(t, dbInfo)

		const (
			blockHash822015Fork = "82471bbf045ab13825a245b37de71d77ec12513b37e2524ec11551d18c19f7c3"
			blockHash822016Fork = "032c3688bc7536b2d787f3a196b1145a09bf33183cd1448ff6b1a9dfbb022db8"

			blockHash822014StartOfChain = "67708796ef57464ed9eaf2a663d3da32372e4c2fb65558020000000000000000"
			blockHash822015             = "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000"
			blockHash822016             = "e1df1273e6e7270f96b508545d7aa80aebda7d758dc82e080000000000000000"
			blockHash822017             = "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000"
			blockHash822018Orphan       = "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde"

			txhash822015          = "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
			txhash822015Competing = "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"
			txhash822016          = "2ff4430eb883c6f6c0640a5d716b2d107bbc0efa5aeaa237aec796d4686b0a8f"
			txhash822017          = "ece2b7e40d98749c03c551b783420d6e3fdc3c958244bbf275437839585829a6"
		)

		blockHash := testutils.RevChainhash(t, blockHash822016Fork)
		txHash := testutils.RevChainhash(t, txhash822016)
		txHash2 := testutils.RevChainhash(t, "ee76f5b746893d3e6ae6a14a15e464704f4ebd601537820933789740acdcf6aa")
		treeStore := bc.BuildMerkleTreeStoreChainHash([]*chainhash.Hash{txHash, txHash2})
		merkleRoot := treeStore[len(treeStore)-1]
		prevhash := testutils.RevChainhash(t, blockHash822015Fork)

		// should become LONGEST
		// reorg should happen
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevhash, // block with status STALE at height 822015
				MerkleRoot: *merkleRoot,
				Bits:       0x1a05db8b, // chainwork: "12301577519373468" higher than the competing chain
			},
			Height:            uint64(822016), // competing block already exists at this height
			TransactionHashes: []*chainhash.Hash{txHash, txHash2},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block and perform reorg
		time.Sleep(1 * time.Second)

		// verify that reorg happened
		verifyBlock(t, store, blockHash822016Fork, 822016, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822015Fork, 822015, blocktx_api.Status_LONGEST)

		verifyBlock(t, store, blockHash822014StartOfChain, 822014, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822015, 822015, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822016, 822016, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822017, 822017, blocktx_api.Status_STALE)

		verifyBlock(t, store, blockHash822018Orphan, 822018, blocktx_api.Status_ORPHANED)

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
				TransactionHash: testutils.RevChainhash(t, txhash822015Competing)[:],
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

		publishedTxs := getPublishedTxs(publishedTxsCh)

		verifyTxs(t, expectedTxs, publishedTxs)
	})

	t.Run("stale orphans", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/stale_orphans")

		processor, p2pMsgHandler, store, _, publishedTxsCh := setupSut(t, dbInfo)

		const (
			blockHash822017Longest = "00000000000000000643d48201cf609b8cc50befe804194f19a7ec61cf046239"
			blockHash822017Stale   = "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000"
			blockHash822018Orphan  = "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde"
			blockHash822019Orphan  = "00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656"
			blockHash822020Orphan  = "00000000000000000a5c4d27edc0178e953a5bb0ab0081e66cb30c8890484076"
			blockHash822021        = "d46bf0a189927b62c8ff785d393a545093ca01af159aed771a8d94749f06c060"
			blockHash822022Orphan  = "0000000000000000059d6add76e3ddb8ec4f5ffd6efecd4c8b8c577bd32aed6c"
			blockHash822023Orphan  = "0000000000000000082131979a4e25a5101912a5f8461e18f306d23e158161cd"
		)

		blockHash := testutils.RevChainhash(t, blockHash822021)
		txHash := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
		merkleRoot := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
		prevhash := testutils.RevChainhash(t, blockHash822020Orphan)

		// should become STALE
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevhash, // block with status ORPHANED at height 822020 - connected to STALE chain
				MerkleRoot: *merkleRoot,
				Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing chain
			},
			Height:            uint64(822021),
			TransactionHashes: []*chainhash.Hash{txHash},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block and find orphans
		time.Sleep(1 * time.Second)

		// verify that the block and orphans have STALE status
		verifyBlock(t, store, blockHash822017Stale, 822017, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822018Orphan, 822018, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822019Orphan, 822019, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822020Orphan, 822020, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822021, 822021, blocktx_api.Status_STALE)

		// verify that the longest chain is still the same
		verifyBlock(t, store, blockHash822017Longest, 822017, blocktx_api.Status_LONGEST)

		// verify that the blocks after the next gap are still orphans
		verifyBlock(t, store, blockHash822022Orphan, 822022, blocktx_api.Status_ORPHANED)
		verifyBlock(t, store, blockHash822023Orphan, 822023, blocktx_api.Status_ORPHANED)

		publishedTxs := getPublishedTxs(publishedTxsCh)

		// verify no transaction was published
		require.Len(t, publishedTxs, 0)
	})

	t.Run("reorg orphans", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/reorg_orphans")

		processor, p2pMsgHandler, store, _, publishedTxsCh := setupSut(t, dbInfo)

		const (
			blockHash822014StartOfChain = "f97e20396f02ab990ed31b9aec70c240f48b7e5ea239aa050000000000000000"
			blockHash822015             = "c9b4e1e4dcf9188416027511671b9346be8ef93c0ddf59060000000000000000"
			blockHash822016             = "e1df1273e6e7270f96b508545d7aa80aebda7d758dc82e080000000000000000"
			blockHash822017             = "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000"

			blockHash822015Fork = "82471bbf045ab13825a245b37de71d77ec12513b37e2524ec11551d18c19f7c3"
			blockHash822016Fork = "032c3688bc7536b2d787f3a196b1145a09bf33183cd1448ff6b1a9dfbb022db8"

			blockHash822018Orphan = "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde"
			blockHash822019Orphan = "00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656"
			blockHash822020Orphan = "00000000000000000a5c4d27edc0178e953a5bb0ab0081e66cb30c8890484076"
			blockHash822021       = "743c7dc491ae5fddd37ebf63058f9574b4db9f6a89f483a4baec31820e5df61d"
			blockHash822022Orphan = "0000000000000000059d6add76e3ddb8ec4f5ffd6efecd4c8b8c577bd32aed6c"
			blockHash822023Orphan = "0000000000000000082131979a4e25a5101912a5f8461e18f306d23e158161cd"

			txhash822015          = "cd3d2f97dfc0cdb6a07ec4b72df5e1794c9553ff2f62d90ed4add047e8088853"
			txhash822015Competing = "b16cea53fc823e146fbb9ae4ad3124f7c273f30562585ad6e4831495d609f430"
			txhash822016          = "2ff4430eb883c6f6c0640a5d716b2d107bbc0efa5aeaa237aec796d4686b0a8f"
			txhash822017          = "ece2b7e40d98749c03c551b783420d6e3fdc3c958244bbf275437839585829a6"
		)

		blockHash := testutils.RevChainhash(t, blockHash822021)
		prevhash := testutils.RevChainhash(t, blockHash822020Orphan)
		txHash := testutils.RevChainhash(t, "3e15f823a7de25c26ce9001d4814a6f0ebc915a1ca4f1ba9cfac720bd941c39c")
		merkleRoot := testutils.RevChainhash(t, "3e15f823a7de25c26ce9001d4814a6f0ebc915a1ca4f1ba9cfac720bd941c39c")

		// should become LONGEST
		// reorg should happen
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevhash, // block with status ORPHANED at height 822020 - connected to STALE chain
				MerkleRoot: *merkleRoot,
				Bits:       0x1d00ffff, // chainwork: "4295032833" lower than the competing chain
				// the sum of orphan chain has a higher chainwork and should cause a reorg
			},
			Height:            uint64(822021),
			TransactionHashes: []*chainhash.Hash{txHash},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block, find orphans and perform reorg
		time.Sleep(3 * time.Second)

		// verify that the reorg happened
		verifyBlock(t, store, blockHash822014StartOfChain, 822014, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822015, 822015, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822016, 822016, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822017, 822017, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822018Orphan, 822018, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822019Orphan, 822019, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822020Orphan, 822020, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822021, 822021, blocktx_api.Status_LONGEST)

		verifyBlock(t, store, blockHash822015Fork, 822015, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822016Fork, 822016, blocktx_api.Status_STALE)

		verifyBlock(t, store, blockHash822022Orphan, 822022, blocktx_api.Status_ORPHANED)
		verifyBlock(t, store, blockHash822023Orphan, 822023, blocktx_api.Status_ORPHANED)

		bh822015 := testutils.RevChainhash(t, blockHash822015)
		bh822015Fork := testutils.RevChainhash(t, blockHash822015Fork)
		bh822016Fork := testutils.RevChainhash(t, blockHash822016Fork)
		bh822017 := testutils.RevChainhash(t, blockHash822017)

		expectedTxs := []*blocktx_api.TransactionBlock{
			{ // in stale chain
				BlockHash:       bh822015Fork[:],
				BlockHeight:     822015,
				TransactionHash: testutils.RevChainhash(t, txhash822015)[:],
				BlockStatus:     blocktx_api.Status_STALE,
			},
			{ // in both chains - should have blockdata updated
				BlockHash:       bh822015[:],
				BlockHeight:     822015,
				TransactionHash: testutils.RevChainhash(t, txhash822015Competing)[:],
				BlockStatus:     blocktx_api.Status_LONGEST,
			},
			{ // in stale chain
				BlockHash:       bh822016Fork[:],
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

		publishedTxs := getPublishedTxs(publishedTxsCh)

		verifyTxs(t, expectedTxs, publishedTxs)
	})

	t.Run("unorphan blocks until gap", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/stale_orphans")

		processor, p2pMsgHandler, store, _, publishedTxsCh := setupSut(t, dbInfo)

		const (
			blockHash822017Longest = "00000000000000000643d48201cf609b8cc50befe804194f19a7ec61cf046239"
			blockHash822017Stale   = "76404890880cb36ce68100abb05b3a958e17c0ed274d5c0a0000000000000000"
			blockHash822018Orphan  = "000000000000000003b15d668b54c4b91ae81a86298ee209d9f39fd7a769bcde"
			blockHash822019Orphan  = "00000000000000000364332e1bbd61dc928141b9469c5daea26a4b506efc9656"
			blockHash822020Orphan  = "00000000000000000a5c4d27edc0178e953a5bb0ab0081e66cb30c8890484076"
			blockHash822021        = "d46bf0a189927b62c8ff785d393a545093ca01af159aed771a8d94749f06c060"
			blockHash822022Orphan  = "0000000000000000059d6add76e3ddb8ec4f5ffd6efecd4c8b8c577bd32aed6c"
			blockHash822023Orphan  = "0000000000000000082131979a4e25a5101912a5f8461e18f306d23e158161cd"
		)

		blockHash := testutils.RevChainhash(t, blockHash822021)
		txHash := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
		merkleRoot := testutils.RevChainhash(t, "de0753d9ce6f92e340843cbfdd11e58beff8c578956ecdec4c461b018a26b8a9")
		prevhash := testutils.RevChainhash(t, blockHash822017Longest)

		// should become LONGEST
		blockMessage := &bcnet.BlockMessage{
			Hash: blockHash,
			Header: &wire.BlockHeader{
				Version:    541065216,
				PrevBlock:  *prevhash, // block with status ORPHANED at height 822020 - connected to STALE chain
				MerkleRoot: *merkleRoot,
				Bits:       0x1a05db8b, // chainwork: "12301577519373468" higher than the competing chain
			},
			Height:            uint64(822021),
			TransactionHashes: []*chainhash.Hash{txHash},
		}

		processor.StartBlockProcessing()
		p2pMsgHandler.OnReceive(blockMessage, nil)

		// Allow DB to process the block and find orphans
		time.Sleep(1 * time.Second)

		// verify that the orphans before the new block are still orphans
		verifyBlock(t, store, blockHash822017Stale, 822017, blocktx_api.Status_STALE)
		verifyBlock(t, store, blockHash822018Orphan, 822018, blocktx_api.Status_ORPHANED)
		verifyBlock(t, store, blockHash822019Orphan, 822019, blocktx_api.Status_ORPHANED)
		verifyBlock(t, store, blockHash822020Orphan, 822020, blocktx_api.Status_ORPHANED)
		// verify that the new block is longest
		verifyBlock(t, store, blockHash822021, 822021, blocktx_api.Status_LONGEST)

		// verify that the longest chain is still the same
		verifyBlock(t, store, blockHash822017Longest, 822017, blocktx_api.Status_LONGEST)

		// verify that the blocks after the new block and until the next gap are now LONGEST
		verifyBlock(t, store, blockHash822022Orphan, 822022, blocktx_api.Status_LONGEST)
		verifyBlock(t, store, blockHash822023Orphan, 822023, blocktx_api.Status_LONGEST)

		publishedTxs := getPublishedTxs(publishedTxsCh)

		// verify no transaction was published
		require.Len(t, publishedTxs, 0)
	})
}
