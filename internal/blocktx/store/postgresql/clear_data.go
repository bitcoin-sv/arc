package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq" // nolint: revive // required for postgres driver

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

var clearBlocktxTableName = map[store.ClearBlocktxTable]string{
	store.TableRegisteredTransactions: "registered_transactions",
	store.TableBlockProcessing:        "block_processing",
}

func (p *PostgreSQL) ClearBlocktxTable(ctx context.Context, retentionDays int32, table store.ClearBlocktxTable) (int64, error) {
	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	tableName, ok := clearBlocktxTableName[table]
	if !ok {
		return 0, errors.Join(store.ErrInvalidTable, fmt.Errorf("invalid table: %d", table))
	}

	res, err := p.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM blocktx.%s WHERE inserted_at <= $1", tableName), deleteBeforeDate)
	if err != nil {
		return 0, errors.Join(store.ErrUnableToDeleteRows, fmt.Errorf("table %s: %w", tableName, err))
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (p *PostgreSQL) ClearBlocks(ctx context.Context, retentionDays int32) (int64, error) {
	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	res, err := p.db.ExecContext(ctx, `DELETE FROM blocktx.blocks WHERE timestamp <= $1`, deleteBeforeDate)
	if err != nil {
		return 0, errors.Join(store.ErrUnableToDeleteRows, fmt.Errorf("table blocks: %w", err))
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}
