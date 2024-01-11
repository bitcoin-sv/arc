package tests

import (
	"context"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NoUnmined(t *testing.T, s store.MetamorphStore) {
	var err error

	hashes := []chainhash.Hash{
		chainhash.DoubleHashH([]byte("hello world")),
		chainhash.DoubleHashH([]byte("hello again")),
		chainhash.DoubleHashH([]byte("hello again again")),
	}

	for _, hash := range hashes {
		err = s.Set(context.Background(), hash[:], &store.StoreData{
			Hash:   &hash,
			Status: metamorph_api.Status_MINED,
		})
		require.NoError(t, err)
	}

	unseen, err := s.GetUnmined(context.Background(), time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 0)
	require.NoError(t, err)
	assert.Equal(t, 0, len(unseen))

	for _, hash := range hashes {
		err = s.Del(context.Background(), hash[:])
		require.NoError(t, err)
	}
}

func MultipleUnmined(t *testing.T, s store.MetamorphStore) {
	var err error

	hashes := []chainhash.Hash{
		chainhash.DoubleHashH([]byte("hello world")),
		chainhash.DoubleHashH([]byte("hello again")),
		chainhash.DoubleHashH([]byte("hello again again")),
	}

	for _, hash := range hashes {
		err = s.Set(context.Background(), hash[:], &store.StoreData{
			Hash:   &hash,
			Status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
		})
		require.NoError(t, err)
	}

	unseen, err := s.GetUnmined(context.Background(), time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(unseen))

	for _, hash := range hashes {
		err = s.Del(context.Background(), hash[:])
		require.NoError(t, err)
	}
}
