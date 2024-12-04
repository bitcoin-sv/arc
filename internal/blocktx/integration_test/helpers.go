package integrationtest

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	blockchain "github.com/bitcoin-sv/arc/internal/blocktx/blockchain_communication"
	blocktx_p2p "github.com/bitcoin-sv/arc/internal/blocktx/blockchain_communication/p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core"
	nats_mock "github.com/bitcoin-sv/arc/internal/message_queue/nats/client/nats_core/mocks"
	testutils "github.com/bitcoin-sv/arc/internal/test_utils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func setupSut(t *testing.T, dbInfo string) (*blocktx.Processor, *blocktx_p2p.MsgHandler, *postgresql.PostgreSQL, chan *blocktx_api.TransactionBlock) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	blockProcessCh := make(chan *blockchain.BlockMessage, 10)

	publishedTxsCh := make(chan *blocktx_api.TransactionBlock, 10)

	store, err := postgresql.New(dbInfo, 10, 80)
	require.NoError(t, err)

	mockNatsConn := &nats_mock.NatsConnectionMock{
		PublishFunc: func(_ string, data []byte) error {
			serialized := &blocktx_api.TransactionBlock{}
			err := proto.Unmarshal(data, serialized)
			require.NoError(t, err)

			publishedTxsCh <- serialized
			return nil
		},
	}
	mqClient := nats_core.New(mockNatsConn, nats_core.WithLogger(logger))

	p2pMsgHandler := blocktx_p2p.NewMsgHandler(logger, nil, blockProcessCh)
	processor, err := blocktx.NewProcessor(
		logger,
		store,
		nil,
		blockProcessCh,
		blocktx.WithMessageQueueClient(mqClient),
	)
	require.NoError(t, err)

	return processor, p2pMsgHandler, store, publishedTxsCh
}

func getPublishedTxs(publishedTxsCh chan *blocktx_api.TransactionBlock) []*blocktx_api.TransactionBlock {
	publishedTxs := make([]*blocktx_api.TransactionBlock, 0)

	for {
		select {
		case tx := <-publishedTxsCh:
			publishedTxs = append(publishedTxs, tx)
		default:
			return publishedTxs
		}
	}
}

func pruneTables(t *testing.T, db *sql.DB) {
	t.Helper()
	testutils.PruneTables(t, db, "blocktx.blocks")
	testutils.PruneTables(t, db, "blocktx.transactions")
	testutils.PruneTables(t, db, "blocktx.block_transactions_map")
}

func verifyBlock(t *testing.T, store *postgresql.PostgreSQL, hashStr string, height uint64, status blocktx_api.Status) {
	t.Helper()
	hash := testutils.RevChainhash(t, hashStr)
	block, err := store.GetBlock(context.Background(), hash)
	require.NoError(t, err)
	require.Equal(t, height, block.Height)
	require.Equal(t, status, block.Status)
}

func verifyTxs(t *testing.T, expectedTxs []*blocktx_api.TransactionBlock, publishedTxs []*blocktx_api.TransactionBlock) {
	t.Helper()

	strippedTxs := make([]*blocktx_api.TransactionBlock, len(publishedTxs))
	for i, tx := range publishedTxs {
		strippedTxs[i] = &blocktx_api.TransactionBlock{
			BlockHash:       tx.BlockHash,
			BlockHeight:     tx.BlockHeight,
			TransactionHash: tx.TransactionHash,
			BlockStatus:     tx.BlockStatus,
		}
	}

	require.ElementsMatch(t, expectedTxs, strippedTxs)
}
