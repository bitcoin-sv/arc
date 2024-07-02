package broadcaster

import (
	"context"
	"github.com/bitcoin-sv/arc/pkg/keyset"
	"log/slog"
	"math"
	"sync"
	"time"
)

type MultiKeyRateBroadcaster struct {
	rbs       []*RateBroadcaster
	logger    *slog.Logger
	target    int64
	cancelAll context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
}

func NewMultiKeyRateBroadcaster(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, isTestnet bool, opts ...func(p *Broadcaster)) (*MultiKeyRateBroadcaster, error) {
	rbs := make([]*RateBroadcaster, 0, len(keySets))
	for _, ks := range keySets {
		rb, err := NewRateBroadcaster(logger, client, ks, utxoClient, isTestnet, opts...)
		if err != nil {
			return nil, err
		}

		rbs = append(rbs, rb)
	}

	mrb := &MultiKeyRateBroadcaster{
		rbs:    rbs,
		logger: logger,
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	mrb.cancelAll = cancelAll
	mrb.ctx = ctx

	return mrb, nil
}

func (mrb *MultiKeyRateBroadcaster) Start(rateTxsPerSecond int, limit int64) error {
	mrb.logStats()
	mrb.target = 0
	for _, rb := range mrb.rbs {
		err := rb.Start(rateTxsPerSecond, limit)
		if err != nil {
			return err
		}

		mrb.target += limit
	}

	for _, rb := range mrb.rbs {
		rb.wg.Wait()
	}

	return nil
}

func (mrb *MultiKeyRateBroadcaster) Shutdown() {
	mrb.cancelAll()
	for _, rb := range mrb.rbs {
		rb.Shutdown()
	}

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
				totalUtxoSetLength := 0

				for _, rb := range mrb.rbs {
					totalTxsCount += rb.GetTxCount()
					totalConnectionCount += rb.GetConnectionCount()
					totalUtxoSetLength += rb.GetUtxoSetLen()

				}
				mrb.logger.Info("stats",
					slog.Int64("txs", totalTxsCount),
					slog.Int64("target", mrb.target),
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
