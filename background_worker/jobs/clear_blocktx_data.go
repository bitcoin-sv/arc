package jobs

import (
	"context"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"log/slog"
	"time"
)

type Blocktx struct {
	client        blocktx_api.BlockTxAPIClient
	logger        *slog.Logger
	retentionDays int32
}

func NewBlocktx(client blocktx_api.BlockTxAPIClient, retentionDays int32, logger *slog.Logger) *Blocktx {
	return &Blocktx{
		client:        client,
		logger:        logger,
		retentionDays: retentionDays,
	}
}

func (c Blocktx) ClearTransactions() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearTransactions(ctx, &blocktx_api.ClearData{RetentionDays: c.retentionDays})
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp.Rows), slog.Duration("duration", time.Since(start)))

	return nil
}

func (c Blocktx) ClearBlocks() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearBlocks(ctx, &blocktx_api.ClearData{RetentionDays: c.retentionDays})
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp.Rows), slog.Duration("duration", time.Since(start)))

	return nil
}

func (c Blocktx) ClearBlockTransactionsMap() error {
	ctx := context.Background()
	start := time.Now()
	resp, err := c.client.ClearBlockTransactionsMap(ctx, &blocktx_api.ClearData{RetentionDays: c.retentionDays})
	if err != nil {
		return err
	}
	c.logger.Info("cleared transactions in blocktx", slog.Int64("rows", resp.Rows), slog.Duration("duration", time.Since(start)))

	return nil
}
