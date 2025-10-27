package postgresql

import (
	"context"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq" // nolint: revive // required for postgres driver

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

var clearBlocktxTableName = map[store.ClearBlocktxTable]string{
	store.TableRegisteredTransactions: "registered_transactions",
	store.TableBlockProcessing:        "block_processing",
}

func (p *PostgreSQL) ClearBlocktxTable(ctx context.Context, retentionDays int32, table store.ClearBlocktxTable) (*blocktx_api.RowsAffectedResponse, error) {
	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	tableName, ok := clearBlocktxTableName[table]
	if !ok {
		return nil, errors.Join(store.ErrInvalidTable, fmt.Errorf("invalid table: %d", table))
	}

	res, err := p.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM blocktx.%s WHERE inserted_at <= $1", tableName), deleteBeforeDate)
	if err != nil {
		return nil, errors.Join(store.ErrUnableToDeleteRows, err)
	}
	rows, _ := res.RowsAffected()
	return &blocktx_api.RowsAffectedResponse{Rows: rows}, nil
}

func (p *PostgreSQL) ClearBlocks(ctx context.Context, retentionDays int32) (*blocktx_api.RowsAffectedResponse, error) {
	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	res, err := p.db.ExecContext(ctx, `DELETE FROM blocktx.blocks WHERE timestamp <= $1`, deleteBeforeDate)
	if err != nil {
		return nil, errors.Join(store.ErrUnableToDeleteRows, err)
	}
	rows, _ := res.RowsAffected()
	return &blocktx_api.RowsAffectedResponse{Rows: rows}, nil
}
