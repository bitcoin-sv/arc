package txfinder

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/bitcoin-sv/arc/config"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/bitcoin-sv/arc/pkg/metamorph"
	"github.com/ordishs/go-bitcoin"
)

type Finder struct {
	th metamorph.TransactionHandler
	pc *config.PeerRpcConfig
	l  *slog.Logger
	w  *woc_client.WocClient
}

func New(th metamorph.TransactionHandler, pc *config.PeerRpcConfig, w *woc_client.WocClient, l *slog.Logger) Finder {
	return Finder{
		th: th,
		pc: pc,
		l:  l,
		w:  w,
	}
}

func (f Finder) GetRawTxs(ctx context.Context, source validator.FindSourceFlag, ids []string) ([]validator.RawTx, error) {
	// NOTE: we can ignore ALL errors from providers, if one returns err we go to another
	foundTxs := make([]validator.RawTx, 0, len(ids))
	var remainingIDs []string

	// first get transactions from the handler
	if source.Has(validator.SourceTransactionHandler) {
		txs, _ := f.th.GetTransactions(ctx, ids)
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
		if len(remainingIDs) > 0 {
			f.l.Warn("couldn't find transactions in TransactionHandler", slog.Any("ids", remainingIDs))
		}

		ids = remainingIDs[:]
		remainingIDs = nil
	}

	// try to get remaining txs from the node
	if source.Has(validator.SourceNodes) {
		for _, id := range ids {
			nTx, err := getTransactionFromNode(f.pc, id)
			if err != nil {
				f.l.Warn("failed to get transaction from node", slog.String("id", id), slog.String("err", err.Error()))
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

		if len(remainingIDs) > 0 {
			f.l.Warn("couldn't find transactions in node", slog.Any("ids", remainingIDs))
		}

		ids = remainingIDs[:]
		remainingIDs = nil
	}

	// at last try the WoC
	if source.Has(validator.SourceWoC) && len(ids) > 0 {
		wocTxs, _ := f.w.GetRawTxs(ctx, ids)
		for _, wTx := range wocTxs {
			if wTx.Error != "" {
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
		if len(remainingIDs) > 0 {
			f.l.Warn("couldn't find transactions in WoC", slog.Any("ids", remainingIDs))
		}
	}

	return foundTxs, nil
}

func getTransactionFromNode(peerRpc *config.PeerRpcConfig, inputTxID string) (*bitcoin.RawTransaction, error) {
	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpc.User, peerRpc.Password, peerRpc.Host, peerRpc.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to parse rpc URL: %v", err)
	}
	// get the transaction from the bitcoin node rpc
	node, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		return nil, err
	}
	return node.GetRawTransaction(inputTxID)
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
