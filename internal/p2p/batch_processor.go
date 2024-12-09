package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libsv/go-p2p/wire"
)

// batchProcessor for chain hashes runs a specified function on a batch of chain hashes in specified intervals or if the batch has reached a specified size
type batchProcessor struct {
	fn          func([]*wire.InvVect)
	batchSize   int
	runInterval time.Duration
	batch       []*wire.InvVect
	hashChannel chan *wire.InvVect
	wg          *sync.WaitGroup
	cancelAll   context.CancelFunc
	ctx         context.Context
}

func newBatchProcessor(batchSize int, runInterval time.Duration, fn func([]*wire.InvVect), bufferSize uint) *batchProcessor {
	b := &batchProcessor{
		fn:          fn,
		batchSize:   batchSize,
		runInterval: runInterval,
		hashChannel: make(chan *wire.InvVect, bufferSize),
		wg:          &sync.WaitGroup{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	b.cancelAll = cancel

	b.start()

	return b
}

func (b *batchProcessor) Put(item *wire.InvVect) {
	b.hashChannel <- item
}

func (b *batchProcessor) Shutdown() {
	if b.cancelAll != nil {
		b.cancelAll()
		b.wg.Wait()
	}
}

func (b *batchProcessor) runFunction() {
	copyBatch := make([]*wire.InvVect, len(b.batch))

	copy(copyBatch, b.batch)

	b.batch = b.batch[:0] // Clear the batch slice without reallocating the underlying memory

	go b.fn(copyBatch)
}

func (b *batchProcessor) start() {
	runTicker := time.NewTicker(b.runInterval)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-b.ctx.Done():
				runTicker.Stop()
				return
			case item := <-b.hashChannel:
				b.batch = append(b.batch, item)

				if len(b.batch) >= b.batchSize {
					b.runFunction()
					runTicker.Reset(b.runInterval)
				}

			case <-runTicker.C:
				if len(b.batch) > 0 {
					b.runFunction()
					runTicker.Reset(b.runInterval)
				}
			}
		}
	}()
}
