package txfinder

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/ordishs/go-bitcoin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/tracing"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
)

type Finder struct {
	th                metamorph.TransactionHandler
	n                 *bitcoin.Bitcoind
	w                 *woc_client.WocClient
	l                 *slog.Logger
	tracingEnabled    bool
	tracingAttributes []attribute.KeyValue
}

func New(th metamorph.TransactionHandler, pc *config.PeerRPCConfig, w *woc_client.WocClient, l *slog.Logger) Finder {
	l = l.With(slog.String("module", "tx-finder"))
	var n *bitcoin.Bitcoind

	if pc != nil {
		rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", pc.User, pc.Password, pc.Host, pc.Port))
		if err != nil {
			l.Warn("cannot parse node rpc url. Finder will not use node as source")
		} else {
			// get the transaction from the bitcoin node rpc
			n, err = bitcoin.NewFromURL(rpcURL, false)
			if err != nil {
				l.Warn("cannot create node client. Finder will not use node as source")
			}
		}
	}

	return Finder{
		th: th,
		n:  n,
		w:  w,
		l:  l,
	}
}

func (f Finder) GetRawTxs(ctx context.Context, source validator.FindSourceFlag, ids []string) ([]validator.RawTx, error) {
	ctx, span := tracing.StartTracing(ctx, "GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
	defer tracing.EndTracing(span)

	// NOTE: we can ignore ALL errors from providers, if one returns err we go to another
	foundTxs := make([]validator.RawTx, 0, len(ids))
	var remainingIDs []string

	// first get transactions from the handler
	if source.Has(validator.SourceTransactionHandler) {
		txs, thErr := f.th.GetTransactions(ctx, ids)
		for _, tx := range txs {
			rt := validator.RawTx{
				TxID:    tx.TxID,
				Bytes:   tx.Bytes,
				IsMined: tx.BlockHeight > 0,
			}

			foundTxs = append(foundTxs, rt)
		}

		// add remaining ids
		remainingIDs = outerRightJoin(foundTxs, ids)
		if len(remainingIDs) > 0 || thErr != nil {
			f.l.WarnContext(ctx, "couldn't find transactions in TransactionHandler", slog.Any("ids", remainingIDs), slog.Any("source-err", thErr))
		}

		ids = remainingIDs[:]
		remainingIDs = nil
	}

	// try to get remaining txs from the node
	if source.Has(validator.SourceNodes) && f.n != nil {
		var nErr error
		for _, id := range ids {
			var getRawTxsspan trace.Span
			ctx, getRawTxsspan = tracing.StartTracing(ctx, "GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
			nTx, err := f.n.GetRawTransaction(id)
			tracing.EndTracing(getRawTxsspan)
			if err != nil {
				nErr = errors.Join(nErr, fmt.Errorf("%s: %w", id, err))
			}
			if nTx != nil {
				rt, e := newRawTx(nTx.TxID, nTx.Hex, nTx.BlockHeight)
				if e != nil {
					return nil, e
				}

				foundTxs = append(foundTxs, rt)
			} else {
				remainingIDs = append(remainingIDs, id)
			}
		}

		if len(remainingIDs) > 0 || nErr != nil {
			f.l.WarnContext(ctx, "couldn't find transactions in node", slog.Any("ids", remainingIDs), slog.Any("source-error", nErr))
		}

		ids = remainingIDs[:]
		remainingIDs = nil
	}

	// at last try the WoC
	if source.Has(validator.SourceWoC) && len(ids) > 0 {
		var wocSpan trace.Span
		ctx, wocSpan = tracing.StartTracing(ctx, "GetRawTxs", f.tracingEnabled, f.tracingAttributes...)
		wocTxs, wocErr := f.w.GetRawTxs(ctx, ids)
		defer tracing.EndTracing(wocSpan)

		for _, wTx := range wocTxs {
			if wTx.Error != "" {
				wocErr = errors.Join(wocErr, fmt.Errorf("returned error data tx %s: %s", wTx.TxID, wTx.Error))
				continue
			}

			rt, e := newRawTx(wTx.TxID, wTx.Hex, wTx.BlockHeight)
			if e != nil {
				return nil, e
			}

			foundTxs = append(foundTxs, rt)
		}

		// add remaining ids
		remainingIDs = outerRightJoin(foundTxs, ids)
		if len(remainingIDs) > 0 || wocErr != nil {
			f.l.WarnContext(ctx, "couldn't find transactions in WoC", slog.Any("ids", remainingIDs), slog.Any("source-error", wocErr))
		}
	}

	return foundTxs, nil
}

func newRawTx(id, hexTx string, blockH uint64) (validator.RawTx, error) {
	b, e := hex.DecodeString(hexTx)
	if e != nil {
		return validator.RawTx{}, e
	}

	rt := validator.RawTx{
		TxID:    id,
		Bytes:   b,
		IsMined: blockH > 0,
	}

	return rt, nil
}

func outerRightJoin(left []validator.RawTx, right []string) []string {
	var outerRight []string

	for _, id := range right {
		found := false
		for _, tx := range left {
			if tx.TxID == id {
				found = true
				break
			}
		}

		if !found {
			outerRight = append(outerRight, id)
		}
	}

	return outerRight
}
