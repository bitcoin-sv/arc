package broadcaster

import (
	"context"
	"log/slog"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
)

const (
	maxInputsDefault      = 100
	batchSizeDefault      = 20
	millisecondsPerSecond = 1000
)

type UtxoClient interface {
	// GetUTXOs returns a list of UTXOs for the given address.
	// `utxoLimit` is the maximum number of UTXOs to return.
	// `utxoLimit=0` means no limit. The lower the limit, the quicker the result will be returned
	GetUTXOs(ctx context.Context, address string, utxoLimit int) (sdkTx.UTXOs, error)
	// GetUTXOsWithRetries is a wrapper around GetUTXOs that implements a retry mechanism.
	GetUTXOsWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64, utxoLimit int) (sdkTx.UTXOs, error)
	GetBalance(ctx context.Context, address string) (uint64, uint64, error)
	GetBalanceWithRetries(ctx context.Context, address string, constantBackoff time.Duration, retries uint64) (uint64, uint64, error)
	TopUp(ctx context.Context, address string) error
}

type Broadcaster struct {
	logger            *slog.Logger
	client            ArcClient
	isTestnet         bool
	feeModel          feemodel.SatoshisPerKilobyte
	utxoClient        UtxoClient
	callbackURL       string
	callbackToken     string
	fullStatusUpdates bool
	cancelAll         context.CancelFunc
	ctx               context.Context
	maxInputs         int
	batchSize         int
	waitForStatus     metamorph_api.Status
	opReturn          string
	sizeJitterMax     int64
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

func WithCallback(callbackURL string, callbackToken string, addCallbackToToken bool) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.callbackURL = callbackURL
		broadcaster.callbackToken = callbackToken
		if addCallbackToToken {
			broadcaster.callbackToken += "_" + time.Now().Format("20060102150405")
		}
	}
}

func WithFullstatusUpdates(fullStatusUpdates bool) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.fullStatusUpdates = fullStatusUpdates
	}
}

func WithFees(miningFeeSatPerKb uint64) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.feeModel = feemodel.SatoshisPerKilobyte{Satoshis: miningFeeSatPerKb}
	}
}

func WithOpReturn(opReturn string) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.opReturn = opReturn
	}
}

func WithSizeJitter(sizeJitterMax int64) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.sizeJitterMax = sizeJitterMax
	}
}

func WithIsTestnet(isTestnet bool) func(broadcaster *Broadcaster) {
	return func(broadcaster *Broadcaster) {
		broadcaster.isTestnet = isTestnet
	}
}

func NewBroadcaster(logger *slog.Logger, client ArcClient, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (Broadcaster, error) {
	b := Broadcaster{
		logger:        logger,
		client:        client,
		isTestnet:     false,
		batchSize:     batchSizeDefault,
		maxInputs:     maxInputsDefault,
		feeModel:      feemodel.SatoshisPerKilobyte{Satoshis: uint64(1)},
		utxoClient:    utxoClient,
		waitForStatus: metamorph_api.Status_RECEIVED,
	}

	for _, opt := range opts {
		opt(&b)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	b.cancelAll = cancelAll
	b.ctx = ctx

	return b, nil
}
