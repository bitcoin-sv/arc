package postgresql

import (
	"context"
	"time"

	"github.com/ordishs/gocore"
)

const PrimaryDurationSecs = 2 * 60

func (s *PostgreSQL) TryToBecomePrimary(ctx context.Context, myHostName string) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("TryToBecomePrimary").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	primaryBlocktx, err := s.GetPrimary(ctx)
	if err != nil {
		return err
	}

	if primaryBlocktx == "" {
		s.db.QueryRowContext(ctx, `INSERT INTO primary_blocktx (host_name, primary_until) VALUES($1, $2)`, myHostName, time.Now().Local().Add(time.Second*time.Duration(PrimaryDurationSecs)))
	} else {
		s.db.QueryRowContext(ctx, `UPDATE primary_blocktx SET host_name=$1, primary_until=$2 WHERE primary_until<NOW()`, myHostName, time.Now().Local().Add(time.Second*time.Duration(PrimaryDurationSecs)))
	}

	return nil
}
