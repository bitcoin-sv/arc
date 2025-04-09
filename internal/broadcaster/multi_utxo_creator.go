package broadcaster

import (
	"log/slog"
)

type MultiKeyUTXOCreator struct {
	creators []Creator
	logger   *slog.Logger
}

type Creator interface {
	Start(outputs uint64, satoshisPerOutput uint64) error
	Wait()
	Shutdown()
}

func NewMultiKeyUTXOCreator(logger *slog.Logger, creators []Creator, opts ...func(p *MultiKeyUTXOCreator)) *MultiKeyUTXOCreator {
	mkuc := &MultiKeyUTXOCreator{
		creators: creators,
		logger:   logger,
	}

	for _, opt := range opts {
		opt(mkuc)
	}

	return mkuc
}

func (mkuc *MultiKeyUTXOCreator) Start(outputs uint64, satoshisPerOutput uint64) {
	for _, creator := range mkuc.creators {
		err := creator.Start(outputs, satoshisPerOutput)
		if err != nil {
			mkuc.logger.Error("failed to start UTXO creator", slog.String("err", err.Error()))
		}
	}

	for _, creator := range mkuc.creators {
		creator.Wait()
	}
}

func (mkuc *MultiKeyUTXOCreator) Shutdown() {
	for _, creator := range mkuc.creators {
		creator.Shutdown()
	}
}
