package metamorph

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	"github.com/bsv-blockchain/go-sdk/util"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/global"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
)

var ErrRejectUnconfirmed = errors.New("transaction rejected as not existing in node mempool")

// ReAnnounceUnseen re-broadcasts transactions with status lower than SEEN_ON_NETWORK
func ReAnnounceUnseen(ctx context.Context, p *Processor) []attribute.KeyValue {
	// define from what point in time we are interested in unmined transactions
	getUnseenSince := p.now().Add(-1 * p.rebroadcastExpiration)
	var offset int64

	requested := 0
	announced := 0
	for {
		// get all transactions since then chunk by chunk
		unminedTxs, err := p.store.GetUnseen(ctx, getUnseenSince, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
			break
		}

		offset += loadLimit
		if len(unminedTxs) == 0 {
			break
		}

		announcedUnseen, requestedUnseen := p.reAnnounceUnseenTxs(ctx, unminedTxs)
		announced += announcedUnseen
		requested += requestedUnseen
	}

	if announced > 0 || requested > 0 {
		p.logger.Info("Retried unseen transactions", slog.Int("announced", announced), slog.Int("requested", requested), slog.Time("since", getUnseenSince))
	}

	return []attribute.KeyValue{attribute.Int("announced", announced), attribute.Int("requested", requested)}
}

func (p *Processor) reAnnounceUnseenTxs(ctx context.Context, unminedTxs []*global.Data) (int, int) {
	requested := 0
	announced := 0
	for _, tx := range unminedTxs {
		if tx.Retries > p.maxRetries {
			continue
		}

		// save the tx to cache again, in case it was removed or expired
		err := p.saveTxToCache(tx.Hash)
		if err != nil {
			p.logger.Error("Failed to store tx in cache", slog.String("hash", tx.Hash.String()), slog.String("err", err.Error()))
			continue
		}

		// mark that we retried processing this transaction once more
		if err = p.store.IncrementRetries(ctx, tx.Hash); err != nil {
			p.logger.Error("Failed to increment retries in database", slog.String("err", err.Error()))
		}

		// every second time request tx, every other time announce tx
		if tx.Retries%2 != 0 {
			// Send GETDATA to peers to see if they have it
			p.logger.Debug("Re-requesting unseen tx", slog.String("hash", tx.Hash.String()))
			p.bcMediator.AskForTxAsync(ctx, tx)
			requested++
			continue
		}

		p.logger.Debug("Re-announcing unseen tx", slog.String("hash", tx.Hash.String()))
		p.bcMediator.AnnounceTxAsync(ctx, tx)
		announced++
	}
	return announced, requested
}

// RejectUnconfirmedRequested finds transactions which have been requested, but not confirmed by any node and rejects them
func RejectUnconfirmedRequested(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalRejected int
	var txs []*chainhash.Hash
	var blocksSinceLastRequested *blocktx_api.LatestBlocksResponse
	var err error

	for {
		blocksSinceLastRequested, err = p.blocktxClient.LatestBlocks(ctx, p.rejectPendingBlocksSince)
		if err != nil {
			p.logger.Error("Failed to get blocks since last requested", slog.String("err", err.Error()))
			break
		}
		if uint64(len(blocksSinceLastRequested.Blocks)) != p.rejectPendingBlocksSince {
			p.logger.Warn("Unexpected number of blocks received", slog.Uint64("expected", p.rejectPendingBlocksSince), slog.Int("received", len(blocksSinceLastRequested.Blocks)))
			break
		}

		blocks := blocksSinceLastRequested.GetBlocks()
		sinceLastProcessed := p.now().Sub(blocks[len(blocks)-1].ProcessedAt.AsTime())

		// reject all txs, which have been requested at least `rejectPendingSeenLastRequestedAgo` AND the time since `rejectPendingBlocksSince` was processed ago
		requestedAgo := min(sinceLastProcessed, p.rejectPendingSeenLastRequestedAgo)

		txs, err = p.store.GetUnconfirmedRequested(ctx, requestedAgo, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get seen transactions", slog.String("err", err.Error()))
			break
		}

		offset += loadLimit

		p.logger.Debug("Unconfirmed requested txs", slog.Int("count", len(txs)), slog.Duration("requestedAgo", requestedAgo), slog.Bool("enabled", p.rejectPendingSeenEnabled))
		if len(txs) != 0 {
			for _, tx := range txs {
				p.logger.Info("Rejecting unconfirmed tx", slog.Bool("enabled", p.rejectPendingSeenEnabled), slog.String("hash", tx.String()))
				if p.rejectPendingSeenEnabled {
					p.storageStatusUpdateCh <- store.UpdateStatus{
						Hash:      *tx,
						Status:    metamorph_api.Status_REJECTED,
						Error:     ErrRejectUnconfirmed,
						Timestamp: p.now(),
					}
				}
			}
			totalRejected += len(txs)
		}

		time.Sleep(100 * time.Millisecond)
	}

	if totalRejected > 0 {
		p.logger.Info("Rejected txs", slog.Int("count", totalRejected))
	}

	return []attribute.KeyValue{attribute.Int("rejected", totalRejected)}
}

// ReAnnounceSeen re-broadcasts and re-requests SEEN_ON_NETWORK transactions that have been pending
func ReAnnounceSeen(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalSeenOnNetworkTxs int
	var pendingSeen []*global.Data
	var hashes []*chainhash.Hash
	var err error

	for {
		pendingSeen, err = p.store.GetSeenPending(ctx, p.rebroadcastExpiration, p.reAnnounceSeenLastConfirmedAgo, p.reAnnounceSeenPendingSince, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get seen transactions", slog.String("err", err.Error()))
			break
		}
		hashes = make([]*chainhash.Hash, len(pendingSeen))

		if len(pendingSeen) == 0 {
			break
		}

		offset += loadLimit
		totalSeenOnNetworkTxs += len(pendingSeen)

		// re-announce transactions
		for i, tx := range pendingSeen {
			p.logger.Debug("Re-announcing seen tx", slog.String("hash", tx.Hash.String()))
			p.bcMediator.AskForTxAsync(ctx, tx)

			p.logger.Debug("Re-requesting seen tx", slog.String("hash", tx.Hash.String()))
			p.bcMediator.AnnounceTxAsync(ctx, tx)

			hashes[i] = tx.Hash
		}

		time.Sleep(100 * time.Millisecond)

		err = p.store.SetRequested(ctx, hashes)
		if err != nil {
			p.logger.Error("Failed to mark seen txs requested", slog.String("err", err.Error()))
		}

		time.Sleep(100 * time.Millisecond)
	}

	if totalSeenOnNetworkTxs > 0 {
		p.logger.Info("Seen txs re-announced", slog.Int("count", totalSeenOnNetworkTxs))
	}

	return []attribute.KeyValue{attribute.Int("announced", totalSeenOnNetworkTxs)}
}

// RegisterSeenTxs re-registers and SEEN_ON_NETWORK transactions
func RegisterSeenTxs(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalSeenOnNetworkTxs int
	var seenOnNetworkTxs []*global.Data
	var err error

	for {
		seenOnNetworkTxs, err = p.store.GetSeen(ctx, p.rebroadcastExpiration, p.reRegisterSeen, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get SeenOnNetwork transactions", slog.String("err", err.Error()))
			break
		}

		if len(seenOnNetworkTxs) == 0 {
			break
		}

		offset += loadLimit
		totalSeenOnNetworkTxs += len(seenOnNetworkTxs)

		err = p.registerTransactions(ctx, seenOnNetworkTxs)
		if err != nil {
			p.logger.Error("Failed to register txs in blocktx", slog.String("err", err.Error()))
		}

		time.Sleep(100 * time.Millisecond)
	}

	if totalSeenOnNetworkTxs > 0 {
		p.logger.Info("Seen txs re-registered", slog.Int("count", totalSeenOnNetworkTxs))
	}

	return []attribute.KeyValue{attribute.Int("registered", totalSeenOnNetworkTxs)}
}

func ProcessDoubleSpendTxs(ctx context.Context, p *Processor) []attribute.KeyValue {
	var totalRejected int
	doubleSpendTxs, err := p.store.GetDoubleSpendTxs(ctx, p.now().Add(-p.doubleSpendTxStatusOlderThan))
	if err != nil {
		p.logger.Error("failed to get double spend status transactions", slog.String("err", err.Error()))
		return []attribute.KeyValue{attribute.Int("rejected", totalRejected)}
	}

	for _, doubleSpendTx := range doubleSpendTxs {
		competingTxs, err := txBytesFromHex(doubleSpendTx.CompetingTxs)
		if err != nil {
			p.logger.Error("failed to convert competing txs from hex", slog.String("err", err.Error()))
			continue
		}

		competingTxStatuses, err := p.blocktxClient.AnyTransactionsMined(ctx, competingTxs)
		if err != nil {
			p.logger.Error("cannot get competing tx statuses from blocktx", slog.String("err", err.Error()))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// if ANY of those competing txs gets mined we reject this one
		for _, competingTx := range competingTxStatuses {
			if competingTx.Mined {
				_, err := p.store.UpdateStatus(ctx, []store.UpdateStatus{
					{
						Hash:         *doubleSpendTx.Hash,
						Status:       metamorph_api.Status_REJECTED,
						CompetingTxs: doubleSpendTx.CompetingTxs,
						Timestamp:    p.now(),
						Error:        fmt.Errorf("double spend tx rejected, competing tx %s mined", hex.EncodeToString(util.ReverseBytes(competingTx.Hash))),
					},
				})
				if err != nil {
					p.logger.Error("failed to update double spend status", slog.String("err", err.Error()), slog.String("hash", doubleSpendTx.Hash.String()))
					continue
				}
				totalRejected++
				p.logger.Info("Double spend tx rejected", slog.String("hash", doubleSpendTx.Hash.String()), slog.String("competing mined tx hash", hex.EncodeToString(competingTx.Hash)))
				break
			}
		}
	}
	return []attribute.KeyValue{attribute.Int("rejected", totalRejected)}
}
