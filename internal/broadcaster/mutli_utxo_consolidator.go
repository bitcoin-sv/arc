package broadcaster

import (
	"log/slog"
)

type MultiKeyUtxoConsolidator struct {
	cs     []Consolidator
	logger *slog.Logger
}

type Consolidator interface {
	Start() error
	Wait()
	Shutdown()
}

func NewMultiKeyUtxoConsolidator(logger *slog.Logger, cs []Consolidator) *MultiKeyUtxoConsolidator {
	mrb := &MultiKeyUtxoConsolidator{
		cs:     cs,
		logger: logger,
	}

	return mrb
}

func (mrb *MultiKeyUtxoConsolidator) Start() {
	for _, c := range mrb.cs {
		err := c.Start()
		if err != nil {
			mrb.logger.Error("failed to start consolidator", slog.String("err", err.Error()))
		}
	}

	for _, c := range mrb.cs {
		c.Wait()
	}
}

func (mrb *MultiKeyUtxoConsolidator) Shutdown() {
	for _, c := range mrb.cs {
		c.Shutdown()
	}
}
