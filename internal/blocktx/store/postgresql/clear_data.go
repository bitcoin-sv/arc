package postgresql

import (
	"context"
	"fmt"
	"time"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	_ "github.com/lib/pq"
)

const (
	numericalDateHourLayout = "2006010215"
)

func (p *PostgreSQL) ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.ClearDataResponse, error) {

	now := p.now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	stmt, err := p.db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE inserted_at_num <= $1::int", table))
	if err != nil {
		return nil, fmt.Errorf("unable to prepare statement: %v", err)
	}

	res, err := stmt.ExecContext(ctx, deleteBeforeDate.Format(numericalDateHourLayout))
	if err != nil {
		return nil, fmt.Errorf("unable to delete rows: %v", err)
	}
	rows, _ := res.RowsAffected()
	return &blocktx_api.ClearDataResponse{Rows: rows}, nil
}
