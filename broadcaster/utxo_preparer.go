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

type UTXOPreparer struct {
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

func WithFees(miningFeeSatPerKb int) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		var fq = bt.NewFeeQuote()

		newStdFee := *stdFeeDefault
		newDataFee := *dataFeeDefault

		newStdFee.RelayFee.Satoshis = miningFeeSatPerKb
		newDataFee.RelayFee.Satoshis = miningFeeSatPerKb

		fq.AddQuote(bt.FeeTypeData, &newStdFee)
		fq.AddQuote(bt.FeeTypeStandard, &newDataFee)

		preparer.feeQuote = fq
	}
}

func WithBatchSize(batchSize int) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.batchSize = batchSize
	}
}

func WithMaxInputs(maxInputs int) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.maxInputs = maxInputs
	}
}

func WithIsTestnet(isTestnet bool) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.isTestnet = isTestnet
	}
}

func WithCallbackURL(callbackURL string) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.CallbackURL = callbackURL
	}
}

func NewUTXOPreparer(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *UTXOPreparer)) *UTXOPreparer {
	utxoPreparer := &UTXOPreparer{
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
func (b *UTXOPreparer) Payback() error {
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
			err = b.submitPaybackTxs(txs)
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
		err = b.submitPaybackTxs(txs)
		if err != nil {
			return err
		}

		b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))
	}

	return nil
}

func (b *UTXOPreparer) addOutput(tx *bt.Tx, totalSatoshis uint64, feePerKb uint64) error {

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

func (b *UTXOPreparer) addOutputsConsolidate(tx *bt.Tx, totalSatoshis uint64, requestedSatoshis uint64, feePerKb uint64) error {

	if requestedSatoshis > totalSatoshis {
		err := b.addOutput(tx, totalSatoshis, feePerKb)
		if err != nil {
			return err
		}

		return nil
	}

	err := tx.PayTo(b.fromKeySet.Script, requestedSatoshis)
	if err != nil {
		return err
	}

	err = tx.ChangeToAddress(b.fromKeySet.Address(!b.isTestnet), b.feeQuote)
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

func (b *UTXOPreparer) addOutputsSplit(tx *bt.Tx, splitSatoshis uint64, requestedSatoshis uint64, feePerKb uint64) error {

	if requestedSatoshis < splitSatoshis {

		return fmt.Errorf("requested satoshis %d smaller than satoshis to be split %d", requestedSatoshis, splitSatoshis)
	}

	// split satoshis into requested size
	for remaining := splitSatoshis; remaining > 0; remaining = splitSatoshis - requestedSatoshis {
		err := tx.PayTo(b.fromKeySet.Script, requestedSatoshis)
		if err != nil {
			return err
		}
	}

	err := tx.ChangeToAddress(b.fromKeySet.Address(!b.isTestnet), b.feeQuote)

	unlockerGetter := unlocker.Getter{PrivateKey: b.toKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}

	return nil
}

func (b *UTXOPreparer) submitPaybackTxs(txs []*bt.Tx) error {

	resp, err := b.client.BroadcastTransactions(context.Background(), txs, metamorph_api.Status_SEEN_ON_NETWORK, b.CallbackURL)
	if err != nil {
		return err
	}

	for _, res := range resp {
		if res.Status != metamorph_api.Status_SEEN_ON_NETWORK {
			return fmt.Errorf("payback transaction does not have expected status %s, but %s", metamorph_api.Status_SEEN_ON_NETWORK.String(), res.Status.String())
		}
	}

	return nil
}

func (b *UTXOPreparer) CreateUtxos(requestedOutputs int, requestedSatoshisPerOutput uint64) ([]*bt.UTXO, error) {

	requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

	balance, err := b.utxoClient.GetBalance(!b.isTestnet, b.fromKeySet.Address(!b.isTestnet))
	if err != nil {
		return nil, err
	}

	if requestedOutputsSatoshis > balance {
		return nil, fmt.Errorf("requested total of satoshis %d exceeds balance on funding keyset %d", requestedOutputsSatoshis, balance)
	}

	miningFee, err := b.feeQuote.Fee(bt.FeeTypeStandard)
	utxoSet := make([]*bt.UTXO, 0, requestedOutputs)

	txsBatches := make([][]*bt.Tx, 0)
	txs := make([]*bt.Tx, 0)

	var utxos []*bt.UTXO
	var utxo *bt.UTXO

requestedOutputsLoop:
	for len(utxoSet) < requestedOutputs {

		utxoSet = make([]*bt.UTXO, 0, requestedOutputs)

		utxos, err = b.utxoClient.GetUTXOs(!b.isTestnet, b.fromKeySet.Script, b.fromKeySet.Address(!b.isTestnet))
		if err != nil {
			return nil, err
		}

		txSatoshis := uint64(0)
		for _, utxo = range utxos {

			if len(utxoSet) >= requestedOutputs {
				break requestedOutputsLoop
			}

			if utxo.Satoshis == requestedSatoshisPerOutput {
				utxoSet = append(utxos, utxo)
				continue
			}

			if utxo.Satoshis < requestedSatoshisPerOutput {
				tx := bt.NewTx()
				err = tx.FromUTXOs(utxo)
				if err != nil {
					return nil, err
				}

				txSatoshis += utxo.Satoshis

				// create payback transactions with a maximum of inputs
				if len(tx.Inputs) >= b.maxInputs || txSatoshis > requestedSatoshisPerOutput {

					err = b.addOutputsConsolidate(tx, txSatoshis, requestedSatoshisPerOutput, uint64(miningFee.MiningFee.Satoshis))
					if err != nil {
						return nil, err
					}

					txs = append(txs, tx)

					tx = bt.NewTx()
					txSatoshis = 0
				}

				if len(txs) == b.batchSize {
					txsBatches = append(txsBatches, txs)
					txs = make([]*bt.Tx, 0)
				}
			}

			if utxo.Satoshis > requestedSatoshisPerOutput {
				tx := bt.NewTx()
				err = tx.FromUTXOs(utxo)
				if err != nil {
					return nil, err
				}

				err = b.addOutputsSplit(tx, utxo.Satoshis, requestedSatoshisPerOutput, uint64(miningFee.MiningFee.Satoshis))
				if err != nil {
					return nil, err
				}

				txs = append(txs, tx)

				tx = bt.NewTx()
			}
		}

		if len(txs) > 0 {
			txsBatches = append(txsBatches, txs)
		}

		for _, batch := range txsBatches {
			err = b.submitPaybackTxs(batch)
			if err != nil {
				return nil, err
			}
		}

		time.Sleep(5 * time.Second)
	}
	return utxoSet, nil
}
