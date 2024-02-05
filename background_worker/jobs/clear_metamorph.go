package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
)

type Metamorph struct {
	client        metamorph_api.MetaMorphAPIClient
	logger        *slog.Logger
	retentionDays int32
}

func NewMetamorph(client metamorph_api.MetaMorphAPIClient, retentionDays int32, logger *slog.Logger) *Metamorph {
	return &Metamorph{
		client:        client,
		logger:        logger,
		retentionDays: retentionDays,
	}
}

func (c Metamorph) ClearTransactions(_ string) error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearData(ctx, &metamorph_api.ClearDataRequest{RetentionDays: c.retentionDays})
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in metamorph", slog.Int64("rows", resp.RecordsAffected), slog.Duration("duration", time.Since(start)))

	return nil
}
