package broadcaster

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/ordishs/go-utils"
)

type UTXOPreparer struct {
	logger            utils.Logger
	Client            ClientI
	FromKeySet        *keyset.KeySet
	ToKeySet          *keyset.KeySet
	Outputs           int64
	SatoshisPerOutput uint64
	IsTestnet         bool
	CallbackURL       string
	FeeQuote          *bt.FeeQuote
}

func NewUTXOPreparer(logger utils.Logger, client ClientI, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, feeOpts ...func(fee *bt.Fee)) *UTXOPreparer {
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

func (b *UTXOPreparer) Payback() error {
	utxos, err := b.ToKeySet.GetUTXOs(!b.IsTestnet)
	if err != nil {
		return err
	}
	const feePerKb = 3
	const maxOutputs = 100
	const batchSize = 20

	tx := bt.NewTx()
	txSatoshis := uint64(0)
	batchSatoshis := uint64(0)

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
			err = b.addOutputs(tx, txSatoshis, feePerKb)
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
			b.logger.Infof("paid back %d satoshis", batchSatoshis)

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
		b.logger.Infof("paid back %d satoshis", batchSatoshis)
	}

	return nil
}

func (b *UTXOPreparer) addOutputs(tx *bt.Tx, totalSatoshis uint64, feePerKb uint64) error {
	var fee uint64

	fee = uint64(math.Ceil(float64(tx.Size())/1000) * float64(feePerKb))

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

func (b *UTXOPreparer) submitPaybackTx(tx *bt.Tx) error {
	res, err := b.Client.BroadcastTransaction(context.Background(), tx, metamorph_api.Status_SEEN_ON_NETWORK, b.CallbackURL)
	if err != nil {
		return err
	}

	if res.Status != metamorph_api.Status_SEEN_ON_NETWORK {
		return fmt.Errorf("payback transaction does not have %s status: %s", metamorph_api.Status_SEEN_ON_NETWORK.String(), res.Status.String())
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

func (b *UTXOPreparer) PrepareUTXOSet(outputs uint64, satoshisPerOutput uint64) error {
	addr := b.FromKeySet.Address(!b.IsTestnet)

	balance, err := b.FromKeySet.GetBalance(!b.IsTestnet)
	if err != nil {
		return err
	}

	const (
		requiredNrOutputs = 1000
		requiredOutputSat = 1000
	)

	if balance.Confirmed < outputs*satoshisPerOutput {
		return fmt.Errorf("not enough funds on wallet: %d - at least %d sat needed", balance.Confirmed, requiredNrOutputs*requiredOutputSat)
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
	streamUtxos := make([]*bt.UTXO, 0, requiredNrOutputs)
	consolidationUtxos := make([]*bt.UTXO, 0, len(utxos))
	for _, utxo := range utxos {
		if utxo.Satoshis >= requiredOutputSat {
			streamUtxos = append(streamUtxos, utxo)
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

	// Todo: create missing outputs

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
