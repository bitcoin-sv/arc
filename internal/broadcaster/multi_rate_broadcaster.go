package broadcaster

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type RateBroadcaster interface {
	Start() error
	Wait()
	Shutdown()
	GetLimit() int64
	GetTxCount() int64
	GetConnectionCount() int64
	GetUtxoSetLen() int
}

type MultiKeyRateBroadcaster struct {
	rbs         []RateBroadcaster
	logger      *slog.Logger
	target      int64
	cancelAll   context.CancelFunc
	ctx         context.Context
	wg          sync.WaitGroup
	logInterval time.Duration
}

func WithLogInterval(d time.Duration) func(*MultiKeyRateBroadcaster) {
	return func(p *MultiKeyRateBroadcaster) {
		p.logInterval = d
	}
}

func NewMultiKeyRateBroadcaster(logger *slog.Logger, rbs []RateBroadcaster, opts ...func(client *MultiKeyRateBroadcaster)) *MultiKeyRateBroadcaster {
	mrb := &MultiKeyRateBroadcaster{
		rbs:         rbs,
		logger:      logger,
		logInterval: 2 * time.Second,
		target:      0,
	}

	for _, opt := range opts {
		opt(mrb)
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	mrb.cancelAll = cancelAll
	mrb.ctx = ctx

	return mrb
}

func (mrb *MultiKeyRateBroadcaster) Start() error {
	mrb.logStats()
	for _, rb := range mrb.rbs {
		err := rb.Start()

		atomic.AddInt64(&mrb.target, rb.GetLimit())
		if err != nil {
			return err
		}
	}

	for _, rb := range mrb.rbs {
		rb.Wait()
	}

	return nil
}

func (mrb *MultiKeyRateBroadcaster) Len() int {
	return len(mrb.rbs)
}

func (mrb *MultiKeyRateBroadcaster) Shutdown() {
	for _, rb := range mrb.rbs {
		rb.Shutdown()
	}

	mrb.cancelAll()
	mrb.wg.Wait()
}

func (mrb *MultiKeyRateBroadcaster) logStats() {
	mrb.wg.Add(1)

	logStatsTicker := time.NewTicker(mrb.logInterval)

	go func() {
		defer mrb.wg.Done()
		for {
			select {
			case <-logStatsTicker.C:
				totalTxsCount := int64(0)
				totalConnectionCount := int64(0)
				totalUtxoSetLength := 0

				for _, rb := range mrb.rbs {
					totalTxsCount += rb.GetTxCount()
					totalConnectionCount += rb.GetConnectionCount()
					totalUtxoSetLength += rb.GetUtxoSetLen()
				}
				target := atomic.LoadInt64(&mrb.target)
				mrb.logger.Info("stats",
					slog.Int64("txs", totalTxsCount),
					slog.Int64("target", target),
					slog.Float64("percentage", roundFloat(float64(totalTxsCount)/float64(mrb.target)*100, 2)),
					slog.Int64("connections", totalConnectionCount),
					slog.Int("utxos", totalUtxoSetLength),
				)
			case <-mrb.ctx.Done():
				return
			}
		}
	}()
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
