package broadcaster

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
)

const (
	maxInputsDefault = 100
	batchSizeDefault = 20
	isTestnetDefault = true
)

type UtxoClient interface {
	GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
	GetBalance(mainnet bool, address string) (int64, error)
}

type RateBroadcaster struct {
	logger            *slog.Logger
	client            ArcClient
	fromKeySet        *keyset.KeySet
	toKeySet          *keyset.KeySet
	Outputs           int64
	SatoshisPerOutput uint64
	isTestnet         bool
	CallbackURL       string
	feeQuote          *bt.FeeQuote
	utxoClient        UtxoClient
	standardMiningFee bt.FeeUnit

	maxInputs int
	batchSize int
}

func WithFees(miningFeeSatPerKb int) func(preparer *RateBroadcaster) {
	return func(preparer *RateBroadcaster) {
		var fq = bt.NewFeeQuote()

		newStdFee := *stdFeeDefault
		newDataFee := *dataFeeDefault

		newStdFee.MiningFee.Satoshis = miningFeeSatPerKb
		newDataFee.MiningFee.Satoshis = miningFeeSatPerKb

		fq.AddQuote(bt.FeeTypeData, &newStdFee)
		fq.AddQuote(bt.FeeTypeStandard, &newDataFee)

		preparer.feeQuote = fq
	}
}

func WithBatchSize(batchSize int) func(preparer *RateBroadcaster) {
	return func(preparer *RateBroadcaster) {
		preparer.batchSize = batchSize
	}
}

func WithMaxInputs(maxInputs int) func(preparer *RateBroadcaster) {
	return func(preparer *RateBroadcaster) {
		preparer.maxInputs = maxInputs
	}
}

func WithIsTestnet(isTestnet bool) func(preparer *RateBroadcaster) {
	return func(preparer *RateBroadcaster) {
		preparer.isTestnet = isTestnet
	}
}

func WithCallbackURL(callbackURL string) func(preparer *RateBroadcaster) {
	return func(preparer *RateBroadcaster) {
		preparer.CallbackURL = callbackURL
	}
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *RateBroadcaster)) (*RateBroadcaster, error) {
	broadcaster := &RateBroadcaster{
		logger:     logger,
		client:     client,
		fromKeySet: fromKeySet,
		toKeySet:   toKeyset,
		isTestnet:  isTestnetDefault,
		feeQuote:   bt.NewFeeQuote(),
		utxoClient: utxoClient,
		batchSize:  batchSizeDefault,
		maxInputs:  maxInputsDefault,
	}

	for _, opt := range opts {
		opt(broadcaster)
	}

	standardFee, err := broadcaster.feeQuote.Fee(bt.FeeTypeStandard)
	if err != nil {
		return nil, err
	}

	broadcaster.standardMiningFee = standardFee.MiningFee

	return broadcaster, nil
}

// Payback sends all funds currently held on the receiving address back to the funding address
func (b *RateBroadcaster) Payback() error {
	utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.toKeySet.Script, b.toKeySet.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	tx := bt.NewTx()
	txSatoshis := uint64(0)
	batchSatoshis := uint64(0)
	if err != nil {
		return err
	}

	txs := make([]*bt.Tx, 0, b.batchSize)

	for _, utxo := range utxos {

		err = tx.FromUTXOs(utxo)
		if err != nil {
			return err
		}

		txSatoshis += utxo.Satoshis

		// create payback transactions with maximum 100 inputs
		if len(tx.Inputs) >= b.maxInputs {
			batchSatoshis += txSatoshis

			err = b.addSingleOutput(tx, txSatoshis)
			if err != nil {
				return err
			}

			txs = append(txs, tx)

			tx = bt.NewTx()
			txSatoshis = 0
		}

		if len(txs) == b.batchSize {
			err = b.submitTxs(txs, metamorph_api.Status_SEEN_ON_NETWORK)
			if err != nil {
				return err
			}
			b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))

			batchSatoshis = 0
			txs = make([]*bt.Tx, 0, b.batchSize)
			time.Sleep(time.Millisecond * 100)
		}
	}

	if len(tx.Inputs) > 0 {
		batchSatoshis += txSatoshis

		err = b.addSingleOutput(tx, txSatoshis)
		if err != nil {
			return err
		}

		txs = append(txs, tx)
	}

	if len(txs) > 0 {
		err = b.submitTxs(txs, metamorph_api.Status_SEEN_ON_NETWORK)
		if err != nil {
			return err
		}

		b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))
	}

	return nil
}

func (b *RateBroadcaster) getFeeSat(sizeBytes int) uint64 {
	return uint64(math.Ceil(float64(sizeBytes)/float64(b.standardMiningFee.Bytes)) * float64(b.standardMiningFee.Satoshis))
}

func (b *RateBroadcaster) addSingleOutput(tx *bt.Tx, totalSatoshis uint64) error {

	err := tx.PayTo(b.fromKeySet.Script, totalSatoshis-b.getFeeSat(tx.Size()))
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.toKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}

	return nil
}

func (b *RateBroadcaster) addOutputsConsolidate(tx *bt.Tx, totalSatoshis uint64, requestedSatoshis uint64) (addedOutputs int, err error) {

	if requestedSatoshis > totalSatoshis {
		err := b.addSingleOutput(tx, totalSatoshis)
		if err != nil {
			return 0, err
		}

		return 0, nil
	}

	err = tx.PayTo(b.fromKeySet.Script, requestedSatoshis)
	if err != nil {
		return 0, err
	}

	err = tx.PayTo(b.fromKeySet.Script, totalSatoshis-requestedSatoshis-b.getFeeSat(tx.Size()))
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.toKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return 0, err
	}

	return 1, nil
}

func (b *RateBroadcaster) addOutputsSplit(tx *bt.Tx, splitSatoshis uint64, requestedSatoshis uint64, requestedOutputs int) (addedOutputs int, err error) {

	if requestedSatoshis > splitSatoshis {
		return 0, fmt.Errorf("requested satoshis %d greater than satoshis to be split %d", requestedSatoshis, splitSatoshis)
	}

	counter := 0

	remaining := int64(splitSatoshis)
	for remaining > int64(requestedSatoshis) && counter < requestedOutputs {

		err := tx.PayTo(b.fromKeySet.Script, requestedSatoshis)
		if err != nil {
			return 0, err
		}

		remaining -= int64(requestedSatoshis)
		counter++
	}

	err = tx.PayTo(b.fromKeySet.Script, uint64(remaining)-b.getFeeSat(tx.Size()))
	//err := tx.Change(b.fromKeySet.Script, b.feeQuote)
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.fromKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return 0, err
	}

	return counter, nil
}

func (b *RateBroadcaster) submitTxs(txs []*bt.Tx, expectedStatus metamorph_api.Status) error {

	resp, err := b.client.BroadcastTransactions(context.Background(), txs, expectedStatus, b.CallbackURL)
	if err != nil {
		return err
	}

	for _, res := range resp {
		if res.Status != expectedStatus {
			return fmt.Errorf("transaction does not have expected status %s, but %s", expectedStatus.String(), res.Status.String())
		}
	}

	return nil
}

func (b *RateBroadcaster) CreateUtxos(requestedOutputs int, requestedSatoshisPerOutput uint64) error {

	requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

	balance, err := b.utxoClient.GetBalance(!b.isTestnet, b.fromKeySet.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	if requestedOutputsSatoshis > balance {
		return fmt.Errorf("requested total of satoshis %d exceeds balance on funding keyset %d", requestedOutputsSatoshis, balance)
	}

	utxoSet := make([]*bt.UTXO, 0, requestedOutputs)

requestedOutputsLoop:
	for len(utxoSet) < requestedOutputs {

		utxoSet = make([]*bt.UTXO, 0, requestedOutputs)
		greater := make([]*bt.UTXO, 0)
		smaller := make([]*bt.UTXO, 0)

		utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.fromKeySet.Script, b.fromKeySet.Address(!b.isTestnet))
		if err != nil {
			return err
		}

		for _, utxo := range utxos {
			if len(utxoSet) >= requestedOutputs {
				break requestedOutputsLoop
			}

			if utxo.Satoshis == requestedSatoshisPerOutput {
				utxoSet = append(utxoSet, utxo)
				continue
			}

			if utxo.Satoshis > requestedSatoshisPerOutput {
				greater = append(greater, utxo)
			}

			if utxo.Satoshis < requestedSatoshisPerOutput {
				smaller = append(smaller, utxo)
			}
		}

		txsBatches := make([][]*bt.Tx, 0)
		txs := make([]*bt.Tx, 0)
		outputs := len(utxoSet)

		// prepare txs for splitting down utxos for next iteration
		for _, utxo := range greater {
			if outputs >= requestedOutputs {
				break
			}

			tx := bt.NewTx()
			err = tx.FromUTXOs(utxo)
			if err != nil {
				return err
			}

			addedOutputs, err := b.addOutputsSplit(tx, utxo.Satoshis, requestedSatoshisPerOutput, requestedOutputs-outputs)
			if err != nil {
				return err
			}

			outputs += addedOutputs

			txs = append(txs, tx)

			if len(txs) == b.batchSize {
				txsBatches = append(txsBatches, txs)
				txs = make([]*bt.Tx, 0)
			}
		}

		// prepare txs for consolidating utxos for next iteration
		txSatoshis := uint64(0)
		for _, utxo := range smaller {
			if outputs >= requestedOutputs {
				break
			}

			tx := bt.NewTx()
			err = tx.FromUTXOs(utxo)
			if err != nil {
				return err
			}

			txSatoshis += utxo.Satoshis

			if len(tx.Inputs) >= b.maxInputs || txSatoshis > requestedSatoshisPerOutput {

				addedOutputs, err := b.addOutputsConsolidate(tx, txSatoshis, requestedSatoshisPerOutput)
				if err != nil {
					return err
				}

				outputs += addedOutputs

				txs = append(txs, tx)

				tx = bt.NewTx()
				txSatoshis = 0
			}

			if len(txs) == b.batchSize {
				txsBatches = append(txsBatches, txs)
				txs = make([]*bt.Tx, 0)
			}
		}

		if len(txs) > 0 {
			txsBatches = append(txsBatches, txs)
		}

		for _, batch := range txsBatches {
			err = b.submitTxs(batch, metamorph_api.Status_SEEN_ON_NETWORK)
			if err != nil {
				return err
			}

			time.Sleep(1 * time.Second)
		}

		b.logger.Info("utxo set", slog.Int("ready", outputs), slog.Int("requested", requestedOutputs))
	}

	b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs))

	return nil
}

func (b *RateBroadcaster) Broadcast(requestedOutputs int, requestedSatoshisPerOutput uint64, rate int) error {

	requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

	balance, err := b.utxoClient.GetBalance(!b.isTestnet, b.fromKeySet.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	if requestedOutputsSatoshis > balance {
		return fmt.Errorf("requested total of satoshis %d exceeds balance on funding keyset %d", requestedOutputsSatoshis, balance)
	}

	utxoSet := make([]*bt.UTXO, 0, requestedOutputs)

	utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.fromKeySet.Script, b.fromKeySet.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	for _, utxo := range utxos {

		if len(utxoSet) >= requestedOutputs {
			break
		}

		if utxo.Satoshis == requestedSatoshisPerOutput {
			utxoSet = append(utxoSet, utxo)
			continue
		}

		b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs))
	}

	return nil
}
