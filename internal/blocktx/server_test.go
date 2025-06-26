package blocktx_test

import (
	"context"
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
			require.Equal(t, res.Transactions[0].Mined, tc.isMined)
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
