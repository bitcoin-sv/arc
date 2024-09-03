package postgresql

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *PostgreSQL) UpdateBlocksStatuses(ctx context.Context, hashes []*chainhash.Hash, status blocktx_api.Status) error {
	return nil
}
