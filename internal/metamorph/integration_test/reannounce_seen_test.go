package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
)

func TestReAnnounceSeen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testutils.RunParallel(t, true, "re-announce seen", func(t *testing.T) {
		defer pruneTables(t, dbConn)
		testutils.LoadFixtures(t, dbConn, "fixtures/reannounce_seen")

		now := time.Date(2025, 5, 8, 11, 15, 0, 0, time.UTC)

		t.Log(dbInfo)

		mtmStore, err := postgresql.New(dbInfo, "re-announce-integration-test", 10, 80, postgresql.WithNow(func() time.Time { return now }))
		require.NoError(t, err)
		defer func() { _ = mtmStore.Close(context.Background()) }()

		cacheStore := cache.NewRedisStore(context.Background(), redisClient)

		messenger := &mocks.MediatorMock{
			AskForTxAsyncFunc:   func(_ context.Context, _ *chainhash.Hash) {},
			AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {},
		}

		statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)

		sut, err := metamorph.NewProcessor(mtmStore, cacheStore, messenger, statusMessageChannel,
			metamorph.WithReBroadcastExpiration(24*time.Hour),
			metamorph.WithReAnnounceSeenLastConfirmedAgo(30*time.Minute),
		)
		require.NoError(t, err)
		defer sut.Shutdown()

		metamorph.ReAnnounceSeen(context.TODO(), sut)

		chainHash1 := testutils.RevChainhash(t, "21132d32cb5411c058bb4391f24f6a36ed9b810df851d0e36cac514fd03d6b4e")
		chainHash2 := testutils.RevChainhash(t, "4910f3dccc84bd77bccbb14b739d6512dcfc70fb8b3c61fb74d491baa01aea0a")
		chainHash3 := testutils.RevChainhash(t, "8289758c1929505f9476e71698623387fc16a20ab238a3e6ce1424bc0aae368e")

		d, err := sqlx.Open("postgres", dbInfo)
		require.NoError(t, err)

		var requestedAt1 time.Time
		expectedRequestedAt1 := now
		assert.NoError(t, d.Get(&requestedAt1, "SELECT requested_at FROM metamorph.transactions WHERE hash = $1", chainHash1[:]))
		assert.True(t, expectedRequestedAt1.Equal(requestedAt1))

		var requestedAt2 time.Time
		expectedRequestedAt2 := time.Date(2025, 5, 8, 10, 10, 0, 0, time.UTC)
		assert.NoError(t, d.Get(&requestedAt2, "SELECT requested_at FROM metamorph.transactions WHERE hash = $1", chainHash2[:]))
		assert.True(t, expectedRequestedAt2.Equal(requestedAt2))

		var requestedAt3 time.Time
		expectedRequestedAt3 := now
		assert.NoError(t, d.Get(&requestedAt3, "SELECT requested_at FROM metamorph.transactions WHERE hash = $1", chainHash3[:]))
		assert.True(t, expectedRequestedAt3.Equal(requestedAt3))
	})
}
