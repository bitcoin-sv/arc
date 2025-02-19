package integrationtest

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"

	btxMocks "github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/internal/metamorph/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/p2p"
	p2p_mocks "github.com/bitcoin-sv/arc/internal/p2p/mocks"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

func TestProcessor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("registered transactions in redis cache", func(t *testing.T) {
		// given
		mtmStore, err := postgresql.New(dbInfo, "txs-cache-integration-test", 10, 80)
		require.NoError(t, err)

		cacheStore := cache.NewRedisStore(context.Background(), redisClient)

		peer := &p2p_mocks.PeerIMock{
			WriteMsgFunc:  func(_ wire.Message) {},
			NetworkFunc:   func() wire.BitcoinNet { return wire.TestNet },
			StringFunc:    func() string { return "peer" },
			ConnectedFunc: func() bool { return true },
		}

		pm := p2p.NewPeerManager(slog.Default(), wire.TestNet)
		err = pm.AddPeer(peer)
		require.NoError(t, err)

		messenger := p2p.NewNetworkMessenger(slog.Default(), pm)
		defer messenger.Shutdown()
		mediator := bcnet.NewMediator(slog.Default(), true, messenger, nil)

		mqClient := &mocks.MessageQueueClientMock{
			PublishFunc: func(_ context.Context, _ string, _ []byte) error {
				return nil
			},
		}

		statusMessageChannel := make(chan *metamorph_p2p.TxStatusMessage, 10)

		blocktxClient := &btxMocks.ClientMock{RegisterTransactionFunc: func(_ context.Context, _ []byte) error { return nil }}

		sut, err := metamorph.NewProcessor(mtmStore, cacheStore, mediator, statusMessageChannel,
			metamorph.WithProcessStatusUpdatesInterval(200*time.Millisecond),
			metamorph.WithMessageQueueClient(mqClient),
			metamorph.WithBlocktxClient(blocktxClient),
		)
		require.NoError(t, err)
		defer sut.Shutdown()

		sut.StartSendStatusUpdate()
		sut.StartProcessStatusUpdatesInStorage()

		tx1 := testutils.RevChainhash(t, "830b8424653d2e2eaedfd802d37696821ee5f538a0837dd27ae817a20804b4c5")
		tx2 := testutils.RevChainhash(t, "f00bf349d23b14ab23931e668312f2fe8e58024b462e3d038332581c1433e4a2")
		txNotRegistered := testutils.RevChainhash(t, "acd4d7bf340e420abe925a63f0d6cf9310292106a8f396ac738a19ad5b9b3b63")

		tx1ResponseChannel := make(chan metamorph.StatusAndError, 10)
		tx2ResponseChannel := make(chan metamorph.StatusAndError, 10)

		req1 := &metamorph.ProcessorRequest{
			Data: &store.Data{
				Hash:              tx1,
				Status:            metamorph_api.Status_STORED,
				FullStatusUpdates: false,
			},
			ResponseChannel: tx1ResponseChannel,
		}
		req2 := &metamorph.ProcessorRequest{
			Data: &store.Data{
				Hash:              tx2,
				Status:            metamorph_api.Status_STORED,
				FullStatusUpdates: false,
			},
			ResponseChannel: tx2ResponseChannel,
		}

		// when
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sut.ProcessTransaction(ctx, req1)
		sut.ProcessTransaction(ctx, req2)

		// then
		cacheTxs, err := cacheStore.Get(tx1.String())
		require.NoError(t, err)
		require.NotNil(t, cacheTxs)

		cacheTxs, err = cacheStore.Get(tx2.String())
		require.NoError(t, err)
		require.NotNil(t, cacheTxs)

		cacheTxs, err = cacheStore.Get(txNotRegistered.String())
		require.Nil(t, cacheTxs)
		require.ErrorIs(t, err, cache.ErrCacheNotFound)

	consumeStatuses:
		for {
			select {
			case r1 := <-tx1ResponseChannel:
				require.Equal(t, tx1.String(), r1.Hash.String())
			case r2 := <-tx2ResponseChannel:
				require.Equal(t, tx2.String(), r2.Hash.String())
			default:
				break consumeStatuses
			}
		}

		// when
		statusMessageChannel <- &metamorph_p2p.TxStatusMessage{
			Hash:   tx1,
			Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Peer:   "",
		}
		statusMessageChannel <- &metamorph_p2p.TxStatusMessage{
			Hash:   tx2,
			Status: metamorph_api.Status_REJECTED, // this tx should be removed from the cache - final status
			Peer:   "",
		}
		statusMessageChannel <- &metamorph_p2p.TxStatusMessage{
			Hash:   txNotRegistered, // this tx should not be processed
			Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			Peer:   "",
		}
		time.Sleep(400 * time.Millisecond) // give time to process tx and save in db

		// then
		select {
		case r1 := <-tx1ResponseChannel:
			require.Equal(t, tx1.String(), r1.Hash.String())
			require.Equal(t, metamorph_api.Status_ACCEPTED_BY_NETWORK, r1.Status)
		case r2 := <-tx2ResponseChannel:
			require.Equal(t, tx2.String(), r2.Hash.String())
			require.Equal(t, metamorph_api.Status_REJECTED, r2.Status)
		default:
			t.Fatal("did not receive update on the status channel")
		}

		cacheTxs, err = cacheStore.Get(tx1.String())
		require.NoError(t, err)
		require.NotNil(t, cacheTxs)

		cacheTxs, err = cacheStore.Get(tx2.String())
		require.Nil(t, cacheTxs)
		require.ErrorIs(t, err, cache.ErrCacheNotFound)

		cacheTxs, err = cacheStore.Get(txNotRegistered.String())
		require.Nil(t, cacheTxs)
		require.ErrorIs(t, err, cache.ErrCacheNotFound)
	})
}
