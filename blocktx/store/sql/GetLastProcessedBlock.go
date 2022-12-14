package sql

import (
	pb "github.com/TAAL-GmbH/arcblocktx_api"

	"context"
)

func (s *SQL) GetLastProcessedBlock(ctx context.Context) (*pb.Block, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	q := `
		SELECT
		 b.hash
		,b.header
		,b.height
		,b.orphanedyn
		FROM blocks b
		WHERE b.processedyn = true
		ORDER BY b.height DESC
		LIMIT 1
	`

	block := &pb.Block{}

	if err := s.db.QueryRowContext(ctx, q).Scan(&block.Hash, &block.Header, &block.Height, &block.Orphaned); err != nil {
		return nil, err
	}

	return block, nil
}
