package background_worker

import (
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
)

type Scheduler struct {
	scheduler *gocron.Scheduler
	interval  time.Duration
	logger    *slog.Logger
}

func NewScheduler(scheduler *gocron.Scheduler, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		scheduler: scheduler,
		interval:  interval,
		logger:    logger,
	}
}

func (b *Scheduler) RunJob(jobName string, job func() error) {
	_, err := b.scheduler.Every(b.interval).Do(func() {
		err := job()
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

func (b *Scheduler) Start() {
	b.scheduler.StartAsync()
}

func (b *Scheduler) Shutdown() {
	b.scheduler.Stop()
}
