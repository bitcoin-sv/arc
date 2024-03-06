package broadcaster

import (
	"context"
	"encoding/json"
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
	maxInputsDefault      = 100
	batchSizeDefault      = 20
	isTestnetDefault      = true
	millisecondsPerSecond = 1000
)

type UtxoClient interface {
	GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
	GetBalance(mainnet bool, address string) (int64, error)
}

type RateBroadcaster struct {
	logger            *slog.Logger
	client            ArcClient
	fundingKeyset     *keyset.KeySet
	receivingKeyset   *keyset.KeySet
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
		logger:          logger,
		client:          client,
		fundingKeyset:   fromKeySet,
		receivingKeyset: toKeyset,
		isTestnet:       isTestnetDefault,
		feeQuote:        bt.NewFeeQuote(),
		utxoClient:      utxoClient,
		batchSize:       batchSizeDefault,
		maxInputs:       maxInputsDefault,
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
	utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.receivingKeyset.Script, b.receivingKeyset.Address(!b.isTestnet))
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

			err = b.payToFundingKeySet(tx, txSatoshis, b.receivingKeyset)
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

		err = b.payToFundingKeySet(tx, txSatoshis, b.receivingKeyset)
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

func (b *RateBroadcaster) calculateFeeSat(tx *bt.Tx) uint64 {
	size, err := tx.EstimateSizeWithTypes()
	if err != nil {
		return 0
	}
	varIntUpper := bt.VarInt(tx.OutputCount()).UpperLimitInc()
	if varIntUpper == -1 {
		return 0
	}

	changeOutputFee := varIntUpper
	changeP2pkhByteLen := uint64(8 + 1 + 25)

	totalBytes := size.TotalStdBytes + changeP2pkhByteLen

	miningFeeSat := float64(totalBytes*uint64(b.standardMiningFee.Satoshis)) / float64(b.standardMiningFee.Bytes)

	sFees := uint64(math.Ceil(miningFeeSat))
	txFees := sFees + uint64(changeOutputFee)

	return txFees
}

func (b *RateBroadcaster) payToFundingKeySet(tx *bt.Tx, totalSatoshis uint64, payingKeyset *keyset.KeySet) error {

	err := tx.PayTo(b.fundingKeyset.Script, totalSatoshis-b.calculateFeeSat(tx))
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: payingKeyset.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}

	return nil
}

func (b *RateBroadcaster) consolidateToFundingKeyset(tx *bt.Tx, totalSatoshis uint64, requestedSatoshis uint64) (addedOutputs int, err error) {

	if requestedSatoshis > totalSatoshis {
		err := b.payToFundingKeySet(tx, totalSatoshis, b.fundingKeyset)
		if err != nil {
			return 0, err
		}

		return 0, nil
	}

	err = tx.PayTo(b.fundingKeyset.Script, requestedSatoshis)
	if err != nil {
		return 0, err
	}

	err = tx.PayTo(b.fundingKeyset.Script, totalSatoshis-requestedSatoshis-b.calculateFeeSat(tx))
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return 0, err
	}

	return 1, nil
}

func (b *RateBroadcaster) splitToFundingKeyset(tx *bt.Tx, splitSatoshis uint64, requestedSatoshis uint64, requestedOutputs int) (addedOutputs int, err error) {

	if requestedSatoshis > splitSatoshis {
		return 0, fmt.Errorf("requested satoshis %d greater than satoshis to be split %d", requestedSatoshis, splitSatoshis)
	}

	counter := 0

	remaining := int64(splitSatoshis)
	for remaining > int64(requestedSatoshis) && counter < requestedOutputs {
		if uint64(remaining)-requestedSatoshis < b.calculateFeeSat(tx) {
			break
		}

		err := tx.PayTo(b.fundingKeyset.Script, requestedSatoshis)
		if err != nil {
			return 0, err
		}

		remaining -= int64(requestedSatoshis)
		counter++

	}

	err = tx.PayTo(b.fundingKeyset.Script, uint64(remaining)-b.calculateFeeSat(tx))
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
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

	balance, err := b.utxoClient.GetBalance(!b.isTestnet, b.fundingKeyset.Address(!b.isTestnet))
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

		utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
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

			addedOutputs, err := b.splitToFundingKeyset(tx, utxo.Satoshis, requestedSatoshisPerOutput, requestedOutputs-outputs)
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
		tx := bt.NewTx()
		for _, utxo := range smaller {
			if outputs >= requestedOutputs {
				break
			}

			err = tx.FromUTXOs(utxo)
			if err != nil {
				return err
			}

			txSatoshis += utxo.Satoshis

			if len(tx.Inputs) >= b.maxInputs || txSatoshis > requestedSatoshisPerOutput {

				addedOutputs, err := b.consolidateToFundingKeyset(tx, txSatoshis, requestedSatoshisPerOutput)
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

		b.logger.Info("utxo set", slog.Int("created", outputs), slog.Int("requested", requestedOutputs))

		if len(txs) > 0 {
			txsBatches = append(txsBatches, txs)
		}

		for _, batch := range txsBatches {
			err = b.submitTxs(batch, metamorph_api.Status_SEEN_ON_NETWORK)
			if err != nil {
				return err
			}

			// do not performance test ARC when creating the utxos
			time.Sleep(2 * time.Second)
		}
	}

	b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs))

	return nil
}

func (b *RateBroadcaster) Broadcast(rateTxsPerSecond int) error {

	utxoSet, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	b.logger.Info("starting broadcasting", slog.Int("batch size", b.batchSize))

	submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(b.batchSize)

	if submitBatchesPerSecond > millisecondsPerSecond {
		return fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, b.batchSize, submitBatchesPerSecond)
	}

	batchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond

	ticker := time.NewTicker(batchInterval)
	shutdown := make(chan struct{})

	batchIndex := 0

sendBatchLoop:
	for {
		select {
		case <-shutdown:
			break sendBatchLoop
		case <-ticker.C:
			if len(utxoSet) == batchIndex+b.batchSize+1 {
				utxoSet, err = b.utxoClient.GetUTXOs(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
				if err != nil {
					return err
				}

				batchIndex = 0
			}

			txs, err := b.createSelfPayingTxs(utxoSet[batchIndex : batchIndex+b.batchSize])
			if err != nil {
				return err
			}

			b.sendTxsBatchAsync(txs, shutdown)

			batchIndex += b.batchSize

		}
	}

	return nil
}

func (b *RateBroadcaster) createSelfPayingTxs(utxos []*bt.UTXO) ([]*bt.Tx, error) {
	txs := make([]*bt.Tx, len(utxos))

	for i, utxo := range utxos {
		tx := bt.NewTx()

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		err = tx.PayTo(b.fundingKeyset.Script, utxo.Satoshis-b.calculateFeeSat(tx))
		if err != nil {
			return nil, err
		}

		unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
		err = tx.FillAllInputs(context.Background(), &unlockerGetter)
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

func (b *RateBroadcaster) sendTxsBatchAsync(txs []*bt.Tx, shutdown chan struct{}) {
	go func() {

		resp, err := b.client.BroadcastTransactions(context.Background(), txs, metamorph_api.Status_SEEN_ON_NETWORK, b.CallbackURL)
		if err != nil {

			slog.Default().Error("failed to submit batch", slog.String("err", err.Error()))

			shutdown <- struct{}{}
			return
		}

		for _, res := range resp {

			resBytes, err := json.Marshal(res)
			if err != nil {

				slog.Default().Error("failed to marshal response", slog.String("err", err.Error()))

				shutdown <- struct{}{}
				return
			}

			slog.Default().Info(string(resBytes))
		}
	}()
}
