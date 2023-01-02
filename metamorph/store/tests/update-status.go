package tests

import (
	"context"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func UpdateStatus(t *testing.T, s store.Store) {
	err := s.Set(context.Background(), tx1Bytes, &store.StoreData{
		Hash:   tx1Bytes,
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})
	require.NoError(t, err)

	var data *store.StoreData
	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)

	assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
	assert.Equal(t, "", data.RejectReason)
	assert.Equal(t, []byte(nil), data.BlockHash)
	assert.Equal(t, int32(0), data.BlockHeight)

	err = s.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_SENT_TO_NETWORK, "")
	require.NoError(t, err)

	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)
	assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, data.Status)
	assert.Equal(t, "", data.RejectReason)
	assert.Equal(t, []byte(nil), data.BlockHash)
	assert.Equal(t, int32(0), data.BlockHeight)
}

func UpdateStatusWithError(t *testing.T, s store.Store) {
	err := s.Set(context.Background(), tx1Bytes, &store.StoreData{
		Hash:   tx1Bytes,
		Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
	})
	require.NoError(t, err)

	var data *store.StoreData
	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)

	assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, data.Status)
	assert.Equal(t, "", data.RejectReason)

	err = s.UpdateStatus(context.Background(), tx1Bytes, metamorph_api.Status_REJECTED, "error encountered")
	require.NoError(t, err)

	data, err = s.Get(context.Background(), tx1Bytes)
	require.NoError(t, err)
	assert.Equal(t, metamorph_api.Status_REJECTED, data.Status)
	assert.Equal(t, "error encountered", data.RejectReason)
}
