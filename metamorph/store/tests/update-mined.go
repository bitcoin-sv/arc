package tests

import (
	"context"
	"testing"

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

	err = s.UpdateMined(context.Background(), Tx1Hash, Block1Hash, 123)
	require.NoError(t, err)

	data, err = s.Get(context.Background(), Tx1Hash[:])
	require.NoError(t, err)
	assert.Equal(t, metamorph_api.Status_MINED, data.Status)
	assert.Equal(t, Block1Hash, data.BlockHash)
	assert.Equal(t, uint64(123), data.BlockHeight)
}
