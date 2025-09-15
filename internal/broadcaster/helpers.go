package broadcaster

import (
	"errors"
	"math"
	"time"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"

	"github.com/bitcoin-sv/arc/internal/varintutils"
)

func PayTo(tx *sdkTx.Transaction, s *script.Script, satoshis uint64) error {
	if !s.IsP2PKH() {
		return sdkTx.ErrInvalidScriptType
	}

	tx.AddOutput(&sdkTx.TransactionOutput{
		Satoshis:      satoshis,
		LockingScript: s,
	})
	return nil
}

func SignAllInputs(tx *sdkTx.Transaction, privKey *primitives.PrivateKey) error {
	unlockingScriptTemplate, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return err
	}

	for _, in := range tx.Inputs {
		if in.UnlockingScriptTemplate != nil {
			continue
		}

		in.UnlockingScriptTemplate = unlockingScriptTemplate
	}

	err = tx.Sign()
	if err != nil {
		return err
	}
	return nil
}

func estimateSize(tx *sdkTx.Transaction) int {
	size := 4                                                   // version
	size += varintutils.VarInt(uint64(len(tx.Inputs))).Length() // number of inputs

	inputSize := len(tx.Inputs) * (40 + 1 + 107) // txid, output index, sequence number, script length
	size += inputSize

	size += varintutils.VarInt(len(tx.Outputs)).Length() // number of outputs
	for _, out := range tx.Outputs {
		size += 8 // satoshis
		length := len(*out.LockingScript)
		size += varintutils.VarInt(length).Length() // script length
		size += length                              // script
	}
	size += 4  // lock time
	size += 34 // change

	return size
}

// ComputeFee calculates the transaction fee based on its size in bytes.
func ComputeFee(tx *sdkTx.Transaction, s feemodel.SatoshisPerKilobyte) (uint64, error) {
	txSize := estimateSize(tx)

	fee := float64(txSize) * float64(s.Satoshis) / 1000

	// the minimum fee required is 1 satoshi
	feesRequiredRounded := uint64(math.Round(fee))
	if feesRequiredRounded < 1 {
		feesRequiredRounded = 1
	}

	return feesRequiredRounded, nil
}

type RampUpTicker struct {
	ticker        *time.Ticker
	startInterval time.Duration
	endInterval   time.Duration
	steps         int64
}

var (
	ErrStepsZero                          = errors.New("steps must be greater than 0")
	ErrStartIntervalNotGreaterEndInterval = errors.New("startInterval must be greater than endInterval")
)

// NewRampUpTicker returns a dynamic ticker based on time.Ticker. The time intervals linearly decrease starting from startInterval to endInterval. After a specified number of steps the time interval is equal to endInterval.
func NewRampUpTicker(startInterval time.Duration, endInterval time.Duration, steps int64) (*RampUpTicker, error) {
	if steps < 1 {
		return nil, ErrStepsZero
	}

	if startInterval <= endInterval {
		return nil, ErrStartIntervalNotGreaterEndInterval
	}

	ticker := RampUpTicker{
		ticker:        time.NewTicker(startInterval),
		startInterval: startInterval,
		endInterval:   endInterval,
		steps:         steps,
	}

	return &ticker, nil
}

func (t *RampUpTicker) Stop() {
	t.ticker.Stop()
}

func (t *RampUpTicker) GetTickerCh() <-chan time.Time {
	timeCh := make(chan time.Time)
	step := int64(0)
	stepsReached := false
	stepNs := int64(float64(t.startInterval.Nanoseconds()-t.endInterval.Nanoseconds()) / float64(t.steps))

	go func() {
		for tick := range t.ticker.C {
			timeCh <- tick

			if step >= t.steps-1 {
				if !stepsReached {
					t.ticker.Reset(t.endInterval)
					stepsReached = true
				}

				continue
			}

			step++

			newInterval := t.startInterval - time.Duration(stepNs*step)

			t.ticker.Reset(newInterval)
		}
	}()

	return timeCh
}

type ConstantTicker struct {
	ticker *time.Ticker
}

func (t *ConstantTicker) Stop() {
	t.ticker.Stop()
}

func (t *ConstantTicker) GetTickerCh() <-chan time.Time {
	timeCh := make(chan time.Time)

	go func() {
		for tick := range t.ticker.C {
			timeCh <- tick
		}
	}()

	return timeCh
}

func NewConstantTicker(endInterval time.Duration) *ConstantTicker {
	ticker := ConstantTicker{
		ticker: time.NewTicker(endInterval),
	}

	return &ticker
}
