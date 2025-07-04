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
	"github.com/ccoveille/go-safecast"
)

var (
	ErrRequestedSatoshisTooHigh      = errors.New("requested total of satoshis exceeds balance")
	ErrRequestedSatoshisExceedsSplit = errors.New("requested satoshis greater than satoshis to be split")
)

type UTXOCreator struct {
	Broadcaster
	keySet *keyset.KeySet
	wg     sync.WaitGroup
}
type splittingOutput struct {
	satoshis uint64
	vout     uint32
}

func NewUTXOCreator(logger *slog.Logger, client ArcClient, keySet *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*UTXOCreator, error) {
	b, err := NewBroadcaster(logger, client, utxoClient, opts...)
	if err != nil {
		return nil, err
	}

	creator := &UTXOCreator{
		Broadcaster: b,
		keySet:      keySet,
	}

	return creator, nil
}

func (b *UTXOCreator) Wait() {
	b.wg.Wait()
}
func (b *UTXOCreator) Start(requestedOutputs uint64, requestedSatoshisPerOutput uint64) error {
	// Use a goroutine for concurrent execution

	b.logger.Info("creating utxos", slog.String("address", b.keySet.Address(!b.isTestnet)))

	requestedOutputsSatoshis := requestedOutputs * requestedSatoshisPerOutput

	confirmed, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(b.ctx, b.keySet.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return errors.Join(ErrFailedToGetBalance, err)
	}

	balance := confirmed + unconfirmed

	if requestedOutputsSatoshis > balance {
		return errors.Join(ErrRequestedSatoshisTooHigh, fmt.Errorf("requested: %d, balance: %d", requestedOutputsSatoshis, balance))
	}

	utxos, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, b.keySet.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return errors.Join(ErrFailedToGetUTXOs, err)
	}

	utxoSet := list.New()
	// Increment the WaitGroup counter
	b.wg.Add(1)
	go func() {
		defer func() {
			// Log the shutdown message
			b.logger.Info("shutting down")
			// Mark this goroutine as done
			b.wg.Done()
		}()
		err := b.collectRightSizedUTXOs(utxos, utxoSet, requestedOutputsSatoshis, requestedOutputs)
		if err != nil {
			return
		}

		satoshiMap := map[string][]splittingOutput{}
		lastUtxoSetLen := 0
		// if requested outputs not satisfied, create them
		err = b.createRightSizedUTXOs(lastUtxoSetLen, satoshiMap, utxoSet, requestedOutputs, requestedSatoshisPerOutput)
		if err != nil {
			return
		}

		b.logger.Info("utxo set creation completed", slog.Int("ready", utxoSet.Len()), slog.Uint64("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))
	}()

	return nil
}

func (b *UTXOCreator) splitOutputs(requestedOutputs uint64, requestedSatoshisPerOutput uint64, utxoSet *list.List, satoshiMap map[string][]splittingOutput, fundingKeySet *keyset.KeySet) ([]sdkTx.Transactions, error) {
	txsSplitBatches := make([]sdkTx.Transactions, 0)
	txsSplit := make(sdkTx.Transactions, 0)
	outputs, err := safecast.ToUint64(utxoSet.Len())
	if err != nil {
		return nil, err
	}

	var next *list.Element
	for front := utxoSet.Front(); front != nil; front = next {
		next = front.Next()

		if outputs >= requestedOutputs {
			break
		}

		utxo, ok := front.Value.(*sdkTx.UTXO)
		if !ok {
			return nil, ErrFailedToParseValueToUTXO
		}

		tx := sdkTx.NewTransaction()
		err = tx.AddInputsFromUTXOs(utxo)
		if err != nil {
			return nil, fmt.Errorf("failed to add inputs from UTXOs: %w", err)
		}
		// only split if splitting increases nr of outputs
		const feeMargin = 50
		if utxo.Satoshis < 2*requestedSatoshisPerOutput+feeMargin {
			continue
		}

		addedOutputs, err := b.splitToFundingKeyset(tx, utxo.Satoshis, requestedSatoshisPerOutput, requestedOutputs-outputs, fundingKeySet)
		if err != nil {
			return nil, fmt.Errorf("failed to split to funding keyset: %w", err)
		}
		utxoSet.Remove(front)

		outputs += addedOutputs
		txsSplit = append(txsSplit, tx)

		txOutputs := make([]splittingOutput, len(tx.Outputs))
		for i, txOutput := range tx.Outputs {
			voutUint32, err := safecast.ToUint32(i)
			if err != nil {
				return nil, fmt.Errorf("failed to convert int to uint32: %w", err)
			}
			txOutputs[i] = splittingOutput{satoshis: txOutput.Satoshis, vout: voutUint32}
		}

		satoshiMap[tx.TxID().String()] = txOutputs

		if len(txsSplit) == b.batchSize {
			txsSplitBatches = append(txsSplitBatches, txsSplit)
			txsSplit = make(sdkTx.Transactions, 0)
		}
	}

	if len(txsSplit) > 0 {
		txsSplitBatches = append(txsSplitBatches, txsSplit)
	}
	return txsSplitBatches, nil
}

func (b *UTXOCreator) splitToFundingKeyset(tx *sdkTx.Transaction, splitSatoshis, requestedSatoshis uint64, requestedOutputs uint64, fundingKeySet *keyset.KeySet) (uint64, error) {
	if requestedSatoshis > splitSatoshis {
		return 0, errors.Join(ErrRequestedSatoshisExceedsSplit, fmt.Errorf("requested: %d, split: %d", requestedSatoshis, splitSatoshis))
	}

	counter := uint64(0)
	var err error
	var fee uint64

	remaining := splitSatoshis

	for remaining > requestedSatoshis && counter < requestedOutputs {
		fee, err = ComputeFee(tx, b.feeModel)
		if err != nil {
			return 0, err
		}
		if uint64(remaining)-requestedSatoshis < fee {
			break
		}

		err = PayTo(tx, fundingKeySet.Script, requestedSatoshis)
		if err != nil {
			return 0, errors.Join(ErrFailedToAddOutput, err)
		}

		remaining -= requestedSatoshis
		counter++
	}

	fee, err = ComputeFee(tx, b.feeModel)
	if err != nil {
		return 0, err
	}

	err = PayTo(tx, fundingKeySet.Script, uint64(remaining)-fee)
	if err != nil {
		return 0, errors.Join(ErrFailedToAddOutput, err)
	}

	err = SignAllInputs(tx, fundingKeySet.PrivateKey)
	if err != nil {
		return 0, err
	}

	return counter, nil
}

func (b *UTXOCreator) Shutdown() {
	b.cancelAll()
	b.wg.Wait()
}

func (b *UTXOCreator) collectRightSizedUTXOs(utxos sdkTx.UTXOs, utxoSet *list.List, requestedSatoshisPerOutput uint64, requestedOutputs uint64) error {
	for _, utxo := range utxos {
		// collect right sized utxos
		if utxo.Satoshis >= requestedSatoshisPerOutput {
			utxoSet.PushBack(utxo)
		}
	}
	// if requested outputs satisfied, return
	utxoLen, err := safecast.ToUint64(utxoSet.Len())
	if err != nil {
		b.logger.Error("failed to convert utxo set length to uint64", slog.String("err", err.Error()))
		return err
	}
	if utxoLen >= requestedOutputs {
		b.logger.Info("utxo set", slog.Int("ready", utxoSet.Len()), slog.Uint64("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))
		return errors.New("utxo set ready")
	}
	return nil
}

func (b *UTXOCreator) createRightSizedUTXOs(lastUtxoSetLen int, satoshiMap map[string][]splittingOutput, utxoSet *list.List, requestedOutputs uint64, requestedSatoshisPerOutput uint64) error {
	for {
		if lastUtxoSetLen >= utxoSet.Len() {
			b.logger.Error("utxo set length hasn't changed since last iteration")
			break
		}
		lastUtxoSetLen = utxoSet.Len()
		// if requested outputs satisfied, return

		utxoLen, err := safecast.ToUint64(utxoSet.Len())
		if err != nil {
			b.logger.Error("failed to convert utxo set length to uint64", slog.String("err", err.Error()))
			return err
		}
		if utxoLen >= requestedOutputs {
			break
		}

		b.logger.Info("splitting outputs", slog.Int("ready", utxoSet.Len()), slog.Uint64("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput))
		// create splitting txs

		txsSplitBatches, err := b.splitOutputs(requestedOutputs, requestedSatoshisPerOutput, utxoSet, satoshiMap, b.keySet)
		if err != nil {
			b.logger.Error("failed to split outputs", slog.String("err", err.Error()))
			return err
		}

		for i, batch := range txsSplitBatches {
			nrOutputs, nrInputs := 0, 0
			for _, txBatch := range batch {
				nrOutputs += len(txBatch.Outputs)
				nrInputs += len(txBatch.Inputs)
			}

			b.logger.Info(fmt.Sprintf("broadcasting splitting batch %d/%d", i+1, len(txsSplitBatches)), slog.Int("size", len(batch)), slog.Int("inputs", nrInputs), slog.Int("outputs", nrOutputs))

			resp, err := b.client.BroadcastTransactions(context.Background(), batch, metamorph_api.Status_SEEN_ON_NETWORK, "", "", false, false)
			if err != nil {
				b.logger.Error("failed to broadcast transactions", slog.String("err", err.Error()))
				return err
			}

			b.processBroadcastResponse(resp, satoshiMap, utxoSet, batch)
			// do not performance test ARC when creating the utxos
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func (b *UTXOCreator) processBroadcastResponse(resp []*metamorph_api.TransactionStatus, satoshiMap map[string][]splittingOutput, utxoSet *list.List, batch sdkTx.Transactions) {
	for _, res := range resp {
		if res.Status == metamorph_api.Status_REJECTED || res.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
			b.logger.Error("splitting tx was not successful", slog.String("status", res.Status.String()), slog.String("hash", res.Txid), slog.String("reason", res.RejectReason))
			for _, tx := range batch {
				if tx.TxID().String() == res.Txid {
					b.logger.Debug(tx.String())
					break
				}
			}
			continue
		}

		foundOutputs, found := satoshiMap[res.Txid]
		if !found {
			b.logger.Error("output not found", slog.String("hash", res.Txid))
			continue
		}

		hash, err := chainhash.NewHashFromHex(res.Txid)
		if err != nil {
			b.logger.Error("failed to create chainhash txid", slog.String("err", err.Error()))
			continue
		}
		for _, foundOutput := range foundOutputs {
			newUtxo := &sdkTx.UTXO{
				TxID:          hash,
				Vout:          foundOutput.vout,
				LockingScript: b.keySet.Script,
				Satoshis:      foundOutput.satoshis,
			}

			utxoSet.PushBack(newUtxo)
		}
		delete(satoshiMap, res.Txid)
	}
}
