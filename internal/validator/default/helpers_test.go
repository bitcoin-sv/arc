package defaultvalidator

import (
	"context"
	"errors"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/validator/default/testdata"
	"github.com/bitcoin-sv/arc/internal/validator/mocks"
)

func TestDefaultValidator_helpers_extendTx(t *testing.T) {
	tcs := []struct {
		name              string
		txHex             string
		foundTransactions []validator.RawTx
		expectedErr       error
	}{
		{
			name:              "cannot find parents",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: nil,
			expectedErr:       ErrParentNotFound,
		},
		{
			name:              "cannot find all parents",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx1},
			expectedErr:       ErrParentNotFound,
		},
		{
			name:              "tx finder returns rubbish",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx1, testdata.RandomTx1},
			expectedErr:       ErrParentNotFound,
		},
		{
			name:              "success",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx1, testdata.ParentTx2},
			expectedErr:       nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			txFinder := mocks.TxFinderIMock{
				GetRawTxsFunc: func(_ context.Context, _ validator.FindSourceFlag, _ []string) ([]validator.RawTx, error) {
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
	tcs := []struct {
		name                   string
		txHex                  string
		mempoolAncestors       []validator.RawTx
		getMempoolAncestorsErr error

		expectedError error
	}{
		{
			name:             "tx finder returns rubbish",
			txHex:            testdata.ValidTxRawHex,
			mempoolAncestors: []validator.RawTx{testdata.ParentTx1, testdata.RandomTx1},

			expectedError: ErrParentNotFound,
		},
		{
			name:             "with mined parents only",
			txHex:            testdata.ValidTxRawHex,
			mempoolAncestors: []validator.RawTx{testdata.ParentTx1, testdata.ParentTx2},
		},
		{
			name:                   "with mined parents only",
			txHex:                  testdata.ValidTxRawHex,
			mempoolAncestors:       nil,
			getMempoolAncestorsErr: errors.New("some error"),

			expectedError: ErrFailedToGetMempoolAncestors,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given

			txFinder := mocks.TxFinderIMock{
				GetMempoolAncestorsFunc: func(_ context.Context, _ []string) ([]validator.RawTx, error) {
					return tc.mempoolAncestors, tc.getMempoolAncestorsErr
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
			expectedUnminedAncestors := make([]validator.RawTx, 0)
			require.Len(t, actual, len(expectedUnminedAncestors))
		})
	}
}
