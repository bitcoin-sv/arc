package integrationtest

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet"
	"github.com/bitcoin-sv/arc/internal/blocktx/bcnet/blocktx_p2p"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store/postgresql"
	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

func setupSut(t *testing.T, dbInfo string) (*blocktx.Processor, *blocktx_p2p.MsgHandler, *postgresql.PostgreSQL, chan []byte, chan *blocktx_api.TransactionBlocks) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	blockProcessCh := make(chan *bcnet.BlockMessage, 10)

	publishedTxsCh := make(chan *blocktx_api.TransactionBlocks, 10)
	registerTxChannel := make(chan []byte, 10)

	store, err := postgresql.New(dbInfo, 10, 80)
	require.NoError(t, err)

	mqClient := &mocks.MessageQueueClientMock{
		PublishFunc: func(_ context.Context, _ string, data []byte) error {
			serialized := &blocktx_api.TransactionBlocks{}
			err := proto.Unmarshal(data, serialized)
			require.NoError(t, err)

			publishedTxsCh <- serialized
			return nil
		},
	}

	p2pMsgHandler := blocktx_p2p.NewMsgHandler(logger, nil, blockProcessCh)
	processor, err := blocktx.NewProcessor(
		logger,
		store,
		nil,
		blockProcessCh,
		blocktx.WithMessageQueueClient(mqClient),
		blocktx.WithRegisterTxsChan(registerTxChannel),
		blocktx.WithRegisterTxsBatchSize(1), // process transaction immediately
	)
	require.NoError(t, err)

	return processor, p2pMsgHandler, store, registerTxChannel, publishedTxsCh
}

func getPublishedTxs(publishedTxsCh chan *blocktx_api.TransactionBlocks) []*blocktx_api.TransactionBlock {
	publishedTxs := make([]*blocktx_api.TransactionBlock, 0)

	for {
		select {
		case tx := <-publishedTxsCh:
			publishedTxs = append(publishedTxs, tx.TransactionBlocks...)
		default:
			return publishedTxs
		}
	}
}

func pruneTables(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec("DELETE FROM blocktx.blocks WHERE hash IS NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
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
