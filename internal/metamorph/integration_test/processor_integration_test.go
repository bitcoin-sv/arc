package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	nats_mocks "github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/stretchr/testify/require"
)

func TestProcessor(t *testing.T) {
	t.Run("registered transactions in redis cache", func(t *testing.T) {
		// given
		mtmStore, err := postgresql.New(dbInfo, "txs-cache-integration-test", 10, 80)
		require.NoError(t, err)

		cacheStore := cache.NewRedisStore(context.Background(), redisClient)

		pm := &mocks.PeerManagerMock{ShutdownFunc: func() {}}
		natsMock := &nats_mocks.NatsConnectionMock{
			DrainFunc: func() error {
				return nil
			},
			PublishFunc: func(_ string, _ []byte) error {
				return nil
			},
		}
		natsQueue := nats_core.New(natsMock)
		statusMessageChannel := make(chan *metamorph.TxStatusMessage, 10)

		sut, err := metamorph.NewProcessor(mtmStore, cacheStore, pm, statusMessageChannel,
			metamorph.WithProcessStatusUpdatesInterval(200*time.Millisecond),
			metamorph.WithMessageQueueClient(natsQueue),
		)
		require.NoError(t, err)
		defer sut.Shutdown()

		sut.StartSendStatusUpdate()
		sut.StartProcessStatusUpdatesInStorage()

		tx1 := testutils.RevChainhash(t, "830b8424653d2e2eaedfd802d37696821ee5f538a0837dd27ae817a20804b4c5")
		tx2 := testutils.RevChainhash(t, "f00bf349d23b14ab23931e668312f2fe8e58024b462e3d038332581c1433e4a2")

		tx1ResponseChannel := make(chan metamorph.StatusAndError, 10)
		req := &metamorph.ProcessorRequest{
			Data: &store.Data{
				Hash:              tx1,
				Status:            metamorph_api.Status_STORED,
				FullStatusUpdates: false,
			},
			ResponseChannel: make(chan metamorph.StatusAndError),
		}

		// when
		sut.ProcessTransaction(context.Background(), req)

		// then
		tx1Bytes, err := cacheStore.MapGet(metamorph.CacheRegisteredTxsHash, tx1.String())
		require.NoError(t, err)
		require.NotNil(t, tx1Bytes)

		txCount, err := cacheStore.MapLen(metamorph.CacheRegisteredTxsHash)
		require.NoError(t, err)
		require.Equal(t, int64(1), txCount)

		for r := range tx1ResponseChannel {
			require.Equal(t, tx1.String(), r.Hash.String())
		}

		// when
		statusMessageChannel <- &metamorph.TxStatusMessage{
			Hash:   tx1,
			Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Peer:   "",
		}
		statusMessageChannel <- &metamorph.TxStatusMessage{
			Hash:   tx2, // this tx should not be processed
			Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Peer:   "",
		}
		time.Sleep(200 * time.Millisecond) // give time to process tx and save in db

		// then
		select {
		case r := <-tx1ResponseChannel:
			require.Equal(t, tx1.String(), r.Hash.String())
			require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, r.Status)
		default:
			t.Fatal("did not receive update on the status channel")
		}

		tx1Stored, err := mtmStore.Get(context.Background(), tx1[:])
		require.NoError(t, err)
		require.NotNil(t, tx1Stored)
		require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, tx1Stored.Status)

		tx2Stored, err := mtmStore.Get(context.Background(), tx2[:])
		require.ErrorIs(t, err, store.ErrNotFound)
		require.Nil(t, tx2Stored)

		txCount, err = cacheStore.MapLen(metamorph.CacheRegisteredTxsHash)
		require.NoError(t, err)
		require.Equal(t, int64(1), txCount)
	})
}
