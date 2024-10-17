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

	return c[len(c)-1], nil
}

func (c chain) getHashes() [][]byte {
	hashes := make([][]byte, len(c))

	for i, b := range c {
		hashes[i] = b.Hash
	}

	return hashes
}
