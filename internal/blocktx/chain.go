package blocktx

import (
	"errors"

	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
)

var ErrEmptyChain = errors.New("empty chain of blocks")

type chain []*blocktx_api.Block

func (c chain) getTip() (*blocktx_api.Block, error) {
	if len(c) == 0 {
		return nil, ErrEmptyChain
	}

	return c[0], nil
}
