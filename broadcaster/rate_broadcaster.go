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

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *RateBroadcaster)) *RateBroadcaster {
	utxoPreparer := &RateBroadcaster{
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
		opt(utxoPreparer)
	}

	return utxoPreparer
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
	miningFee, err := b.feeQuote.Fee(bt.FeeTypeStandard)
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

			err = b.addOutput(tx, txSatoshis, uint64(miningFee.MiningFee.Satoshis))
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

		err = b.addOutput(tx, txSatoshis, uint64(miningFee.MiningFee.Satoshis))
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

func (b *RateBroadcaster) addOutput(tx *bt.Tx, totalSatoshis uint64, feePerKb uint64) error {

	fee := uint64(math.Ceil(float64(tx.Size())/1000) * float64(feePerKb))

	err := tx.PayTo(b.fromKeySet.Script, totalSatoshis-fee)
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

	miningFee, err := b.feeQuote.Fee(bt.FeeTypeStandard)
	if err != nil {
		return 0, err
	}

	feePerKb := uint64(miningFee.MiningFee.Satoshis)

	fee := uint64(math.Ceil(float64(tx.Size()) / 1000 * float64(feePerKb)))

	err = tx.PayTo(b.fromKeySet.Script, uint64(remaining)-fee)
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
		}

		txsBatches := make([][]*bt.Tx, 0)
		txs := make([]*bt.Tx, 0)
		outputs := len(utxoSet)

		for _, utxo := range greater {
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

			//fmt.Println(tx.String())

			txs = append(txs, tx)

			tx = bt.NewTx()
		}

		if len(txs) > 0 {
			txsBatches = append(txsBatches, txs)
		}

		for _, batch := range txsBatches {
			err = b.submitTxs(batch, metamorph_api.Status_SEEN_ON_NETWORK)
			if err != nil {
				return err
			}
		}

		b.logger.Info("utxo set", slog.Int("ready", outputs), slog.Int("requested", requestedOutputs))

		time.Sleep(1 * time.Second)
	}

	b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs))

	return nil
}
