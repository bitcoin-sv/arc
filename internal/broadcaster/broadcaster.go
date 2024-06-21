package broadcaster

import (
	"context"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/libsv/go-bt/v2"
	"log/slog"
	"math"
)

const (
	maxInputsDefault      = 100
	batchSizeDefault      = 20
	isTestnetDefault      = true
	millisecondsPerSecond = 1000
)

type Broadcaster struct {
	logger            *slog.Logger
	client            ArcClient
	keySets           []*keyset.KeySet
	isTestnet         bool
	feeQuote          *bt.FeeQuote
	utxoClient        UtxoClient
	standardMiningFee bt.FeeUnit
	callbackURL       string
	callbackToken     string
	fullStatusUpdates bool
	cancelAll         context.CancelFunc
	ctx               context.Context
	maxInputs         int
	batchSize         int
}

func WithIsTestnet(isTestnet bool) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.isTestnet = isTestnet
	}
}

func WithBatchSize(batchSize int) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.batchSize = batchSize
	}
}

func WithMaxInputs(maxInputs int) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.maxInputs = maxInputs
	}
}

func WithCallback(callbackURL string, callbackToken string) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.callbackURL = callbackURL
		broadcaster.callbackToken = callbackToken
	}
}

func WithFullstatusUpdates(fullStatusUpdates bool) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.fullStatusUpdates = fullStatusUpdates
	}
}

func WithFees(miningFeeSatPerKb int) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		var fq = bt.NewFeeQuote()

		newStdFee := *stdFeeDefault
		newDataFee := *dataFeeDefault

		newStdFee.MiningFee.Satoshis = miningFeeSatPerKb
		newDataFee.MiningFee.Satoshis = miningFeeSatPerKb

		fq.AddQuote(bt.FeeTypeData, &newStdFee)
		fq.AddQuote(bt.FeeTypeStandard, &newDataFee)

		broadcaster.feeQuote = fq
	}
}

func NewBroadcaster(logger *slog.Logger, client ArcClient, fromKeySet []*keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*Broadcaster, error) {

	b := &Broadcaster{
		logger:     logger,
		client:     client,
		keySets:    fromKeySet,
		isTestnet:  isTestnetDefault,
		batchSize:  batchSizeDefault,
		maxInputs:  maxInputsDefault,
		feeQuote:   bt.NewFeeQuote(),
		utxoClient: utxoClient,
	}

	standardFee, err := b.feeQuote.Fee(bt.FeeTypeStandard)
	if err != nil {
		return nil, err
	}

	b.standardMiningFee = standardFee.MiningFee

	for _, opt := range opts {
		opt(b)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	b.cancelAll = cancelAll
	b.ctx = ctx

	return b, nil
}

func (b *Broadcaster) calculateFeeSat(tx *bt.Tx) uint64 {
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
