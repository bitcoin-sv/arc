package blocktx

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
)

type incomingBlock struct {
	hash   *chainhash.Hash
	header *wire.BlockHeader
	height uint64

	txs []*chainhash.Hash
}

func startBlockProcessing(l *slog.Logger, s store.BlocktxStore, mq MessageQueueClient, b *incomingBlock) bool {
	ctx := context.Background()

	l.Info("Start processing", slog.String("hash", b.hash.String()), slog.Uint64("height", b.height))

	storedBlck, _ := s.GetBlock(ctx, b.hash)
	if storedBlck != nil && storedBlck.Processed {
		// block exists and was processed, we can leave it, aloha!
		l.Info("Block exists already", slog.String("hash", b.hash.String()), slog.Uint64("height", storedBlck.Height))
		return false
	}

	l = l.With(slog.String("hash", b.hash.String()), slog.Uint64("height", b.height))

	if storedBlck != nil {
		l.Info("Block exists - reprocessing started")
	}

	// build and validate data before saving them in store
	newBlock := makeBlock(s, b)

	mTree := bc.BuildMerkleTreeStoreChainHash(b.txs) // TODO: tracing etc
	if !b.header.MerkleRoot.IsEqual(mTree[len(mTree)-1]) {
		// declared MR is not same as root computed from sended transactions (is it even possible?)
		l.Error("Incorrect MR")
		return false
	}

	newTxs := makeTxs(l, b, mTree, len(b.txs))

	// store block and its transactions
	// TODO: rethink if it should be done here
	bID, err := s.UpsertBlock(ctx, newBlock)
	if err != nil {
		l.Error("Insert block to store failed",
			slog.Any("err", err),
			slog.Uint64("id", bID))
		return false
	}

	for _, txs := range newTxs {
		err = s.UpsertBlockTransactions(ctx, bID, txs)
		if err != nil {
			l.Error("Insert block txs to store failed",
				slog.Any("err", err),
				slog.Uint64("id", bID))

			return false
		}
	}

	// alright, we have the block, and now:

	// 1. it's LONGEST -> pub txs, aloha!
	// 2. it's ORPHAN, so check if is a part of ORPHANED/STALE/LONGEST chain. If it's part of LONGEST or STALE merge it into the chain  and process it as
	// 3. it's STALE -> check chainwork against the LONGEST, if needed  make reorg

	var txsToPub [][]store.TransactionBlock

checkAgain:

	switch newBlock.Status {
	case blocktx_api.Status_LONGEST:
		txs, err := s.GetRegisteredTxsByBlockHashes(ctx, [][]byte{newBlock.Hash})
		if err != nil {
			l.Error("Fetching regitered txs for block failed", slog.Any("err", err))
			return false
		}

		txsToPub = append(txsToPub, txs)

	case blocktx_api.Status_ORPHANED:
		l.Info("Block is considered as ORPHANED")

		ancestors, err := s.TraceToNonOrphanChain(ctx, newBlock.PreviousHash)
		if err != nil {
			l.Error("Trace ORPHANED block to non orphan chain failed", slog.Any("err", err))
			return false
		}

		if len(ancestors) == 0 || ancestors[0].Status == blocktx_api.Status_ORPHANED {
			// no parents or part of ORPHANED chain
			l.Info("Block processing complete", slog.String("status", newBlock.Status.String()))
			return true
		}

		nestor := ancestors[0]

		orphans := ancestors
		orphans[0] = newBlock
		ancestors = nil

		if nestor.Status == blocktx_api.Status_LONGEST {
			// merge into LONGEST, and publish transactions from WHOLE orphaned chain
			nh, _ := chainhash.NewHash(nestor.Hash)

			l.Info("ORPHANED block is part of LONGEST chain", slog.String("nestor", nh.String()))

			err = acceptIntoChain(s, orphans, blocktx_api.Status_LONGEST)
			if err != nil {
				l.Error("Merging ORPHANED blocks into LONGEST chain failed",
					slog.String("nestor", nh.String()),
					slog.Any("err", err))

				return false
			}

			// prepare txs to pub
			oh := make([][]byte, len(orphans))
			for i, o := range orphans {
				oh[i] = o.Hash
			}

			txs, err := s.GetRegisteredTxsByBlockHashes(ctx, oh)
			if err != nil {
				l.Error("Fetching regitered txs for block failed",
					slog.String("nestor", nh.String()),
					slog.Any("err", err))
				return false
			}

			txsToPub = append(txsToPub, txs)

		} else if nestor.Status == blocktx_api.Status_STALE {
			// merge into STALE
			nh, _ := chainhash.NewHash(nestor.Hash)

			l.Info("ORPHANED block is part of STALE chain", slog.String("nestor", nh.String()))

			err = acceptIntoChain(s, orphans, blocktx_api.Status_STALE)
			if err != nil {
				l.Error("Merging ORPHANED blocks into STALE chain failed",
					slog.String("nestor", nh.String()),
					slog.Any("err", err))

				return false
			}

			// now the new block and any possible orphaned parents are merged into the STALE chain
			// check if a reorg needs to be done

			// dirty, but it's POC
			goto checkAgain

		} else {
			l.Error("FATAL! The new block has an unsupported status. Connect with the maintainer to check the data consistency in the database.", slog.String("status", newBlock.Status.String()))
			return false
		}

	case blocktx_api.Status_STALE:
		l.Info("Block is considered as part of STALE chain")

		staleParents, err := s.GetStaleChainBackFromHash(ctx, newBlock.PreviousHash)
		if err != nil {
			l.Error("Fetching other blocks in STALE chain failed for incoming block", slog.Any("err", err))
			return false
		}

		lowestBlock := newBlock
		if len(staleParents) > 0 {
			lowestBlock = staleParents[0]
		}

		challangerChain := append(staleParents, newBlock)

		clChain, err := s.GetLongestChainFromHeight(ctx, lowestBlock.Height)
		if err != nil {
			l.Error("Fetching LONGEST chain to compare with STALE failed", slog.Any("err", err))
			return false
		}

		newChainwork := computeChainwork(challangerChain)
		currentChainwork := computeChainwork(clChain)

		reorg := currentChainwork.Cmp(newChainwork) < 0
		if !reorg {
			l.Info("Block processing complete", slog.String("status", newBlock.Status.String()))
			return true // nowy nie jest kozakiem. olać lamusa. mogę już wyjść?
		}

		l.Warn("STALE chain has greater chainwork then current LONGEST chain. Perform chain reorganization.")
		txsToPub, err = doReorgMagic(s, clChain, challangerChain)
		if err != nil {
			l.Error("Reorg failed", slog.String("hash", b.hash.String()), slog.Any("err", err))
			return false
		}

	default:
		// what happend here?! rethink returning nil from makeBlock() if parent has UNKNOWN status
	}

	// at this moment we already know, that the new block is LONGEST (because it of its ancestr or after reorg)
	// publish transactions

	l.Info("Publish registered mined transactions")
	for _, txs := range txsToPub {
		for _, tx := range txs {
			qm := &blocktx_api.TransactionBlock{
				BlockHash:       tx.BlockHash,
				BlockHeight:     tx.BlockHeight,
				TransactionHash: tx.TxHash,
				MerklePath:      tx.MerklePath,
				BlockStatus:     tx.BlockStatus,
			}

			err = mq.PublishMarshal(MinedTxsTopic, qm)
			if err != nil {
				l.Error("Publishing mined transaction to queue failed.",
					slog.String("topic", MinedTxsTopic),
					slog.String("txhash", hex.EncodeToString(qm.TransactionHash)),
					slog.String("block-status", qm.BlockStatus.String()),
					slog.Any("err", err))
			}
		}
	}

	l.Info("Block processing complete", slog.String("status", newBlock.Status.String()))
	return true
}

func makeBlock(s store.BlocktxStore, b *incomingBlock) *blocktx_api.Block {
	ctx := context.Background()

	tip, _ := s.GetChainTip(ctx)
	parent, _ := s.GetBlock(ctx, &b.header.PrevBlock)

	new := blocktx_api.Block{
		Hash:         b.hash[:],
		PreviousHash: b.header.PrevBlock[:],
		MerkleRoot:   b.header.MerkleRoot[:],
		Height:       b.height,
		Chainwork:    calculateChainwork(b.header.Bits).String(),
	}

	if parent == nil {
		if tip != nil {
			// nie ma rodzica :( a tabela nie jest pusta
			new.Status = blocktx_api.Status_ORPHANED
		} else {
			// najszybszy block na dzikim zachodzie! w nagrode dostaje status najadłużego
			new.Status = blocktx_api.Status_LONGEST
		}
	} else if parent.Processed {
		// TODO: rename getLongestBlokByHeigh
		existing, _ := s.GetBlockByHeight(ctx, b.height) // TODO: zrób coś z tym błędem

		// ooooo mamy chalangera!
		if existing != nil {
			// zabezpiecznie przed tym, ze inna instancja nas ubiegła i procesuje ten sam block...
			// potrzebne?
			if !bytes.Equal(existing.Hash, new.Hash) {
				new.Status = blocktx_api.Status_STALE
			}
		} else {
			// jaki ojciec taki syn
			new.Status = parent.Status
		}
	} else {
		// not processed parent - we cannot be sure its state when it's during processing
		new.Status = blocktx_api.Status_ORPHANED
	}

	return &new
}

func makeTxs(l *slog.Logger, b *incomingBlock, mTree []*chainhash.Hash, txCount int) [][]store.TxWithMerklePath {
	// TODO: dilowanie z błędami. implementacja jest prosta, o perf pomysle póżniej
	// TODO: tracing
	// TODO: muszę się douczyć o merkletree - na chłopski rozum mogę podać txCount
	//       a nie iterować przez polowe mTree żeby widzieć ile mam transakcji
	const batchSize = 8192
	var bundles [][]store.TxWithMerklePath
	batch := make([]store.TxWithMerklePath, min(batchSize, txCount))
	bundles = append(bundles, batch)

	// TODO:
	//progress := progressIndices(txCount, 5)

	// TODO:  tracing
	leaves := mTree[:(len(mTree)+1)/2]

	for i, th := range leaves {
		if th == nil {
			break // padding - transakcje się wzieli i skończyli
		}

		// te errory można zignorować - niezdarzalne
		bump, _ := bc.NewBUMPFromMerkleTreeAndIndex(b.height, mTree, uint64(i)) // NOSONAR
		bHex, _ := bump.String()

		batch[i%cap(batch)].Hash = th[:]
		batch[i%cap(batch)].MerklePath = bHex

		// robim nówkę sztuke batch jeśli mamy wincyj transakcji to zapisania
		if (i+1) == cap(batch) && (i+1) < txCount {
			l.Info("Create next transactions batch", slog.Int("after", i+1), slog.Int("all", txCount))
			batch := make([]store.TxWithMerklePath, min(batchSize, txCount-i))
			bundles = append(bundles, batch)
		}
	}

	return bundles
}

func acceptIntoChain(s store.BlocktxStore, orphans []*blocktx_api.Block, chainType blocktx_api.Status) error {
	ctx := context.Background()

	newStatuses := make([]store.BlockStatusUpdate, len(orphans))
	for _, o := range orphans {
		o.Status = chainType

		newStatuses = append(newStatuses, store.BlockStatusUpdate{
			Hash:   o.Hash,
			Status: chainType,
		})
	}

	return s.UpdateBlocksStatuses(ctx, newStatuses)
}

func computeChainwork(c []*blocktx_api.Block) *big.Int {
	res := big.NewInt(0)

	for _, b := range c {
		chainwork := new(big.Int)
		chainwork.SetString(b.Chainwork, 10)

		res = res.Add(res, chainwork)
	}

	return res
}

func doReorgMagic(s store.BlocktxStore, current, new []*blocktx_api.Block) ([][]store.TransactionBlock, error) {
	ctx := context.Background()

	clHashes := make([][]byte, 0, len(current))
	nlHashes := make([][]byte, 0, len(new))
	newStatuses := make([]store.BlockStatusUpdate, 0, len(current)+len(new))

	// prepare swap statuses
	for _, b := range current {
		clHashes = append(clHashes, b.Hash)

		b.Status = blocktx_api.Status_STALE
		newStatuses = append(newStatuses, store.BlockStatusUpdate{
			Hash:   b.Hash,
			Status: b.Status,
		})
	}

	for _, b := range new {
		nlHashes = append(nlHashes, b.Hash)

		b.Status = blocktx_api.Status_LONGEST
		newStatuses = append(newStatuses, store.BlockStatusUpdate{
			Hash:   b.Hash,
			Status: b.Status,
		})
	}

	// reorg
	err := s.UpdateBlocksStatuses(ctx, newStatuses)
	if err != nil {
		return nil, fmt.Errorf("updating statuses during REORG failed. %w", err)
	}

	// bloks swaped, so far so good

	// read publishable transactions after the swap— it simplifies the logic later
	// TODO: remember to change this if we decide not to save the block at the beginning
	plTxs, err := s.GetRegisteredTxsByBlockHashes(ctx, clHashes)
	if err != nil {
		// chlip chlip
		return nil, fmt.Errorf("GetRegisteredTxsByBlockHashes during REORG failed. %w", err)
	}

	nlTxs, err := s.GetRegisteredTxsByBlockHashes(ctx, nlHashes)
	if err != nil {
		return nil, fmt.Errorf("GetRegisteredTxsByBlockHashes during REORG failed. %w", err)
	}

	return [][]store.TransactionBlock{
		nlTxs,
		outherRightJoin(nlTxs, plTxs),
	}, nil
}

func outherRightJoin(left, right []store.TransactionBlock) []store.TransactionBlock {
	leftSet := make(map[string]struct{})
	for _, item := range left {
		leftSet[string(item.TxHash)] = struct{}{}
	}

	var outerRight []store.TransactionBlock
	for _, item := range right {
		if _, found := leftSet[string(item.TxHash)]; !found {
			outerRight = append(outerRight, item)
		}
	}

	return outerRight
}
