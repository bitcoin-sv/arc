package broadcaster

import (
	"container/list"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
)

type UTXOCreator struct {
	Broadcaster
	keySets []*keyset.KeySet
}

func NewUTXOCreator(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, isTestnet bool, opts ...func(p *Broadcaster)) (*UTXOCreator, error) {
	b, err := NewBroadcaster(logger, client, utxoClient, isTestnet, opts...)
	if err != nil {
		return nil, err
	}

	creator := &UTXOCreator{
		Broadcaster: b,
		keySets:     keySets,
	}

	return creator, nil
}

func (b *UTXOCreator) CreateUtxos(requestedOutputs int, requestedSatoshisPerOutput uint64) error {
	for _, ks := range b.keySets {
		b.logger.Info("creating utxos", slog.String("address", ks.Address(!b.isTestnet)))

		requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

		confirmed, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(b.ctx, !b.isTestnet, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return err
		}

		if unconfirmed > 0 {
			return fmt.Errorf("total balance not confirmed yet")
		}

		balance := confirmed + unconfirmed

		if requestedOutputsSatoshis > balance {
			return fmt.Errorf("requested total of satoshis %d exceeds balance on funding keyset %d", requestedOutputsSatoshis, balance)
		}

		utxos, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, !b.isTestnet, ks.Script, ks.Address(!b.isTestnet), 1*time.Second, 5)
		if err != nil {
			return err
		}

		utxoSet := list.New()
		for _, utxo := range utxos {
			// collect right sized utxos
			if utxo.Satoshis >= requestedSatoshisPerOutput {
				utxoSet.PushBack(utxo)
				continue
			}
		}

		// if requested outputs satisfied, return
		if utxoSet.Len() >= requestedOutputs {
			b.logger.Info("utxo set", slog.Int("ready", utxoSet.Len()), slog.Int("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))
			return nil
		}

		satoshiMap := map[string][]splittingOutput{}
		lastUtxoSetLen := 0

		// if requested outputs not satisfied, create them
		for {
			if lastUtxoSetLen >= utxoSet.Len() {
				b.logger.Error("utxo set length hasn't changed since last iteration")
				break
			}
			lastUtxoSetLen = utxoSet.Len()

			// if requested outputs satisfied, return
			if utxoSet.Len() >= requestedOutputs {
				break
			}

			b.logger.Info("splitting outputs", slog.Int("ready", utxoSet.Len()), slog.Int("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))

			// create splitting txs
			txsSplitBatches, err := b.splitOutputs(requestedOutputs, requestedSatoshisPerOutput, utxoSet, satoshiMap, ks)
			if err != nil {
				return err
			}

			for i, batch := range txsSplitBatches {
				nrOutputs := 0
				nrInputs := 0
				for _, txBatch := range batch {
					nrOutputs += len(txBatch.Outputs)
					nrInputs += len(txBatch.Inputs)
				}

				b.logger.Info(fmt.Sprintf("broadcasting splitting batch %d/%d", i+1, len(txsSplitBatches)), slog.Int("size", len(batch)), slog.Int("inputs", nrInputs), slog.Int("outputs", nrOutputs))

				resp, err := b.client.BroadcastTransactions(context.Background(), batch, metamorph_api.Status_SEEN_ON_NETWORK, b.callbackURL, b.callbackToken, b.fullStatusUpdates, false)
				if err != nil {
					return fmt.Errorf("failed to braodcast tx: %v", err)
				}

				for _, res := range resp {
					if res.Status == metamorph_api.Status_REJECTED || res.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
						b.logger.Error("splitting tx was not successful", slog.String("status", res.Status.String()), slog.String("hash", res.Txid), slog.String("reason", res.RejectReason))
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

					foundOutputs, found := satoshiMap[res.Txid]
					if !found {
						b.logger.Error("output not found", slog.String("hash", res.Txid))
						continue
					}

					for _, foundOutput := range foundOutputs {
						newUtxo := &bt.UTXO{
							TxID:          txIDBytes,
							Vout:          foundOutput.vout,
							LockingScript: ks.Script,
							Satoshis:      foundOutput.satoshis,
						}

						utxoSet.PushBack(newUtxo)
					}
					delete(satoshiMap, res.Txid)
				}

				// do not performance test ARC when creating the utxos
				time.Sleep(100 * time.Millisecond)
			}
		}

		b.logger.Info("utxo set", slog.Int("ready", utxoSet.Len()), slog.Int("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))
	}
	return nil
}

func (b *UTXOCreator) splitOutputs(requestedOutputs int, requestedSatoshisPerOutput uint64, utxoSet *list.List, satoshiMap map[string][]splittingOutput, fundingKeySet *keyset.KeySet) ([][]*bt.Tx, error) {
	txsSplitBatches := make([][]*bt.Tx, 0)
	txsSplit := make([]*bt.Tx, 0)
	outputs := utxoSet.Len()
	var err error

	var next *list.Element
	for front := utxoSet.Front(); front != nil; front = next {
		next = front.Next()

		if front == nil || outputs >= requestedOutputs {
			break
		}

		utxo, ok := front.Value.(*bt.UTXO)
		if !ok {
			return nil, errors.New("failed to parse value to utxo")
		}

		tx := bt.NewTx()
		err = tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		// only split if splitting increases nr of outputs
		const feeMargin = 50
		if utxo.Satoshis < 2*requestedSatoshisPerOutput+feeMargin {
			continue
		}

		addedOutputs, err := b.splitToFundingKeyset(tx, utxo.Satoshis, requestedSatoshisPerOutput, requestedOutputs-outputs, fundingKeySet)
		if err != nil {
			return nil, err
		}
		utxoSet.Remove(front)

		outputs += addedOutputs

		txsSplit = append(txsSplit, tx)

		txOutputs := make([]splittingOutput, len(tx.Outputs))
		for i, txOutput := range tx.Outputs {
			txOutputs[i] = splittingOutput{satoshis: txOutput.Satoshis, vout: uint32(i)}
		}

		satoshiMap[tx.TxID()] = txOutputs

		if len(txsSplit) == b.batchSize {
			txsSplitBatches = append(txsSplitBatches, txsSplit)
			txsSplit = make([]*bt.Tx, 0)
		}
	}

	if len(txsSplit) > 0 {
		txsSplitBatches = append(txsSplitBatches, txsSplit)
	}
	return txsSplitBatches, nil
}

func (b *UTXOCreator) splitToFundingKeyset(tx *bt.Tx, splitSatoshis uint64, requestedSatoshis uint64, requestedOutputs int, fundingKeySet *keyset.KeySet) (addedOutputs int, err error) {
	if requestedSatoshis > splitSatoshis {
		return 0, fmt.Errorf("requested satoshis %d greater than satoshis to be split %d", requestedSatoshis, splitSatoshis)
	}

	counter := 0

	remaining := int64(splitSatoshis)
	for remaining > int64(requestedSatoshis) && counter < requestedOutputs {
		if uint64(remaining)-requestedSatoshis < b.calculateFeeSat(tx) {
			break
		}

		err := tx.PayTo(fundingKeySet.Script, requestedSatoshis)
		if err != nil {
			return 0, err
		}

		remaining -= int64(requestedSatoshis)
		counter++

	}

	fee := b.calculateFeeSat(tx)
	err = tx.PayTo(fundingKeySet.Script, uint64(remaining)-fee)
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: fundingKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return 0, err
	}

	return counter, nil
}

type splittingOutput struct {
	satoshis uint64
	vout     uint32
}
