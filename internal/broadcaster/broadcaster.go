package broadcaster

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/fees"
	"github.com/bitcoin-sv/go-sdk/script"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/go-sdk/transaction"
)

const (
	maxInputsDefault      = 100
	batchSizeDefault      = 20
	millisecondsPerSecond = 1000
)

type UtxoClient interface {
	GetUTXOs(ctx context.Context, lockingScript *script.Script, address string) (transaction.UTXOs, error)
	GetUTXOsWithRetries(ctx context.Context, lockingScript *script.Script, address string, constantBackoff time.Duration, retries uint64) (transaction.UTXOs, error)
	GetBalance(ctx context.Context, address string) (int64, int64, error)
	GetBalanceWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error)
	TopUp(ctx context.Context, address string) error
}

type Broadcaster struct {
	logger            *slog.Logger
	client            ArcClient
	isTestnet         bool
	feeModel          fees.FeeModel
	utxoClient        UtxoClient
	standardMiningFee fees.FeeModel
	callbackURL       string
	callbackToken     string
	fullStatusUpdates bool
	cancelAll         context.CancelFunc
	ctx               context.Context
	maxInputs         int
	batchSize         int
	waitForStatus     metamorph_api.Status
}

func WithBatchSize(batchSize int) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.batchSize = batchSize
	}
}

func WithWaitForStatus(waitForStatus metamorph_api.Status) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.waitForStatus = waitForStatus
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
		feeModel := fees.SatoshisPerKilobyte{Satoshis: uint64(miningFeeSatPerKb)}
		broadcaster.feeModel = feeModel
	}
}

func NewBroadcaster(logger *slog.Logger, client ArcClient, utxoClient UtxoClient, isTestnet bool, opts ...func(p *Broadcaster)) (Broadcaster, error) {

	b := Broadcaster{
		logger:        logger,
		client:        client,
		isTestnet:     isTestnet,
		batchSize:     batchSizeDefault,
		maxInputs:     maxInputsDefault,
		feeModel:      fees.DefaultSatoshisPerKilobyte(),
		utxoClient:    utxoClient,
		waitForStatus: metamorph_api.Status_RECEIVED,
	}

	for _, opt := range opts {
		opt(&b)
	}

	b.standardMiningFee = fees.DefaultSatoshisPerKilobyte()

	ctx, cancelAll := context.WithCancel(context.Background())
	b.cancelAll = cancelAll
	b.ctx = ctx

	return b, nil
}

func (b *Broadcaster) calculateFeeSat(tx *transaction.Transaction) uint64 {
	return CalculateFeeSat(tx, b.standardMiningFee)
}
