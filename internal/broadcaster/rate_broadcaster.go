package broadcaster

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
)

type RateBroadcaster struct {
	Broadcaster
	totalTxs        int64
	connectionCount int64
	shutdown        chan struct{}
	utxoCh          chan *bt.UTXO
	wg              sync.WaitGroup
	satoshiMap      sync.Map
	ks              *keyset.KeySet
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, ks *keyset.KeySet, utxoClient UtxoClient, isTestnet bool, opts ...func(p *Broadcaster)) (*RateBroadcaster, error) {

	b, err := NewBroadcaster(logger.With(slog.String("address", ks.Address(!isTestnet))), client, utxoClient, isTestnet, opts...)
	if err != nil {
		return nil, err
	}
	rb := &RateBroadcaster{
		Broadcaster:     b,
		shutdown:        make(chan struct{}, 1),
		utxoCh:          nil,
		wg:              sync.WaitGroup{},
		satoshiMap:      sync.Map{},
		ks:              ks,
		totalTxs:        0,
		connectionCount: 0,
	}

	return rb, nil
}

func (b *RateBroadcaster) calculateFeeSat(tx *bt.Tx) uint64 {
	size, err := tx.EstimateSizeWithTypes()
	if err != nil {
		return 0
	}
	varIntUpper := bt.VarInt(tx.OutputCount()).UpperLimitInc()
	if varIntUpper == -1 {
		return 0
	}

	changeOutputFee := varIntUpper
	changeP2pkhByteLen := uint64(8 + 1 + 25)

	totalBytes := size.TotalStdBytes + changeP2pkhByteLen

	miningFeeSat := float64(totalBytes*uint64(b.standardMiningFee.Satoshis)) / float64(b.standardMiningFee.Bytes)

	sFees := uint64(math.Ceil(miningFeeSat))
	txFees := sFees + uint64(changeOutputFee)

	return txFees
}

func (b *RateBroadcaster) Start(rateTxsPerSecond int, limit int64) error {

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-b.shutdown:
				b.cancelAll()
			case <-b.ctx.Done():
				return
			}
		}
	}()

	_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(b.ctx, !b.isTestnet, b.ks.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return err
	}
	if math.Abs(float64(unconfirmed)) > 0 {
		return fmt.Errorf("key with address %s balance has unconfirmed amount %d sat", b.ks.Address(!b.isTestnet), unconfirmed)
	}

	utxoSet, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, !b.isTestnet, b.ks.Script, b.ks.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return fmt.Errorf("failed to get utxos: %v", err)
	}

	submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(b.batchSize)

	if submitBatchesPerSecond > millisecondsPerSecond {
		return fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, b.batchSize, submitBatchesPerSecond)
	}

	if len(utxoSet) < b.batchSize {
		return fmt.Errorf("size of utxo set %d is smaller than requested batch size %d - create more utxos first", len(utxoSet), b.batchSize)
	}

	b.utxoCh = make(chan *bt.UTXO, 100000)
	for _, utxo := range utxoSet {
		b.utxoCh <- utxo
	}

	submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond
	submitBatchTicker := time.NewTicker(submitBatchInterval)

	errCh := make(chan error, 100)

	b.wg.Add(1)
	go func() {
		defer func() {
			b.logger.Info("shutting down broadcaster")
			b.wg.Done()
		}()

		for {
			select {
			case <-b.ctx.Done():
				return
			case <-submitBatchTicker.C:

				txs, err := b.createSelfPayingTxs()
				if err != nil {
					b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
					b.shutdown <- struct{}{}
					continue
				}

				if limit > 0 && atomic.LoadInt64(&b.totalTxs) >= limit {
					b.logger.Info("limit reached", slog.Int64("total", atomic.LoadInt64(&b.totalTxs)), slog.Int64("limit", limit))
					b.shutdown <- struct{}{}
				}

				b.broadcastBatchAsync(txs, errCh, metamorph_api.Status_RECEIVED)

			case responseErr := <-errCh:
				b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
			}
		}
	}()

	return nil
}

func (b *RateBroadcaster) createSelfPayingTxs() ([]*bt.Tx, error) {
	txs := make([]*bt.Tx, 0, b.batchSize)

utxoLoop:
	for {
		select {
		case <-b.ctx.Done():
			return txs, nil
		//case <-time.NewTimer(1 * time.Second).C:
		//	return txs, nil
		case utxo := <-b.utxoCh:
			tx := bt.NewTx()

			err := tx.FromUTXOs(utxo)
			if err != nil {
				return nil, err
			}

			fee := b.calculateFeeSat(tx)

			if utxo.Satoshis <= fee {
				continue
			}

			err = tx.PayTo(b.ks.Script, utxo.Satoshis-fee)
			if err != nil {
				return nil, err
			}

			// Todo: Add OP_RETURN with text "ARC testing" so that WoC can tag it

			unlockerGetter := unlocker.Getter{PrivateKey: b.ks.PrivateKey}
			err = tx.FillAllInputs(context.Background(), &unlockerGetter)
			if err != nil {
				return nil, err
			}

			b.satoshiMap.Store(tx.TxID(), tx.Outputs[0].Satoshis)

			txs = append(txs, tx)

			if len(txs) >= b.batchSize {
				break utxoLoop
			}
		}
	}

	return txs, nil
}

func (b *RateBroadcaster) broadcastBatchAsync(txs []*bt.Tx, errCh chan error, waitForStatus metamorph_api.Status) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
		defer cancel()

		atomic.AddInt64(&b.connectionCount, 1)
		resp, err := b.client.BroadcastTransactions(b.ctx, txs, waitForStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, false)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			errCh <- err
		}

		atomic.AddInt64(&b.connectionCount, -1)

		for _, res := range resp {

			txIDBytes, err := hex.DecodeString(res.Txid)
			if err != nil {
				b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
				continue
			}

			sat, found := b.satoshiMap.Load(res.Txid)
			satoshis, isValid := sat.(uint64)

			if found && isValid {
				newUtxo := &bt.UTXO{
					TxID:          txIDBytes,
					Vout:          0,
					LockingScript: b.ks.Script,
					Satoshis:      satoshis,
				}
				b.utxoCh <- newUtxo
			}

			b.satoshiMap.Delete(res.Txid)

			atomic.AddInt64(&b.totalTxs, 1)
		}
	}()
}
func (b *RateBroadcaster) Shutdown() {
	b.cancelAll()

	b.wg.Wait()
}

func (b *RateBroadcaster) GetTxCount() int64 {
	return atomic.LoadInt64(&b.totalTxs)
}

func (b *RateBroadcaster) GetConnectionCount() int64 {
	return atomic.LoadInt64(&b.connectionCount)
}

func (b *RateBroadcaster) GetUtxoSetLen() int {
	return len(b.utxoCh)
}
