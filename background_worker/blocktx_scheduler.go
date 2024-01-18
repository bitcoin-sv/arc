package background_worker

import (
	"log/slog"

	"github.com/bitcoin-sv/arc/background_worker/jobs"
	"github.com/go-co-op/gocron"
)

type BlockTxScheduler struct {
	Scheduler       *gocron.Scheduler
	IntervalInHours int
	Params          jobs.ClearRecordsParams
	logger          *slog.Logger
}

func NewBlocktxScheduler(scheduler *gocron.Scheduler, intervalHours int, params jobs.ClearRecordsParams, logger *slog.Logger) *BlockTxScheduler {
	return &BlockTxScheduler{
		Scheduler:       scheduler,
		IntervalInHours: intervalHours,
		Params:          params,
		logger:          logger,
	}
}

func (b *BlockTxScheduler) RunJob(jobName string, table string, job func(params jobs.ClearRecordsParams, table string) error) {
	_, err := b.Scheduler.Every(b.IntervalInHours).Hours().Do(func() {
		err := job(b.Params, table)
		if err != nil {
			b.logger.Error("failed to run job", slog.String("name", jobName), slog.String("err", err.Error()))
			return
		}
		b.logger.Info("job complete", slog.String("name", jobName))
	})

	if err != nil {
		b.logger.Error("unable to run job", slog.String("name", jobName), slog.String("err", err.Error()))
		return
	}
}

func (b *BlockTxScheduler) Start() {
	b.Scheduler.StartAsync()
}

func (b *BlockTxScheduler) Shutdown() {
	b.Scheduler.Stop()
}
