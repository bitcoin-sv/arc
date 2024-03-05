package broadcaster_test

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/bitcoin-sv/arc/broadcaster"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"log/slog"
	"os"
	"testing"

	"github.com/bitcoin-sv/arc/broadcaster/mocks"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/stretchr/testify/require"
)

//go:generate moq -pkg mocks -out ./mocks/arc_client_mock.go . ArcClient
//go:generate moq -pkg mocks -out ./mocks/utxo_client_mock.go . UtxoClient

func TestUTXOPreparer_Payback(t *testing.T) {
	tt := []struct {
		name         string
		statuses     []*metamorph_api.TransactionStatus
		broadcastErr error

		expectedBatchSubmissions int
		expectedErrorStr         string
	}{
		{
			name: "success",
			statuses: []*metamorph_api.TransactionStatus{
				{
					Status: metamorph_api.Status_SEEN_ON_NETWORK,
				},
				{
					Status: metamorph_api.Status_SEEN_ON_NETWORK,
				},
			},

			expectedBatchSubmissions: 2,
		},
		{
			name: "status not seen",
			statuses: []*metamorph_api.TransactionStatus{
				{
					Status: metamorph_api.Status_SEEN_ON_NETWORK,
				},
				{
					Status: metamorph_api.Status_SENT_TO_NETWORK,
				},
			},

			expectedBatchSubmissions: 1,
			expectedErrorStr:         "payback transaction does not have expected status SEEN_ON_NETWORK, but SENT_TO_NETWORK",
		},
		{
			name:         "broadcast err",
			broadcastErr: errors.New("failed to broadcast"),

			expectedBatchSubmissions: 1,
			expectedErrorStr:         "failed to broadcast",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			client := &mocks.ArcClientMock{
				BroadcastTransactionsFunc: func(ctx context.Context, txs []*bt.Tx, waitForStatus metamorph_api.Status, callbackURL string) ([]*metamorph_api.TransactionStatus, error) {
					return tc.statuses, tc.broadcastErr
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

			fromKeySet, err := keyset.NewFromExtendedKeyStr("xprv9s21ZrQH143K3uWZ5zfEG9v1JimHetdddkbnFAVKx2ELSws3T51wHoQuhfxsXTF4XGREBt7fVVbJiVpXJzrzb3dUVGsMsve5HaMGma4r6SG", "0/0")
			require.NoError(t, err)

			toKeySet, err := keyset.NewFromExtendedKeyStr("xprv9s21ZrQH143K3uWZ5zfEG9v1JimHetdddkbnFAVKx2ELSws3T51wHoQuhfxsXTF4XGREBt7fVVbJiVpXJzrzb3dUVGsMsve5HaMGma4r6SG", "0/1")
			require.NoError(t, err)

			utxoClient := &mocks.UtxoClientMock{
				GetUTXOsFunc: func(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error) {
					lockingScriptHex, err := hex.DecodeString("76a914522cf9e7626d9bd8729e5a1398ece40dad1b6a2f88ac")
					require.NoError(t, err)

					return []*bt.UTXO{
						{
							TxID:          testdata.TX1Hash[:],
							Vout:          0,
							LockingScript: bscript.NewFromBytes(lockingScriptHex),
							Satoshis:      10,
						},
						{
							TxID:          testdata.TX2Hash[:],
							Vout:          0,
							LockingScript: bscript.NewFromBytes(lockingScriptHex),
							Satoshis:      10,
						},
						{
							TxID:          testdata.TX3Hash[:],
							Vout:          0,
							LockingScript: bscript.NewFromBytes(lockingScriptHex),
							Satoshis:      10,
						},
						{
							TxID:          testdata.TX4Hash[:],
							Vout:          0,
							LockingScript: bscript.NewFromBytes(lockingScriptHex),
							Satoshis:      10,
						},
						{
							TxID:          testdata.TX5Hash[:],
							Vout:          0,
							LockingScript: bscript.NewFromBytes(lockingScriptHex),
							Satoshis:      10,
						},
					}, nil
				},
			}

			utxoPreparer := broadcaster.NewRateBroadcaster(logger, client, fromKeySet, toKeySet, utxoClient,
				broadcaster.WithFees(10),
				broadcaster.WithBatchSize(2),
				broadcaster.WithMaxInputs(2),
			)

			err = utxoPreparer.Payback()
			require.Equal(t, tc.expectedBatchSubmissions, len(client.BroadcastTransactionsCalls()))
			require.Equal(t, 1, len(utxoClient.GetUTXOsCalls()))

			if tc.expectedErrorStr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tc.expectedErrorStr)
		})
	}
}
