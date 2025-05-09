package metamorph

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
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
		unminedTxs, err := p.store.GetUnseen(ctx, getUnseenSince, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get unmined transactions", slog.String("err", err.Error()))
			break
		}

		offset += loadLimit
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

// RejectUnconfirmedRequested re-broadcasts SEEN_ON_NETWORK transactions
func RejectUnconfirmedRequested(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalRejected int
	var txHashes []*chainhash.Hash
	var err error

	// first delete all requested transactions which have transitioned to another status
	rows, err := p.store.DeleteConfirmedRequested(ctx)
	if err != nil {
		p.logger.Error("Failed to delete confirmed seen", slog.String("err", err.Error()))
		return nil
	}

	if rows > 0 {
		p.logger.Info("Deleted confirmed requested", slog.Int64("count", rows))
	}

	for {
		txHashes, err = p.store.GetAndDeleteUnconfirmedRequested(ctx, p.rebroadcastExpiration, loadLimit, offset)
		if err != nil {
			p.logger.Error("Failed to get seen transactions", slog.String("err", err.Error()))
			break
		}

		offset += loadLimit
		totalRejected += len(txHashes)

		if len(txHashes) == 0 {
			break
		}

		for _, txHash := range txHashes {
			p.logger.Debug("Re-announcing seen tx", slog.String("hash", txHash.String()))

			p.statusMessageCh <- &metamorph_p2p.TxStatusMessage{
				Start:  time.Now(),
				Hash:   txHash,
				Status: metamorph_api.Status_REJECTED,
				Err:    errors.New("transaction rejected as not existing in node mempool"),
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	if totalRejected > 0 {
		p.logger.Info("Rejected txs", slog.Int("count", totalRejected))
	}

	return []attribute.KeyValue{attribute.Int("rejected", totalRejected)}
}

// ReAnnounceSeen re-broadcasts and re-requests SEEN_ON_NETWORK transactions that have been pending since more than 10 min
func ReAnnounceSeen(ctx context.Context, p *Processor) []attribute.KeyValue {
	var offset int64
	var totalSeenOnNetworkTxs int
	var pendingSeen []*store.Data
	var hashes []*chainhash.Hash
	var err error
	const pendingSince = 10 * time.Minute
	for {
		pendingSeen, err = p.store.GetSeenPending(ctx, p.rebroadcastExpiration, p.reAnnounceSeen, pendingSince, loadLimit, offset)
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
	}

	err = p.store.SetRequested(ctx, hashes)
	if err != nil {
		p.logger.Error("Failed to store hashes", slog.String("err", err.Error()))
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
