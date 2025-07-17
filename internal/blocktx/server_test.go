package blocktx_test

import (
	"context"
	mqMocks "github.com/bitcoin-sv/arc/internal/mq/mocks"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/store"
	storeMocks "github.com/bitcoin-sv/arc/internal/blocktx/store/mocks"
	"github.com/bitcoin-sv/arc/internal/grpc_utils"
	"github.com/bitcoin-sv/arc/internal/p2p"
)

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

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, grpc_utils.ServerConfig{}, 0, nil)
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
					TxHash: []byte("tx2"),
				},
			},
			isMined: false,
		},
		{
			name: "found mined transaction",
			minedTxs: []store.BlockTransaction{
				{
					TxHash: []byte("tx1"),
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

			sut, err := blocktx.NewServer(logger, storeMock, pm, nil, grpc_utils.ServerConfig{}, 0, nil)
			require.NoError(t, err)
			defer sut.GracefulStop()

			res, err := sut.AnyTransactionsMined(context.Background(), &blocktx_api.Transactions{Transactions: []*blocktx_api.Transaction{
				{Hash: []byte("tx1")},
			}})
			require.NoError(t, err)
			require.Equal(t, tc.isMined, res.Transactions[0].Mined)
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

			sut, err := blocktx.NewServer(logger, nil, nil, proc, grpc_utils.ServerConfig{}, 0, nil)
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

			sut, err := blocktx.NewServer(logger, storeMock, pm, proc, grpc_utils.ServerConfig{}, 0, nil)
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
			sut, err := blocktx.NewServer(logger, nil, nil, proc, grpc_utils.ServerConfig{}, 0, nil)
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
			sut, err := blocktx.NewServer(logger, nil, nil, nil, grpc_utils.ServerConfig{}, 0, mqMock)
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
			sut, err := blocktx.NewServer(logger, storeMock, nil, nil, grpc_utils.ServerConfig{}, 0, nil)
			require.NoError(t, err)
			defer sut.GracefulStop()

			resp, err := sut.ClearBlocks(context.Background(), &blocktx_api.ClearData{RetentionDays: 1})
			require.NoError(t, err)
			require.Equal(t, int64(42), resp.Rows)
		})
	}
}
