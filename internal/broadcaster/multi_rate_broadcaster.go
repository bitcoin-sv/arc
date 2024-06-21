package broadcaster

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/gosuri/uilive"
)

type MultiRateBroadcaster struct {
	rbs []*RateBroadcaster

	cancelAll context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
}

func NewMultiRateBroadcaster(logger *slog.Logger, client ArcClient, keySets []*keyset.KeySet, utxoClient UtxoClient, opts ...func(p *Broadcaster)) (*MultiRateBroadcaster, error) {

	rbs := make([]*RateBroadcaster, 0, len(keySets))
	for _, key := range keySets {
		b, err := NewBroadcaster(logger, client, utxoClient, opts...)
		if err != nil {
			return nil, err
		}
		rb := &RateBroadcaster{
			Broadcaster: b,
			totalTxs:    0,
			shutdown:    make(chan struct{}, 10),
			wg:          sync.WaitGroup{},
			ks:          key,
		}
		rbs = append(rbs, rb)
	}

	mrb := &MultiRateBroadcaster{
		rbs: rbs,
	}

	ctx, cancelAll := context.WithCancel(context.Background())
	mrb.cancelAll = cancelAll
	mrb.ctx = ctx

	return mrb, nil
}

func (mrb *MultiRateBroadcaster) Start(rateTxsPerSecond int, limit int64) error {
	mrb.startPrintStats()

	for _, rb := range mrb.rbs {
		err := rb.Start(rateTxsPerSecond, limit)
		if err != nil {
			return err
		}
	}

	for _, rb := range mrb.rbs {
		rb.wg.Wait()
	}

	return nil
}

func (mrb *MultiRateBroadcaster) Shutdown() {
	for _, rb := range mrb.rbs {
		rb.Shutdown()
	}

	mrb.cancelAll()

	mrb.wg.Wait()
}

func (mrb *MultiRateBroadcaster) startPrintStats() {
	mrb.wg.Add(1)
	go func() {
		defer mrb.wg.Done()
		var m runtime.MemStats
		var writer = uilive.New()
		writer.Start()
		for {
			select {
			case <-time.NewTicker(500 * time.Millisecond).C:

				for _, rb := range mrb.rbs {
					totalTxsCount := rb.GetTxCount()
					totalConnectionCount := rb.GetConnectionCount()
					totalUtxoSetLength := rb.GetUtxoSetLen()

					_, _ = fmt.Fprintf(writer, "Address: %s\tCurrent connections count: %d\tTx count: %d\tUTXO set length: %d\n", rb.ks.Address(!rb.isTestnet), totalConnectionCount, totalTxsCount, totalUtxoSetLength)
				}

				runtime.ReadMemStats(&m)
				_, _ = fmt.Fprintf(writer, "Alloc:\t\t%v MiB\n", m.Alloc/1024/1024)
				_, _ = fmt.Fprintf(writer, "TotalAlloc:\t%v MiB\n", m.TotalAlloc/1024/1024)
				_, _ = fmt.Fprintf(writer, "Sys:\t\t%v MiB\n", m.Sys/1024/1024)
				_, _ = fmt.Fprintf(writer, "NumGC:\t\t%v\n", m.NumGC)
			case <-mrb.ctx.Done():
				return
			}
		}
	}()
}
