package broadcaster

import (
	"log/slog"
)

type MultiKeyUTXOCreator struct {
	creators []*UTXOCreator // Slice of UTXOCreator instances
	logger   *slog.Logger   // Logger instance
}

type Creator interface {
	Start(outputs int, satoshisPerOutput uint64) error
	Wait()
	Shutdown()
}

// NewMultiKeyUTXOCreator initializes the MultiKeyUTXOCreator.
func NewMultiKeyUTXOCreator(logger *slog.Logger, creators []*UTXOCreator, opts ...func(p *MultiKeyUTXOCreator)) *MultiKeyUTXOCreator {
	// Create a new instance of MultiKeyUTXOCreator
	mkuc := &MultiKeyUTXOCreator{
		creators: creators,
		logger:   logger,
	}

	// Apply optional configurations
	for _, opt := range opts {
		opt(mkuc)
	}

	return mkuc
}

func (mkuc *MultiKeyUTXOCreator) Start(outputs int, satoshisPerOutput uint64) {
	// Start each creator
	for _, creator := range mkuc.creators {
		err := creator.Start(outputs, satoshisPerOutput)
		if err != nil {
			mkuc.logger.Error("failed to start UTXO creator", slog.String("err", err.Error()))
		}
	}

	// Wait for all creators to finish
	for _, creator := range mkuc.creators {
		creator.Wait()
	}
}

func (mkuc *MultiKeyUTXOCreator) Shutdown() {
	// Loop through each UTXOCreator and call its Shutdown method
	for _, creator := range mkuc.creators {
		creator.Shutdown()
	}
}
