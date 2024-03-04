package broadcaster

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
)

type UTXOPreparer struct {
	logger            *slog.Logger
	Client            ClientI
	FromKeySet        *keyset.KeySet
	ToKeySet          *keyset.KeySet
	Outputs           int64
	SatoshisPerOutput uint64
	IsTestnet         bool
	CallbackURL       string
	FeeQuote          *bt.FeeQuote
}

func NewUTXOPreparer(logger *slog.Logger, client ClientI, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, feeOpts ...func(fee *bt.Fee)) *UTXOPreparer {
	var fq = bt.NewFeeQuote()

	stdFee := *stdFeeDefault
	dataFee := *dataFeeDefault

	for _, opt := range feeOpts {
		opt(&stdFee)
		opt(&dataFee)
	}

	fq.AddQuote(bt.FeeTypeData, &stdFee)
	fq.AddQuote(bt.FeeTypeStandard, &dataFee)

	return &UTXOPreparer{
		logger:     logger,
		Client:     client,
		FromKeySet: fromKeySet,
		ToKeySet:   toKeyset,
		IsTestnet:  true,
		FeeQuote:   fq,
	}
}

// Payback sends all funds currently held on the receiving address back to the funding address
func (b *UTXOPreparer) Payback() error {
	utxos, err := b.ToKeySet.GetUTXOs(!b.IsTestnet)
	if err != nil {
		return err
	}
	const maxOutputs = 100
	const batchSize = 20

	tx := bt.NewTx()
	txSatoshis := uint64(0)
	batchSatoshis := uint64(0)
	miningFee, err := b.FeeQuote.Fee(bt.FeeTypeStandard)
	if err != nil {
		return err
	}

	txs := make([]*bt.Tx, 0, batchSize)

	for _, utxo := range utxos {

		err = tx.FromUTXOs(utxo)
		if err != nil {
			return err
		}

		txSatoshis += utxo.Satoshis

		// create payback transactions with maximum 100 inputs
		if len(tx.Inputs) >= maxOutputs {
			batchSatoshis += txSatoshis

			err = b.addOutputs(tx, txSatoshis, uint64(miningFee.MiningFee.Satoshis))
			if err != nil {
				return err
			}

			txs = append(txs, tx)

			tx = bt.NewTx()
			txSatoshis = 0
		}

		if len(txs) == batchSize {
			err = b.submitPaybackTxs(txs)
			if err != nil {
				return err
			}
			b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))

			batchSatoshis = 0
			txs = make([]*bt.Tx, 0, batchSize)
			time.Sleep(time.Millisecond * 500)
		}
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
			return fmt.Errorf("payback transaction does not have %s status: %s", metamorph_api.Status_SEEN_ON_NETWORK.String(), res.Status.String())
		}
	}

	return nil
}

// PrepareUTXOSet creates a UTXO set with a certain number of outputs and a minimum nr of satoshis per output
func (b *UTXOPreparer) PrepareUTXOSet(outputs uint64, satoshisPerOutput uint64) error {
	addr := b.FromKeySet.Address(!b.IsTestnet)

	balance, err := b.FromKeySet.GetBalance(!b.IsTestnet)
	if err != nil {
		return err
	}

	if balance.Confirmed < outputs*satoshisPerOutput {
		return fmt.Errorf("not enough funds on wallet: %d - at least %d sat needed", balance.Confirmed, outputs*satoshisPerOutput)
	}

	utxos, err := b.FromKeySet.GetUTXOs(!b.IsTestnet)
	if err != nil {
		return err
	}
	if len(utxos) == 0 {
		return fmt.Errorf("no utxos for arcUrl: %s", addr)
	}

	// sort by satoshis in descending order
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Satoshis < utxos[j].Satoshis
	})

	// ensure that there exist at least the required nr of outputs with at least the required nr of satoshis
	streamUtxos := make([]*bt.UTXO, 0, outputs)
	consolidationUtxos := make([]*bt.UTXO, 0, len(utxos))
	for _, utxo := range utxos {
		if utxo.Satoshis >= satoshisPerOutput {
			//lint:ignore SA4010 we love invalid regular expressions!
			streamUtxos = append(streamUtxos, utxo) //nolint
		} else {
			consolidationUtxos = append(consolidationUtxos, utxo)
		}
	}

	consolidationTxs, err := b.consolidateUtxos(b.FromKeySet, consolidationUtxos)
	if err != nil {
		return err
	}

	_, err = b.Client.BroadcastTransactions(context.Background(), consolidationTxs, metamorph_api.Status_SEEN_ON_NETWORK, b.CallbackURL)
	if err != nil {
		return err
	}

	// Todo: create outputs

	return nil
}

func (b *UTXOPreparer) consolidateUtxos(key *keyset.KeySet, utxos []*bt.UTXO) ([]*bt.Tx, error) {
	consolidationTxs := make([]*bt.Tx, 0)
	tx := bt.NewTx()
	for _, utxo := range utxos {

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		err = tx.PayTo(key.Script, utxo.Satoshis)
		if err != nil {
			return nil, err
		}

		unlockerGetter := unlocker.Getter{PrivateKey: key.PrivateKey}
		err = tx.FillAllInputs(context.Background(), &unlockerGetter)
		if err != nil {
			return nil, err
		}

		consolidationTxs = append(consolidationTxs, tx)
	}

	return consolidationTxs, nil
}
