package tests

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func UpdateMined(t *testing.T, s store.MetamorphStore) {
	err := s.Set(context.Background(), Tx1Hash[:], &store.StoreData{
		Hash:   Tx1Hash,
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})
	require.NoError(t, err)

	var data *store.StoreData
	data, err = s.Get(context.Background(), Tx1Hash[:])
	require.NoError(t, err)

	assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
	assert.Nil(t, data.BlockHash)
	assert.Equal(t, uint64(0), data.BlockHeight)

	txBlocks := &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{{
		BlockHash:       Block1Hash[:],
		BlockHeight:     123,
		TransactionHash: Tx1Hash[:],
		MerklePath:      "merkle-path-1",
	}}}

	updated, err := s.UpdateMined(context.Background(), txBlocks)
	require.NoError(t, err)

	require.Equal(t, "merkle-path-1", updated[0].MerklePath)
	require.Equal(t, uint64(123), updated[0].BlockHeight)
	require.True(t, Block1Hash.IsEqual(updated[0].BlockHash))
	require.True(t, Tx1Hash.IsEqual(updated[0].Hash))

	data, err = s.Get(context.Background(), Tx1Hash[:])
	require.NoError(t, err)
	assert.Equal(t, metamorph_api.Status_MINED, data.Status)
	assert.Equal(t, Block1Hash, data.BlockHash)
	assert.Equal(t, uint64(123), data.BlockHeight)
}
