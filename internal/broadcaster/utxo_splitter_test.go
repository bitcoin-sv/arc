package broadcaster_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	chaincfg "github.com/bitcoin-sv/go-sdk/transaction/chaincfg"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/internal/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

func TestSplitUtxo(t *testing.T) {
	tt := []struct {
		name                     string
		broadcastTransactionsErr error
		dryRun                   bool

		expectedError                     error
		expectedBroadcastTransactionCalls int
	}{
		{
			name: "success",

			expectedBroadcastTransactionCalls: 1,
		},
		{
			name:   "success - dry run",
			dryRun: true,

			expectedBroadcastTransactionCalls: 0,
		},
		{
			name:                     "error - failed to broadcast transaction",
			broadcastTransactionsErr: errors.New("error"),

			expectedBroadcastTransactionCalls: 0,
			expectedError:                     broadcaster.ErrFailedToBroadcastTx,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			fromKs, err := keyset.New(&chaincfg.MainNet)
			require.NoError(t, err)

			toKs1, err := keyset.New(&chaincfg.MainNet)
			require.NoError(t, err)
			toKs2, err := keyset.New(&chaincfg.MainNet)
			require.NoError(t, err)
			toKs3, err := keyset.New(&chaincfg.MainNet)
			require.NoError(t, err)

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client := &mocks.ArcClientMock{
				BroadcastTransactionFunc: func(_ context.Context, tx *sdkTx.Transaction, _ metamorph_api.Status, _ string) (*metamorph_api.TransactionStatus, error) {
					require.Len(t, tx.Outputs, 3)
					require.Equal(t, uint64(500), tx.Outputs[0].Satoshis)
					require.Equal(t, uint64(500), tx.Outputs[1].Satoshis)
					require.Equal(t, uint64(499), tx.Outputs[2].Satoshis)
					require.Equal(t, toKs1.Script, tx.Outputs[0].LockingScript)
					require.Equal(t, toKs2.Script, tx.Outputs[1].LockingScript)
					require.Equal(t, toKs3.Script, tx.Outputs[2].LockingScript)

					require.Len(t, tx.Inputs, 1)

					txHash, err := chainhash.NewHashFromStr("842f1acda7a169f388765af73733dd3188e8c1cc52baa78fdac4279d53d98911")
					require.NoError(t, err)

					require.Equal(t, txHash.String(), tx.Inputs[0].SourceTXID.String())

					return &metamorph_api.TransactionStatus{
						Txid:   tx.TxID().String(),
						Status: metamorph_api.Status_SEEN_ON_NETWORK,
					}, tc.broadcastTransactionsErr
				},
			}

			toKeySets := []*keyset.KeySet{toKs1, toKs2, toKs3}

			sut, err := broadcaster.NewUTXOSplitter(logger, client, fromKs, toKeySets, false)
			require.NoError(t, err)

			// when
			err = sut.SplitUtxo("842f1acda7a169f388765af73733dd3188e8c1cc52baa78fdac4279d53d98911", 1500, 0, tc.dryRun)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tc.expectedBroadcastTransactionCalls, len(client.BroadcastTransactionCalls()))
		})
	}
}
