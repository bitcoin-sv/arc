package broadcaster

import (
	"container/list"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
)

type UTXOConsolidator struct {
	Broadcaster
}

func NewUTXOConsolidator(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*UTXOConsolidator, error) {

	b, err := NewBroadcaster(logger, client, keySets, utxoClient, opts...)
	if err != nil {
		return nil, err
	}

	consolidator := &UTXOConsolidator{
		Broadcaster: *b,
	}

	return consolidator, nil
}

func (b *UTXOConsolidator) Consolidate() error {
	for _, ks := range b.keySets {
		b.logger.Info("consolidating utxos", slog.String("address", ks.Address(!b.isTestnet)))
		_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(b.ctx, !b.isTestnet, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return err
		}
		if math.Abs(float64(unconfirmed)) > 0 {
			return fmt.Errorf("key with address %s balance has unconfirmed amount %d sat", ks.Address(!b.isTestnet), unconfirmed)
		}

		utxoSet, err := b.utxoClient.GetUTXOsListWithRetries(b.ctx, !b.isTestnet, ks.Script, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return fmt.Errorf("failed to get utxos: %v", err)
		}

		if utxoSet.Len() == 1 {
			b.logger.Info("utxos already consolidated")
			return nil
		}

		satoshiMap := map[string]uint64{}
		lastUtxoSetLen := 100_000_000

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

			consolidationTxsBatches, err := b.createConsolidationTxs(utxoSet, satoshiMap, ks)
			if err != nil {
				return fmt.Errorf("failed to create consolidation txs: %v", err)
			}

			for i, batch := range consolidationTxsBatches {
				time.Sleep(100 * time.Millisecond) // do not performance test ARC

				nrOutputs := 0
				nrInputs := 0
				for _, txBatch := range batch {
					nrOutputs += len(txBatch.Outputs)
					nrInputs += len(txBatch.Inputs)
				}

				b.logger.Info(fmt.Sprintf("broadcasting consolidation batch %d/%d", i+1, len(consolidationTxsBatches)), slog.Int("size", len(batch)), slog.Int("inputs", nrInputs), slog.Int("outputs", nrOutputs))

				resp, err := b.client.BroadcastTransactions(context.Background(), batch, metamorph_api.Status_SEEN_ON_NETWORK, b.callbackURL, b.callbackToken, b.fullStatusUpdates, false)
				if err != nil {
					return fmt.Errorf("failed to broadcast consolidation txs: %v", err)
				}

				for _, res := range resp {
					if res.Status == metamorph_api.Status_REJECTED || res.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
						b.logger.Error("consolidation tx was not successful", slog.String("status", res.Status.String()), slog.String("hash", res.Txid), slog.String("reason", res.RejectReason))
						for _, tx := range batch {
							if tx.TxID() == res.Txid {
								b.logger.Debug(tx.String())
								break
							}
						}
						continue
					}

					txIDBytes, err := hex.DecodeString(res.Txid)
					if err != nil {
						b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
						continue
					}

					newUtxo := &bt.UTXO{
						TxID:          txIDBytes,
						Vout:          0,
						LockingScript: ks.Script,
						Satoshis:      satoshiMap[res.Txid],
					}

					delete(satoshiMap, res.Txid)

					utxoSet.PushBack(newUtxo)
				}
			}
		}
	}
	return nil
}

func (b *UTXOConsolidator) createConsolidationTxs(utxoSet *list.List, satoshiMap map[string]uint64, fundingKeySet *keyset.KeySet) ([][]*bt.Tx, error) {
	tx := bt.NewTx()
	txSatoshis := uint64(0)
	txsConsolidationBatches := make([][]*bt.Tx, 0)
	txsConsolidation := make([]*bt.Tx, 0)
	const consolidateBatchSize = 20

	var next *list.Element
	for front := utxoSet.Front(); front != nil; front = next {
		next = front.Next()
		utxoSet.Remove(front)
		utxo, ok := front.Value.(*bt.UTXO)
		if !ok {
			return nil, errors.New("failed to parse value to utxo")
		}

		txSatoshis += utxo.Satoshis
		if next == nil {
			if len(tx.Inputs) > 0 {
				err := tx.FromUTXOs(utxo)
				if err != nil {
					return nil, err
				}

				err = b.consolidateToFundingKeyset(tx, txSatoshis, fundingKeySet)
				if err != nil {
					return nil, err
				}

				txsConsolidation = append(txsConsolidation, tx)
				satoshiMap[tx.TxID()] = tx.TotalOutputSatoshis()
			}

			if len(txsConsolidation) > 0 {
				txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			}
			break
		}

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		if len(tx.Inputs) >= b.maxInputs {
			err = b.consolidateToFundingKeyset(tx, txSatoshis, fundingKeySet)
			if err != nil {
				return nil, err
			}

			txsConsolidation = append(txsConsolidation, tx)

			satoshiMap[tx.TxID()] = tx.TotalOutputSatoshis()
			tx = bt.NewTx()
			txSatoshis = 0
		}

		if len(txsConsolidation) >= consolidateBatchSize {
			txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			txsConsolidation = make([]*bt.Tx, 0)
		}

	}

	return txsConsolidationBatches, nil
}

func (b *UTXOConsolidator) consolidateToFundingKeyset(tx *bt.Tx, txSatoshis uint64, fundingKeySet *keyset.KeySet) error {
	fee := b.calculateFeeSat(tx)
	err := tx.PayTo(fundingKeySet.Script, txSatoshis-fee)
	if err != nil {
		return err
	}
	unlockerGetter := unlocker.Getter{PrivateKey: fundingKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}
	return nil
}
