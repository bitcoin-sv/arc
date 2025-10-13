package blocktx_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

var tx1 = []byte("tx1")
var tx2 = []byte("tx2")

func TestListenAndServe(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "start and shutdown",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &storeMocks.BlocktxStoreMock{}
			pm := &p2p.PeerManager{}

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			err = sut.ListenAndServe("localhost:7000")

			// then
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		})
	}
}

func TestAnyTransactionsMined(t *testing.T) {
	tt := []struct {
		name     string
		minedTxs []store.BlockTransaction
		isMined  bool
	}{
		{
			name: "empty transactions",
			minedTxs: []store.BlockTransaction{
				{
					TxHash: tx2,
				},
			},
			isMined: false,
		},
		{
			name: "found mined transaction",
			minedTxs: []store.BlockTransaction{
				{
					TxHash: tx1,
				},
			},
			isMined: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &storeMocks.BlocktxStoreMock{
				GetMinedTransactionsFunc: func(_ context.Context, _ [][]byte) ([]store.BlockTransaction, error) {
					return tc.minedTxs, nil
				},
			}
			pm := &p2p.PeerManager{}

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			res, err := sut.AnyTransactionsMined(context.Background(), &blocktx_api.Transactions{Transactions: []*blocktx_api.Transaction{
				{Hash: tx1},
			}})
			require.NoError(t, err)
			for _, tx := range res.Transactions {
				if bytes.Equal(tx.Hash, tx1) {
					require.Equal(t, tc.isMined, tx.Mined)
					break
				}
			}
		})
	}
}

func TestRegisterTransactions(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			proc := &mocks.ProcessorIMock{
				RegisterTransactionFunc: func(_ []byte) {},
			}

			sut, err := blocktx.NewServer(logger, nil, nil, proc, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			_, err = sut.RegisterTransactions(
				context.TODO(),
				&blocktx_api.Transactions{Transactions: []*blocktx_api.Transaction{{Hash: []byte("hash")}}},
			)

			// then
			require.NoError(t, err)
		})
	}
}

func TestRegisterTransaction(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &storeMocks.BlocktxStoreMock{}
			pm := &p2p.PeerManager{}

			proc := &mocks.ProcessorIMock{
				RegisterTransactionFunc: func(_ []byte) {},
			}

			sut, err := blocktx.NewServer(logger, storeMock, pm, proc, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			// when
			_, err = sut.RegisterTransaction(
				context.TODO(),
				&blocktx_api.Transaction{Hash: []byte("hash")},
			)

			// then
			require.NoError(t, err)
		})
	}
}

func TestCurrentBlockHeight(t *testing.T) {
	tt := []struct {
		name string
	}{
		{name: "success"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			proc := &mocks.ProcessorIMock{
				CurrentBlockHeightFunc: func() (uint64, error) {
					return 23, nil
				},
			}
			sut, err := blocktx.NewServer(logger, nil, nil, proc, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()
			resp, err := sut.CurrentBlockHeight(context.Background(), nil)
			require.NoError(t, err)
			require.Equal(t, uint64(23), resp.CurrentBlockHeight)
		})
	}
}

func TestHealth(t *testing.T) {
	tt := []struct {
		name string
	}{
		{name: "success"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			mqMock := &mqMocks.MessageQueueClientMock{
				IsConnectedFunc: func() bool {
					return true
				},
				StatusFunc: func() string {
					return "CONNECTED"
				},
			}
			sut, err := blocktx.NewServer(logger, nil, nil, nil, grpc_utils.ServerConfig{}, 0, mqMock, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			resp, err := sut.Health(context.Background(), nil)
			require.NoError(t, err)
			require.True(t, resp.Ok)
			require.Equal(t, "CONNECTED", resp.Nats)
			require.NotNil(t, resp.Timestamp)
		})
	}
}

func TestClearBlocks(t *testing.T) {
	tt := []struct {
		name string
	}{
		{name: "success"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &storeMocks.BlocktxStoreMock{
				ClearBlocktxTableFunc: func(_ context.Context, _ int32, _ string) (*blocktx_api.RowsAffectedResponse, error) {
					return &blocktx_api.RowsAffectedResponse{Rows: 42}, nil
				},
			}
			sut, err := blocktx.NewServer(logger, storeMock, nil, nil, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			resp, err := sut.ClearBlocks(context.Background(), &blocktx_api.ClearData{RetentionDays: 1})
			require.NoError(t, err)
			require.Equal(t, int64(42), resp.Rows)
		})
	}
}

func TestLatestBlocks(t *testing.T) {
	tt := []struct {
		name            string
		getBlockGapsErr error
		latestBlocksErr error

		expectedError error
	}{
		{
			name: "success",
		},
		{
			name:            "failed to get latest blocks",
			latestBlocksErr: errors.New("some error"),

			expectedError: blocktx.ErrFailedToGetLatestBlocks,
		},
		{
			name:            "failed to get block gaps",
			getBlockGapsErr: errors.New("some error"),

			expectedError: blocktx.ErrFailedToGetBlockGaps,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			storeMock := &storeMocks.BlocktxStoreMock{
				LatestBlocksFunc: func(_ context.Context, _ uint64) ([]*blocktx_api.Block, error) {
					return []*blocktx_api.Block{{Hash: []byte("block-hash-1")}, {Hash: []byte("block-hash-2")}}, tc.latestBlocksErr
				},
				GetBlockGapsFunc: func(_ context.Context, _ int) ([]*store.BlockGap, error) {
					return []*store.BlockGap{{Hash: testdata.Block1Hash}, {Hash: testdata.Block2Hash}}, tc.getBlockGapsErr
				},
			}
			sut, err := blocktx.NewServer(logger, storeMock, nil, nil, grpc_utils.ServerConfig{}, 0, nil, 20)
			require.NoError(t, err)
			defer sut.GracefulStop()

			actual, actualError := sut.LatestBlocks(context.Background(), &blocktx_api.NumOfLatestBlocks{Blocks: 10})
			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}
			require.NoError(t, actualError)

			require.Len(t, actual.Blocks, 2)
			require.Len(t, actual.BlockGaps, 2)
		})
	}
}
