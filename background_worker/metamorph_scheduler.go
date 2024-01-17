package background_worker

import (
	"log/slog"

	"github.com/go-co-op/gocron"
)

type MetamorphScheduler struct {
	scheduler              *gocron.Scheduler
	logger                 *slog.Logger
	executionIntervalHours int
}

func NewMetamorphScheduler(scheduler *gocron.Scheduler, logger *slog.Logger, executionIntervalHours int) *MetamorphScheduler {
	return &MetamorphScheduler{
		scheduler:              scheduler,
		logger:                 logger,
		executionIntervalHours: executionIntervalHours,
	}
}

func (s *MetamorphScheduler) RunJob(name string, job func() error) {
	_, err := s.scheduler.Every(s.executionIntervalHours).Hours().Do(func() {

		s.logger.Info("running job", slog.String("name", name))
		err := job()
		if err != nil {
			s.logger.Error("failed to run job", slog.String("err", err.Error()))
			return
		}
	})

	if err != nil {
		s.logger.Error("failed to schedule job", slog.String("err", err.Error()))
		return
	}
}

func (s *MetamorphScheduler) Start() {
	s.scheduler.StartAsync()
}

func (s *MetamorphScheduler) Shutdown() {
	s.scheduler.Stop()
}
