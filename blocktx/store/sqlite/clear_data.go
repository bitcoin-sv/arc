package sqlite

import (
	"context"
	"fmt"
	"github.com/ordishs/gocore"
	"time"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	_ "github.com/lib/pq"
)

const (
	numericalDateHourLayout = "2006010215"
)

func (s *SqLite) ClearBlocktxTable(ctx context.Context, retentionDays int32, table string) (*blocktx_api.ClearDataResponse, error) {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("ClearBlocktxTable").AddTime(start)
	}()

	now := time.Now()
	deleteBeforeDate := now.Add(-24 * time.Hour * time.Duration(retentionDays))

	stmt, err := s.db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE inserted_at_num <= $1::int", table))
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
