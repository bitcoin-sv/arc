package defaultvalidator

import (
	"context"
	"errors"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/validator"
	fixture "github.com/bitcoin-sv/arc/internal/validator/default/testdata"
	"github.com/bitcoin-sv/arc/internal/validator/mocks"
)

func TestDefaultValidator_helpers_extendTx(t *testing.T) {
	tcs := []struct {
		name              string
		txHex             string
		foundTransactions []*sdkTx.Transaction
		expectedErr       error
	}{
		{
			name:              "cannot find parents",
			txHex:             fixture.ValidTxRawHex,
			foundTransactions: nil,
			expectedErr:       ErrParentNotFound,
		},
		{
			name:              "tx finder returns rubbish",
			txHex:             fixture.ValidTxRawHex,
			foundTransactions: []*sdkTx.Transaction{fixture.ParentTx1, fixture.RandomTx1},
			expectedErr:       ErrParentNotFound,
		},
		{
			name:              "success",
			txHex:             fixture.ValidTxRawHex,
			foundTransactions: []*sdkTx.Transaction{fixture.ParentTx1},
			expectedErr:       nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			txFinder := mocks.TxFinderIMock{
				GetRawTxsFunc: func(_ context.Context, _ validator.FindSourceFlag, _ []string) ([]*sdkTx.Transaction, error) {
					return tc.foundTransactions, nil
				},
			}
			tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)

			// when
			err := extendTx(context.TODO(), &txFinder, tx, false)

			// then
			require.Equal(t, tc.expectedErr, err)

			if tc.expectedErr == nil {
				// check if really is extended
				isEF := true
				for _, input := range tx.Inputs {
					if input.SourceTxScript() == nil || (*input.SourceTxSatoshis() == uint64(0) && !input.SourceTxScript().IsData()) {
						isEF = false
						break
					}
				}
				require.True(t, isEF, "")
			}
		})
	}
}

func TestDefaultValidator_helpers_getUnminedAncestors(t *testing.T) {
	txMap := map[string]*sdkTx.Transaction{
		fixture.ParentTxID1:              fixture.ParentTx1,
		fixture.AncestorTxID1:            fixture.AncestorTx1,
		fixture.AncestorOfAncestorTx1ID1: fixture.AncestorOfAncestor1Tx1,
		fixture.RandomTxID1:              fixture.RandomTx1,
	}

	tcs := []struct {
		name                   string
		txHex                  string
		mempoolAncestors       []string
		getMempoolAncestorsErr error

		expectedError error
	}{
		{
			name:             "tx finder returns rubbish",
			txHex:            fixture.ValidTxRawHex,
			mempoolAncestors: []string{fixture.ParentTx1.TxID().String(), fixture.RandomTx1.TxID().String()},

			expectedError: ErrParentNotFound,
		},
		{
			name:             "with mined parents only",
			txHex:            fixture.ValidTxRawHex,
			mempoolAncestors: []string{fixture.ParentTx1.TxID().String()},
		},
		{
			name:                   "with mined parents only",
			txHex:                  fixture.ValidTxRawHex,
			mempoolAncestors:       nil,
			getMempoolAncestorsErr: errors.New("some error"),

			expectedError: ErrFailedToGetMempoolAncestors,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given

			txFinder := mocks.TxFinderIMock{
				GetMempoolAncestorsFunc: func(_ context.Context, _ []string) ([]string, error) {
					return tc.mempoolAncestors, tc.getMempoolAncestorsErr
				},
				GetRawTxsFunc: func(_ context.Context, _ validator.FindSourceFlag, ids []string) ([]*sdkTx.Transaction, error) {
					var rawTxs []*sdkTx.Transaction
					for _, id := range ids {
						rawTx, ok := txMap[id]
						if !ok {
							continue
						}
						rawTxs = append(rawTxs, rawTx)
					}

					return rawTxs, nil
				},
			}

			tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)

			// when
			actual, actualError := getUnminedAncestors(context.TODO(), &txFinder, tx, false)

			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, actualError, tc.expectedError)
				return
			}

			require.NoError(t, actualError)
			require.Len(t, actual, 1)
		})
	}
}
