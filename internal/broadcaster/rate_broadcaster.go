package broadcaster

import (
	"container/list"
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
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
)

type UtxoClient interface {
	GetUTXOs(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
	GetUTXOsWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error)
	GetUTXOsList(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error)
	GetUTXOsListWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) (*list.List, error)
	GetBalance(ctx context.Context, mainnet bool, address string) (int64, int64, error)
	GetBalanceWithRetries(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error)
	TopUp(ctx context.Context, mainnet bool, address string) error
}

type RateBroadcaster struct {
	Broadcaster
	wg         sync.WaitGroup
	totalTxs   int64
	shutdown   chan struct{}
	mu         sync.RWMutex
	satoshiMap map[string]uint64
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*RateBroadcaster, error) {

	b, err := NewBroadcaster(logger, client, keySets, utxoClient, opts...)
	if err != nil {
		return nil, err
	}

	rb := &RateBroadcaster{
		Broadcaster: *b,
		totalTxs:    0,
		mu:          sync.RWMutex{},
		shutdown:    make(chan struct{}, 10),
		satoshiMap:  map[string]uint64{},
		wg:          sync.WaitGroup{},
	}

	go func() {
		for range rb.shutdown {
			rb.cancelAll()
		}
	}()

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

func (b *RateBroadcaster) StartRateBroadcaster(rateTxsPerSecond int, limit int64) error {
	for _, ks := range b.keySets {

		_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(b.ctx, !b.isTestnet, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return err
		}
		if math.Abs(float64(unconfirmed)) > 0 {
			return fmt.Errorf("key with address %s balance has unconfirmed amount %d sat", ks.Address(!b.isTestnet), unconfirmed)
		}

		utxoSet, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, !b.isTestnet, ks.Script, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return fmt.Errorf("failed to get utxos: %v", err)
		}

		b.logger.Info("starting broadcasting", slog.Int("rate [txs/s]", rateTxsPerSecond), slog.Int("batch size", b.batchSize), slog.String("address", ks.Address(!b.isTestnet)))

		submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(b.batchSize)

		if submitBatchesPerSecond > millisecondsPerSecond {
			return fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, b.batchSize, submitBatchesPerSecond)
		}

		if len(utxoSet) < b.batchSize {
			return fmt.Errorf("size of utxo set %d is smaller than requested batch size %d - create more utxos first", len(utxoSet), b.batchSize)
		}

		utxoCh := make(chan *bt.UTXO, len(utxoSet))

		for _, utxo := range utxoSet {
			utxoCh <- utxo
		}

		submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond
		submitBatchTicker := time.NewTicker(submitBatchInterval)

		logSummaryTicker := time.NewTicker(3 * time.Second)

		responseCh := make(chan *metamorph_api.TransactionStatus, 100)
		errCh := make(chan error, 100)

		resultsMap := map[metamorph_api.Status]int64{}

		counter := 0

		b.wg.Add(1)
		go func() {

			defer func() {

				b.logger.Info("shutting down broadcaster", slog.String("address", ks.Address(!b.isTestnet)))
				b.wg.Done()
			}()

			for {
				select {
				case <-b.shutdown:
					return
				case <-b.ctx.Done():
					return
				case <-submitBatchTicker.C:

					txs, err := b.createSelfPayingTxs(utxoCh, ks)
					if err != nil {
						b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
						b.shutdown <- struct{}{}
						continue
					}

					go b.broadcastBatch(txs, responseCh, errCh, utxoCh, false, metamorph_api.Status_STORED, limit, ks)

				case <-logSummaryTicker.C:

					total := atomic.LoadInt64(&b.totalTxs)
					b.logger.Info("summary",
						slog.String("address", ks.Address(!b.isTestnet)),
						slog.Int("utxo set length", len(utxoCh)),
						slog.Int64("total", total),
					)

				case responseErr := <-errCh:
					b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
					counter++
				case res := <-responseCh:
					resultsMap[res.Status]++
				}
			}
		}()
	}
	return nil
}

func (b *RateBroadcaster) createSelfPayingTxs(utxos chan *bt.UTXO, ks *keyset.KeySet) ([]*bt.Tx, error) {
	txs := make([]*bt.Tx, 0, b.batchSize)

	for utxo := range utxos {
		tx := bt.NewTx()

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		fee := b.calculateFeeSat(tx)

		if utxo.Satoshis <= fee {
			continue
		}

		err = tx.PayTo(ks.Script, utxo.Satoshis-fee)
		if err != nil {
			return nil, err
		}

		unlockerGetter := unlocker.Getter{PrivateKey: ks.PrivateKey}
		err = tx.FillAllInputs(context.Background(), &unlockerGetter)
		if err != nil {
			return nil, err
		}

		b.mu.Lock()
		b.satoshiMap[tx.TxID()] = tx.Outputs[0].Satoshis
		b.mu.Unlock()
		txs = append(txs, tx)

		if len(txs) >= b.batchSize {
			break
		}
	}

	return txs, nil
}

func (b *RateBroadcaster) broadcastBatch(txs []*bt.Tx, resultCh chan *metamorph_api.TransactionStatus, errCh chan error, utxoCh chan *bt.UTXO, skipFeeValidation bool, waitForStatus metamorph_api.Status, limit int64, ks *keyset.KeySet) {
	if limit > 0 && atomic.LoadInt64(&b.totalTxs) >= limit {
		return
	}

	limitReachedNotified := false

	resp, err := b.client.BroadcastTransactions(b.ctx, txs, waitForStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, skipFeeValidation)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			b.logger.Debug("broadcasting canceled", slog.String("address", ks.Address(!b.isTestnet)))
			return
		}
		errCh <- err
	}

	for _, res := range resp {
		resultCh <- res

		txIDBytes, err := hex.DecodeString(res.Txid)
		if err != nil {
			b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
			continue
		}
		b.mu.Lock()
		newUtxo := &bt.UTXO{
			TxID:          txIDBytes,
			Vout:          0,
			LockingScript: ks.Script,
			Satoshis:      b.satoshiMap[res.Txid],
		}
		utxoCh <- newUtxo

		delete(b.satoshiMap, res.Txid)

		atomic.AddInt64(&b.totalTxs, 1)
		if limit > 0 && atomic.LoadInt64(&b.totalTxs) >= limit && !limitReachedNotified {
			b.logger.Info("limit reached", slog.Int64("total", atomic.LoadInt64(&b.totalTxs)), slog.String("address", ks.Address(!b.isTestnet)))
			b.shutdown <- struct{}{}
			limitReachedNotified = true
		}
		b.mu.Unlock()
	}
}

func (b *RateBroadcaster) Shutdown() {
	if b.cancelAll != nil {
		b.cancelAll()
		b.wg.Wait()
	}
}

//
//ch := make(chan bool)
//var count int32
//go func() {
//	var m runtime.MemStats
//	var writer = uilive.New()
//	writer.Start()
//	for {
//
//		b.logger.Info("summary",
//			slog.String("address", b.fundingKeyset.Address(!b.isTestnet)),
//			slog.Int("utxo set length", len(utxoCh)),
//			slog.Int64("total", b.totalTxs),
//		)
//
//		<-ch
//		_, _ = fmt.Fprintf(writer, "Current connections count: %d\n", atomic.LoadInt32(&count))
//		_, _ = fmt.Fprintf(writer, "Current connections count: %d\n", b.totalTxs.Load())
//		runtime.ReadMemStats(&m)
//		_, _ = fmt.Fprintf(writer, "Alloc = %v MiB\n", m.Alloc/1024/1024)
//		_, _ = fmt.Fprintf(writer, "TotalAlloc = %v MiB\n", m.TotalAlloc/1024/1024)
//		_, _ = fmt.Fprintf(writer, "Sys = %v MiB\n", m.Sys/1024/1024)
//		_, _ = fmt.Fprintf(writer, "NumGC = %v\n", m.NumGC)
//	}
//}()
