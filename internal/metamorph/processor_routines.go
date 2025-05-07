package metamorph

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/metamorph/store"
)

// ReAnnounceUnseen re-broadcasts transactions with status lower than SEEN_ON_NETWORK
func ReAnnounceUnseen(ctx context.Context, p *Processor) []attribute.KeyValue {
	// define from what point in time we are interested in unmined transactions
	getUnseenSince := p.now().Add(-1 * p.rebroadcastExpiration)
	var offset int64

	requested := 0
	announced := 0
	for {
		// get all transactions since then chunk by chunk
		unminedTxs, err := p.store.GetUnseen(ctx, getUnseenSince, loadUnminedLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
			break
		}

		offset += loadUnminedLimit
		if len(unminedTxs) == 0 {
			break
		}

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
	}

	if announced > 0 || requested > 0 {
		p.logger.Info("Retried unseen transactions", slog.Int("announced", announced), slog.Int("requested", requested), slog.Time("since", getUnseenSince))
	}

	return []attribute.KeyValue{attribute.Int("announced", announced), attribute.Int("requested", requested)}
}

// ReAnnounceSeen re-broadcasts SEEN_ON_NETWORK transactions
func ReAnnounceSeen(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalSeenOnNetworkTxs int
	var seenOnNetworkTxs []*store.Data
	var err error

	for {
		seenOnNetworkTxs, err = p.store.GetSeenSinceLastMined(ctx, p.rebroadcastExpiration, p.reAnnounceSeen, loadSeenOnNetworkLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get seen transactions", slog.String("err", err.Error()))
			break
		}

		if len(seenOnNetworkTxs) == 0 {
			break
		}

		offset += loadSeenOnNetworkLimit
		totalSeenOnNetworkTxs += len(seenOnNetworkTxs)

		// re-announce transactions
		for _, tx := range seenOnNetworkTxs {
			p.logger.Debug("Re-announcing seen tx", slog.String("hash", tx.Hash.String()))
			p.bcMediator.AnnounceTxAsync(ctx, tx)
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
	var seenOnNetworkTxs []*store.Data
	var err error

	for {
		seenOnNetworkTxs, err = p.store.GetSeen(ctx, p.rebroadcastExpiration, p.reRegisterSeen, loadSeenOnNetworkLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get SeenOnNetwork transactions", slog.String("err", err.Error()))
			break
		}

		if len(seenOnNetworkTxs) == 0 {
			break
		}

		offset += loadSeenOnNetworkLimit
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
