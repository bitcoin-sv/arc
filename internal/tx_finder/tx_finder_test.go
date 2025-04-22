package txfinder_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"testing"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	metamorphMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	txfinder "github.com/bitcoin-sv/arc/internal/tx_finder"
	"github.com/bitcoin-sv/arc/internal/tx_finder/mocks"
	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/pkg/woc_client"
)

func Test_GetRawTxs(t *testing.T) {
	txBytes1, err := hex.DecodeString("0100000001b806e345ab7e9ed8449a962c9dbe50717681f0e25bb3aa6107f9eb656eebfc8d000000006a47304402204b44a87d294927d73a557a3c2421dbe0d3b7e740b3257a6bd591ac27c7f2373302201e9aa63baadb0e122069ba08c0c1c0a525f26209264953428f91c86df9f8603941210300767c46048a2ec5b44aa3ac5c8334e531db905486c1db6dad6dbc4072ecf1feffffffff018f860100000000001976a91454e065f828face03fdbe891cd5b353a3eeb6181488ac00000000")
	require.NoError(t, err)
	txBytes2, err := hex.DecodeString("0100000001f8c15fec8548e74d94f8d93347b570dfa24677e90b04b42e190d3e3257609787000000006a47304402202abafd126a779d4c7fde41769a8abaa1a7dd7e7992b66f092921073f320602410220127e1617080931393c6587d16b10465fd5e82fbde84e3ee051561b922c65ba8041210300767c46048a2ec5b44aa3ac5c8334e531db905486c1db6dad6dbc4072ecf1feffffffff018f860100000000001976a91454e065f828face03fdbe891cd5b353a3eeb6181488ac00000000")
	require.NoError(t, err)

	tx1, err := sdkTx.NewTransactionFromBytes(txBytes1)
	require.NoError(t, err)

	tcs := []struct {
		name               string
		source             validator.FindSourceFlag
		ids                []string
		wocGetRawTxsResp   []*woc_client.WocRawTx
		wocGetRawTxsErr    error
		thResp             []*metamorph.Transaction
		getTransactionsErr error
		nodeRawTxsResp     *sdkTx.Transaction
		nodeGetRawTxErr    error

		expectedBytes [][]byte
		expectedError error
	}{
		{
			name:   "search transaction handler only - success",
			source: validator.SourceTransactionHandler,
			ids: []string{
				"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df",
				"8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
			},
			thResp: []*metamorph.Transaction{
				{TxID: "24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", Bytes: txBytes1},
				{TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8", Bytes: txBytes2},
			},

			expectedBytes: [][]byte{txBytes1, txBytes2},
		},
		{
			name:   "search transaction handler only - not found",
			source: validator.SourceTransactionHandler,
			ids:    []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			thResp: []*metamorph.Transaction{},
		},
		{
			name:               "search transaction handler only - error",
			source:             validator.SourceTransactionHandler,
			ids:                []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			thResp:             []*metamorph.Transaction{},
			getTransactionsErr: errors.New("error"),
		},
		{
			name:   "search transaction handler only - malformed hex string",
			source: validator.SourceTransactionHandler,
			ids:    []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			thResp: []*metamorph.Transaction{
				{TxID: "24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", Bytes: []byte("not valid")},
				{TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8", Bytes: []byte("not valid")},
			},
		},
		{
			name:   "search WoC only - success",
			source: validator.SourceWoC,
			ids:    []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			wocGetRawTxsResp: []*woc_client.WocRawTx{
				{
					TxID: "24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df",
					Hex:  "0100000001b806e345ab7e9ed8449a962c9dbe50717681f0e25bb3aa6107f9eb656eebfc8d000000006a47304402204b44a87d294927d73a557a3c2421dbe0d3b7e740b3257a6bd591ac27c7f2373302201e9aa63baadb0e122069ba08c0c1c0a525f26209264953428f91c86df9f8603941210300767c46048a2ec5b44aa3ac5c8334e531db905486c1db6dad6dbc4072ecf1feffffffff018f860100000000001976a91454e065f828face03fdbe891cd5b353a3eeb6181488ac00000000",
				},
				{
					TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8",
					Hex:  "0100000001f8c15fec8548e74d94f8d93347b570dfa24677e90b04b42e190d3e3257609787000000006a47304402202abafd126a779d4c7fde41769a8abaa1a7dd7e7992b66f092921073f320602410220127e1617080931393c6587d16b10465fd5e82fbde84e3ee051561b922c65ba8041210300767c46048a2ec5b44aa3ac5c8334e531db905486c1db6dad6dbc4072ecf1feffffffff018f860100000000001976a91454e065f828face03fdbe891cd5b353a3eeb6181488ac00000000",
				},
			},

			expectedBytes: [][]byte{txBytes1, txBytes2},
		},
		{
			name:             "search WoC only - not found",
			source:           validator.SourceWoC,
			ids:              []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			wocGetRawTxsResp: []*woc_client.WocRawTx{},
		},
		{
			name:             "search WoC only - error",
			source:           validator.SourceWoC,
			ids:              []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			wocGetRawTxsResp: []*woc_client.WocRawTx{},
			wocGetRawTxsErr:  errors.New("some error"),
		},
		{
			name:             "search WoC only - woc response error",
			source:           validator.SourceWoC,
			ids:              []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			wocGetRawTxsResp: []*woc_client.WocRawTx{{TxID: "24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", Error: "some error"}},
		},
		{
			name:             "search WoC only - malformed hex string",
			source:           validator.SourceWoC,
			ids:              []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			wocGetRawTxsResp: []*woc_client.WocRawTx{{Hex: "not valid"}},
		},
		{
			name:   "search transaction handler & WoC - success",
			source: validator.SourceTransactionHandler | validator.SourceWoC,
			ids:    []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			thResp: []*metamorph.Transaction{
				{TxID: "24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", Bytes: txBytes1},
				{TxID: "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8", Bytes: txBytes2},
			},

			expectedBytes: [][]byte{txBytes1, txBytes2},
		},
		{
			name:           "search node - 1 found",
			source:         validator.SourceNodes,
			ids:            []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			nodeRawTxsResp: tx1,

			expectedBytes: [][]byte{txBytes1},
		},
		{
			name:            "search node - error",
			source:          validator.SourceNodes,
			ids:             []string{"24953abc1d6e08643b35af5c905f987b44f4e9b8725d318a115324ed8fe5e9df", "8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			nodeGetRawTxErr: errors.New("some error"),
		},
		{
			name:            "search node - deadline exceeded",
			source:          validator.SourceNodes,
			ids:             []string{"8dfceb6e65ebf90761aab35be2f081767150be9d2c969a44d89e7eab45e306b8"},
			nodeGetRawTxErr: context.DeadlineExceeded,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// when
			var transactionHandler metamorph.TransactionHandler
			if tc.source.Has(validator.SourceTransactionHandler) {
				transactionHandler = &metamorphMocks.TransactionHandlerMock{
					GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
						return tc.thResp, tc.getTransactionsErr
					},
				}
			}

			var wocClient txfinder.WocClient
			if tc.source.Has(validator.SourceWoC) {
				wocClient = &mocks.WocClientMock{
					GetRawTxsFunc: func(_ context.Context, _ []string) ([]*woc_client.WocRawTx, error) {
						return tc.wocGetRawTxsResp, tc.wocGetRawTxsErr
					},
				}
			}

			var nodes txfinder.NodeClient
			if tc.source.Has(validator.SourceNodes) {
				nodes = &mocks.NodeClientMock{
					GetRawTransactionFunc: func(_ context.Context, _ string) (*sdkTx.Transaction, error) {
						return tc.nodeRawTxsResp, tc.nodeGetRawTxErr
					},
				}
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			sut := txfinder.New(transactionHandler, nodes, wocClient, logger)

			// then
			txs := sut.GetRawTxs(context.TODO(), tc.source, tc.ids)

			// assert
			require.NoError(t, err)
			for i := range tc.expectedBytes {
				require.True(t, bytes.Equal(tc.expectedBytes[i], txs[i].Bytes()))
			}
		})
	}
}
