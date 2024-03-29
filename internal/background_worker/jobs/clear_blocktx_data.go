package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx"
)

type Blocktx struct {
	client        blocktx.BlocktxClient
	logger        *slog.Logger
	retentionDays int32
}

func NewBlocktx(client blocktx.BlocktxClient, retentionDays int32, logger *slog.Logger) *Blocktx {
	return &Blocktx{
		client:        client,
		logger:        logger,
		retentionDays: retentionDays,
	}
}

func (c Blocktx) ClearTransactions() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearTransactions(ctx, c.retentionDays)
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp), slog.String("duration", time.Since(start).String()))

	return nil
}

func (c Blocktx) ClearBlocks() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearBlocks(ctx, c.retentionDays)
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp), slog.String("duration", time.Since(start).String()))

	return nil
}

func (c Blocktx) ClearBlockTransactionsMap() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearBlockTransactionsMap(ctx, c.retentionDays)
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp), slog.String("duration", time.Since(start).String()))

	return nil
}
