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

func (p *PostgreSQL) ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.RowsAffectedResponse, error) {
	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	stmt, err := p.db.Prepare(fmt.Sprintf("DELETE FROM blocktx.%s WHERE inserted_at <= $1", table))
	if err != nil {
		return nil, errors.Join(store.ErrUnableToPrepareStatement, err)
	}

	res, err := stmt.ExecContext(ctx, deleteBeforeDate)
	if err != nil {
		return nil, errors.Join(store.ErrUnableToDeleteRows, err)
	}
	rows, _ := res.RowsAffected()
	return &blocktx_api.RowsAffectedResponse{Rows: rows}, nil
}
