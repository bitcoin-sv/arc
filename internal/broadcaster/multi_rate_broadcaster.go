package broadcaster

import (
	"context"
	"log/slog"
	"math"
	"sync"
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
	GetKeyName() string
}

type MultiKeyRateBroadcaster struct {
	rbs       []RateBroadcaster
	logger    *slog.Logger
	target    int64
	cancelAll context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
}

func NewMultiKeyRateBroadcaster(logger *slog.Logger, rbs []RateBroadcaster) *MultiKeyRateBroadcaster {

	mrb := &MultiKeyRateBroadcaster{
		rbs:    rbs,
		logger: logger,
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	mrb.cancelAll = cancelAll
	mrb.ctx = ctx

	return mrb
}

func (mrb *MultiKeyRateBroadcaster) Start() error {
	mrb.logStats()
	mrb.target = 0
	for _, rb := range mrb.rbs {
		err := rb.Start()
		if err != nil {
			return err
		}
	}

	for _, rb := range mrb.rbs {
		rb.Wait()
	}

	return nil
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

	logStatsTicker := time.NewTicker(2 * time.Second)

	go func() {
		defer mrb.wg.Done()
		for {
			select {
			case <-logStatsTicker.C:
				totalTxsCount := int64(0)
				totalConnectionCount := int64(0)
				var logArgs []slog.Attr
				for _, rb := range mrb.rbs {
					totalTxsCount += rb.GetTxCount()
					totalConnectionCount += rb.GetConnectionCount()
					mrb.target += rb.GetLimit()
					logArgs = append(logArgs, slog.Int(rb.GetKeyName(), rb.GetUtxoSetLen()))
				}

				mrb.logger.Info("stats",
					slog.Int64("txs", totalTxsCount),
					slog.Int64("target", mrb.target),
					slog.Float64("percentage", roundFloat(float64(totalTxsCount)/float64(mrb.target)*100, 2)),
					slog.Int64("connections", totalConnectionCount),
					"utxos", logArgs,
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
