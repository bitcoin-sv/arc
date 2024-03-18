package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph"
)

type Metamorph struct {
	client        metamorph.TransactionMaintainer
	logger        *slog.Logger
	retentionDays int32
}

func NewMetamorph(client metamorph.TransactionMaintainer, retentionDays int32, logger *slog.Logger) *Metamorph {
	return &Metamorph{
		client:        client,
		logger:        logger,
		retentionDays: retentionDays,
	}
}

func (c Metamorph) ClearTransactions() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearData(ctx, c.retentionDays)
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in metamorph", slog.Int64("rows", resp), slog.String("duration", time.Since(start).String()))

	return nil
}
