package tests

import (
	"context"
	"testing"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/ordishs/go-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NoUnseen(t *testing.T, s store.MetamorphStore) {
	var err error

	hashes := [][]byte{
		utils.Sha256d([]byte("hello world")),
		utils.Sha256d([]byte("hello again")),
		utils.Sha256d([]byte("hello again again")),
	}

	for _, hash := range hashes {
		err = s.Set(context.Background(), hash, &store.StoreData{
			Hash:   hash,
			Status: metamorph_api.Status_SEEN_ON_NETWORK,
		})
		require.NoError(t, err)
	}

	unseen := make([]*store.StoreData, 0)
	err = s.GetUnseen(context.Background(), func(s *store.StoreData) {
		unseen = append(unseen, s)
	})
	require.NoError(t, err)
	assert.Equal(t, 0, len(unseen))

	for _, hash := range hashes {
		err = s.Del(context.Background(), hash)
		require.NoError(t, err)
	}
}

func MultipleUnseen(t *testing.T, s store.MetamorphStore) {
	var err error

	hashes := [][]byte{
		utils.Sha256d([]byte("hello world")),
		utils.Sha256d([]byte("hello again")),
		utils.Sha256d([]byte("hello again again")),
	}

	for _, hash := range hashes {
		err = s.Set(context.Background(), hash, &store.StoreData{
			Hash:   hash,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		require.NoError(t, err)
	}

	unseen := make([]*store.StoreData, 0)
	err = s.GetUnseen(context.Background(), func(s *store.StoreData) {
		unseen = append(unseen, s)
	})
	require.NoError(t, err)
	assert.Equal(t, 3, len(unseen))

	for _, hash := range hashes {
		err = s.Del(context.Background(), hash)
		require.NoError(t, err)
	}
}
