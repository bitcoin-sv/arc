package broadcaster

import (
	"container/list"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gosuri/uilive"
	"log/slog"
	"math"
	"runtime"
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
	wg              sync.WaitGroup
	totalTxs        int64
	connectionCount int32
	shutdown        chan struct{}
	connectionCh    chan struct{}
	utxoCh          chan *bt.UTXO
	mu              sync.RWMutex
	satoshiMap      map[string]uint64
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*RateBroadcaster, error) {

	b, err := NewBroadcaster(logger, client, keySets, utxoClient, opts...)
	if err != nil {
		return nil, err
	}

	rb := &RateBroadcaster{
		Broadcaster:  *b,
		totalTxs:     0,
		mu:           sync.RWMutex{},
		shutdown:     make(chan struct{}, 10),
		connectionCh: make(chan struct{}, 10),
		satoshiMap:   map[string]uint64{},
		wg:           sync.WaitGroup{},
	}

	rb.startPrintStats()

	go func() {
		for {
			select {
			case <-rb.shutdown:
				rb.cancelAll()
			case <-rb.ctx.Done():
				return
			}
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

		b.utxoCh = make(chan *bt.UTXO, len(utxoSet))

		for _, utxo := range utxoSet {
			utxoCh <- utxo
		}

		submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond
		submitBatchTicker := time.NewTicker(submitBatchInterval)

		responseCh := make(chan *metamorph_api.TransactionStatus, 100)
		errCh := make(chan error, 100)

		resultsMap := map[metamorph_api.Status]int64{}

		counter := 0

		b.wg.Add(1)
		go func(keySet *keyset.KeySet) {
			defer func() {
				b.logger.Info("shutting down broadcaster", slog.String("address", keySet.Address(!b.isTestnet)))
				b.wg.Done()
			}()

			for {
				select {
				case <-b.shutdown:
					return
				case <-b.ctx.Done():
					return
				case <-submitBatchTicker.C:

					txs, err := b.createSelfPayingTxs(b.utxoCh, keySet)
					if err != nil {
						b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
						b.shutdown <- struct{}{}
						continue
					}

					go b.broadcastBatch(txs, responseCh, errCh, b.utxoCh, false, metamorph_api.Status_RECEIVED, limit, keySet)

				case responseErr := <-errCh:
					b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
					counter++
				case res := <-responseCh:
					resultsMap[res.Status]++
				}
			}
		}(ks)
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

	b.connectionCh <- struct{}{}

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
		b.mu.Unlock()

		atomic.AddInt64(&b.totalTxs, 1)
		if limit > 0 && atomic.LoadInt64(&b.totalTxs) >= limit && !limitReachedNotified {
			b.logger.Info("limit reached", slog.Int64("total", atomic.LoadInt64(&b.totalTxs)), slog.String("address", ks.Address(!b.isTestnet)))
			b.shutdown <- struct{}{}
			limitReachedNotified = true
		}
	}
}

func (b *RateBroadcaster) Shutdown() {
	b.cancelAll()
	b.wg.Wait()
}

func (b *RateBroadcaster) startPrintStats() {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		var m runtime.MemStats
		var writer = uilive.New()
		writer.Start()
		for {
			select {
			case <-b.connectionCh:
				_, _ = fmt.Fprintf(writer, "Current connections count: %d\n", atomic.LoadInt32(&b.connectionCount))
				_, _ = fmt.Fprintf(writer, "Tx count: %d\n", b.totalTxs.Load())
				_, _ = fmt.Fprintf(writer, "UTXO set length: %d\n", len(b.utxoCh))
				runtime.ReadMemStats(&m)
				_, _ = fmt.Fprintf(writer, "Alloc = %v MiB\n", m.Alloc/1024/1024)
				_, _ = fmt.Fprintf(writer, "TotalAlloc = %v MiB\n", m.TotalAlloc/1024/1024)
				_, _ = fmt.Fprintf(writer, "Sys = %v MiB\n", m.Sys/1024/1024)
				_, _ = fmt.Fprintf(writer, "NumGC = %v\n", m.NumGC)
			case <-b.ctx.Done():
				return
			}
		}
	}()
}
