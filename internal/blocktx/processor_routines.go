package blocktx

import (
	"log/slog"

	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
)

func FillGaps(p *Processor) error {
	if !p.fillGapsEnabled {
		return nil
	}

	const (
		hoursPerDay   = 24
		blocksPerHour = 6
	)
	i := p.peerIndex.Load()
	defer p.peerIndex.Store(i + 1)
	peer := p.peers[i]

	heightRange := p.dataRetentionDays*hoursPerDay*blocksPerHour - 10
	if heightRange <= 0 {
		return nil
	}

	blockHeightGaps, err := p.store.GetBlockGaps(p.ctx, heightRange)
	if err != nil || len(blockHeightGaps) == 0 {
		return err
	}

	for i, block := range blockHeightGaps {
		p.blockGapsMap.Store(block.Hash, block.Height)

		if i >= maxRequestBlocks {
			continue
		}

		p.logger.Info("adding request for missing block to request channel",
			slog.String("hash", block.Hash.String()),
			slog.Uint64("height", block.Height),
			slog.String("peer", peer.String()),
		)

		select {
		case p.blockRequestCh <- blocktx_p2p.BlockRequest{
			Hash: block.Hash,
			Peer: peer,
		}:
		default:

		}
	}

	return nil
}

func UnorphanRecentWrongOrphans(p *Processor) error {
	if !p.unorphanRecentWrongOrphansEnabled {
		return nil
	}

	rows, err := p.store.UnorphanRecentWrongOrphans(p.ctx)
	if err != nil {
		p.logger.Error("failed to unorphan recent wrong orphans", slog.String("err", err.Error()))
		return err
	}
	for _, b := range rows {
		p.logger.Info("Successfully unorphaned block", slog.Uint64("height", b.Height), slog.String("hash", string(b.Hash)))
	}

	return nil
}
