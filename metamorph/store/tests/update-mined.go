package tests

import (
	"context"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func UpdateMined(t *testing.T, s store.MetamorphStore) {
	err := s.Set(context.Background(), tx1Bytes, &store.StoreData{
		Hash:   tx1Bytes,
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})
	require.NoError(t, err)

	var data *store.StoreData
	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)

	assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
	assert.Equal(t, []byte(nil), data.BlockHash)
	assert.Equal(t, int32(0), data.BlockHeight)

	err = s.UpdateMined(context.Background(), tx1Bytes, []byte("block hash"), 123)
	require.NoError(t, err)

	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)
	assert.Equal(t, metamorph_api.Status_MINED, data.Status)
	assert.Equal(t, []byte("block hash"), data.BlockHash)
	assert.Equal(t, int32(123), data.BlockHeight)
}
