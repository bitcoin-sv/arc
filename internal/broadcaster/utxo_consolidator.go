package broadcaster

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"log/slog"
	"sync"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

var (
	ErrFailedToParseValueToUTXO = errors.New("failed to parse value to utxo")
)

type UTXOConsolidator struct {
	Broadcaster
	keySet *keyset.KeySet
	wg     sync.WaitGroup
}

func NewUTXOConsolidator(logger *slog.Logger, client ArcClient, keySet *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*UTXOConsolidator, error) {
	b, err := NewBroadcaster(logger, client, utxoClient, opts...)
	if err != nil {
		return nil, err
	}

	consolidator := &UTXOConsolidator{
		Broadcaster: b,
		keySet:      keySet,
	}

	return consolidator, nil
}

func (b *UTXOConsolidator) Wait() {
	b.wg.Wait()
}

func (b *UTXOConsolidator) Start(txsRateTxsPerMinute int) error {
	submitBatchesPerMinute := float64(txsRateTxsPerMinute) / float64(b.batchSize)

	submitBatchInterval := time.Duration(millisecondsPerSecond*60/float64(submitBatchesPerMinute)) * time.Millisecond
	utxos, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, b.keySet.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return errors.Join(ErrFailedToGetUTXOs, err)
	}

	utxoSet := list.New()
	for _, utxo := range utxos {
		utxoSet.PushBack(utxo)
	}

	if utxoSet.Len() == 1 {
		b.logger.Info("utxos already consolidated")
		return nil
	}

	satoshiMap := map[string]uint64{}
	lastUtxoSetLen := 100_000_000

	b.logger.Info("starting consolidator", slog.String("batch interval", submitBatchInterval.String()))

	b.wg.Add(1)
	go func() {
		defer func() {
			b.logger.Info("shutting down")
			b.wg.Done()
		}()

		for {
			if lastUtxoSetLen <= utxoSet.Len() {
				b.logger.Error("utxo set length hasn't changed since last iteration")
				break
			}
			lastUtxoSetLen = utxoSet.Len()

			// if requested outputs satisfied, return
			if utxoSet.Len() == 1 {
				break
			}

			b.logger.Info("consolidating outputs", slog.Int("remaining", utxoSet.Len()))

			consolidationTxsBatches, err := b.createConsolidationTxs(utxoSet, satoshiMap, b.keySet)
			if err != nil {
				b.logger.Error("failed to create consolidation txs", slog.String("err", err.Error()))
				return
			}

			b.processConsolidationTxsBatchesBroadcastResponse(consolidationTxsBatches, satoshiMap, utxoSet, submitBatchInterval)
		}
	}()

	return nil
}

func (b *UTXOConsolidator) createConsolidationTxs(utxoSet *list.List, satoshiMap map[string]uint64, fundingKeySet *keyset.KeySet) ([]sdkTx.Transactions, error) {
	tx := sdkTx.NewTransaction()
	txSatoshis := uint64(0)
	txsConsolidationBatches := make([]sdkTx.Transactions, 0)
	txsConsolidation := make(sdkTx.Transactions, 0)
	const consolidateBatchSize = 20

	var next *list.Element
	for front := utxoSet.Front(); front != nil; front = next {
		next = front.Next()
		utxoSet.Remove(front)
		utxo, ok := front.Value.(*sdkTx.UTXO)
		if !ok {
			return nil, ErrFailedToParseValueToUTXO
		}

		txSatoshis += utxo.Satoshis
		if next == nil {
			if len(tx.Inputs) > 0 {
				err := tx.AddInputsFromUTXOs(utxo)
				if err != nil {
					return nil, err
				}

				err = b.consolidateToFundingKeyset(&txsConsolidation, tx, txSatoshis, fundingKeySet, satoshiMap)
				if err != nil {
					return nil, err
				}
			}

			if len(txsConsolidation) > 0 {
				txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			}
			break
		}

		err := tx.AddInputsFromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		if len(tx.Inputs) >= b.maxInputs {
			err = b.consolidateToFundingKeyset(&txsConsolidation, tx, txSatoshis, fundingKeySet, satoshiMap)
			if err != nil {
				return nil, err
			}

			tx = sdkTx.NewTransaction()
			txSatoshis = 0
		}

		if len(txsConsolidation) >= consolidateBatchSize {
			txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			txsConsolidation = make(sdkTx.Transactions, 0)
		}
	}

	return txsConsolidationBatches, nil
}

func (b *UTXOConsolidator) consolidateToFundingKeyset(txsConsolidation *sdkTx.Transactions, tx *sdkTx.Transaction, txSatoshis uint64, fundingKeySet *keyset.KeySet, satoshiMap map[string]uint64) error {
	fee, err := ComputeFee(tx, b.feeModel)
	if err != nil {
		return err
	}

	err = PayTo(tx, fundingKeySet.Script, txSatoshis-fee)
	if err != nil {
		return err
	}

	err = SignAllInputs(tx, fundingKeySet.PrivateKey)
	if err != nil {
		return err
	}

	(*txsConsolidation) = append((*txsConsolidation), tx)
	satoshiMap[tx.TxID().String()] = tx.TotalOutputSatoshis()

	return nil
}

func (b *UTXOConsolidator) Shutdown() {
	b.cancelAll()

	b.wg.Wait()
}

func (b *UTXOConsolidator) processBroadcastResponse(resp []*metamorph_api.TransactionStatus, batch sdkTx.Transactions, satoshiMap map[string]uint64, utxoSet *list.List) {
	for _, res := range resp {
		if res.Status != metamorph_api.Status_SEEN_ON_NETWORK &&
			res.Status != metamorph_api.Status_ACCEPTED_BY_NETWORK &&
			res.Status != metamorph_api.Status_SENT_TO_NETWORK {
			b.logger.Error("consolidation tx was not successful", slog.String("status", res.Status.String()), slog.String("hash", res.Txid), slog.String("reason", res.RejectReason))
			for _, tx := range batch {
				if tx.TxID().String() == res.Txid {
					b.logger.Debug(tx.String())
					break
				}
			}
			return
		}
		hash, err := chainhash.NewHashFromHex(res.Txid)
		if err != nil {
			b.logger.Error("failed to create chainhash txid", slog.String("err", err.Error()))
			return
		}

		newUtxo := &sdkTx.UTXO{
			TxID:          hash,
			Vout:          0,
			LockingScript: b.keySet.Script,
			Satoshis:      satoshiMap[res.Txid],
		}

		delete(satoshiMap, res.Txid)

		utxoSet.PushBack(newUtxo)
	}
}

func (b *UTXOConsolidator) processConsolidationTxsBatchesBroadcastResponse(consolidationTxsBatches []sdkTx.Transactions, satoshiMap map[string]uint64, utxoSet *list.List, submitBatchInterval time.Duration) {
	for i, batch := range consolidationTxsBatches {
		time.Sleep(submitBatchInterval)

		nrOutputs := 0
		nrInputs := 0
		for _, txBatch := range batch {
			nrOutputs += len(txBatch.Outputs)
			nrInputs += len(txBatch.Inputs)
		}

		b.logger.Info(fmt.Sprintf("broadcasting consolidation batch %d/%d", i+1, len(consolidationTxsBatches)), slog.Int("size", len(batch)), slog.Int("inputs", nrInputs), slog.Int("outputs", nrOutputs))

		resp, err := b.client.BroadcastTransactions(b.ctx, batch, metamorph_api.Status_SEEN_ON_NETWORK, "", "", false, false)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			b.logger.Error("failed to broadcast consolidation txs", slog.String("err", err.Error()))
			return
		}
		b.processBroadcastResponse(resp, batch, satoshiMap, utxoSet)
	}
}
