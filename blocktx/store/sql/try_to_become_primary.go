package sql

import (
	"context"
	"time"

	"github.com/ordishs/gocore"
)

const PrimaryDurationSecs = 2 * 60

// GetBlockTransactions returns the transaction hashes for a given block hash
func (s *SQL) TryToBecomePrimary(ctx context.Context, myHostName string) error {
	start := gocore.CurrentNanos()
	defer func() {
		gocore.NewStat("blocktx").NewStat("AmIPrimary").AddTime(start)
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	primaryBlocktx, err := s.PrimaryBlocktx(ctx)
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
