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
	maxOutputsDefault = 100
	batchSizeDefault  = 20
)

type UtxoClient interface {
	GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
}

type UTXOPreparer struct {
	logger            *slog.Logger
	Client            ArcClient
	FromKeySet        *keyset.KeySet
	ToKeySet          *keyset.KeySet
	Outputs           int64
	SatoshisPerOutput uint64
	IsTestnet         bool
	CallbackURL       string
	FeeQuote          *bt.FeeQuote
	UtxoClient        UtxoClient

	maxOutputs int
	batchSize  int
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

		preparer.FeeQuote = fq
	}
}

func WithBatchSize(batchSize int) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.batchSize = batchSize
	}
}

func WithMaxOutputs(maxOutputs int) func(preparer *UTXOPreparer) {
	return func(preparer *UTXOPreparer) {
		preparer.maxOutputs = maxOutputs
	}
}

func NewUTXOPreparer(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *UTXOPreparer)) *UTXOPreparer {
	utxoPreparer := &UTXOPreparer{
		logger:     logger,
		Client:     client,
		FromKeySet: fromKeySet,
		ToKeySet:   toKeyset,
		IsTestnet:  true,
		FeeQuote:   bt.NewFeeQuote(),
		UtxoClient: utxoClient,
		batchSize:  batchSizeDefault,
		maxOutputs: maxOutputsDefault,
	}

	for _, opt := range opts {
		opt(utxoPreparer)
	}

	return utxoPreparer
}

// Payback sends all funds currently held on the receiving address back to the funding address
func (b *UTXOPreparer) Payback() error {
	utxos, err := b.UtxoClient.GetUTXOs(!b.IsTestnet, b.ToKeySet.Script, b.ToKeySet.Address(!b.IsTestnet))
	if err != nil {
		return err
	}

	tx := bt.NewTx()
	txSatoshis := uint64(0)
	batchSatoshis := uint64(0)
	miningFee, err := b.FeeQuote.Fee(bt.FeeTypeStandard)
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
		if len(tx.Inputs) >= b.maxOutputs {
			batchSatoshis += txSatoshis

			err = b.addOutputs(tx, txSatoshis, uint64(miningFee.MiningFee.Satoshis))
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

		err = b.addOutputs(tx, txSatoshis, uint64(miningFee.MiningFee.Satoshis))
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

func (b *UTXOPreparer) addOutputs(tx *bt.Tx, totalSatoshis uint64, feePerKb uint64) error {

	fee := uint64(math.Ceil(float64(tx.Size())/1000) * float64(feePerKb))

	err := tx.PayTo(b.FromKeySet.Script, totalSatoshis-fee)
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.ToKeySet.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}

	return nil
}

func (b *UTXOPreparer) submitPaybackTxs(txs []*bt.Tx) error {

	resp, err := b.Client.BroadcastTransactions(context.Background(), txs, metamorph_api.Status_SEEN_ON_NETWORK, b.CallbackURL)
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
