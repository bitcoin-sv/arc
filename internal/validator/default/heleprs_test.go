package defaultvalidator

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/validator/default/testdata"
	"github.com/bitcoin-sv/arc/internal/validator/mocks"
	"github.com/libsv/go-bt/v2"
	"github.com/stretchr/testify/require"
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
			expectedErr:       errParentNotFound,
		},
		{
			name:              "cannot find all parents",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx_1},
			expectedErr:       errParentNotFound,
		},
		{
			name:              "tx finder returns rubbish",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx_1, testdata.RandomTx_1},
			expectedErr:       errParentNotFound,
		},
		{
			name:              "success",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx_1, testdata.ParentTx_2},
			expectedErr:       nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// when
			txFinder := mocks.TxFinderIMock{
				GetRawTxsFunc: func(ctx context.Context, ids []string) ([]validator.RawTx, error) {
					return tc.foundTransactions, nil
				},
			}

			tx, _ := bt.NewTxFromString(tc.txHex)

			// then
			err := extendTx(context.TODO(), &txFinder, tx)

			// assert
			require.Equal(t, tc.expectedErr, err)

			if tc.expectedErr == nil {

				// check if really is extended
				isEF := true
				for _, input := range tx.Inputs {
					if input.PreviousTxScript == nil || (input.PreviousTxSatoshis == 0 && !input.PreviousTxScript.IsData()) {
						isEF = false
						break
					}
				}

				require.True(t, isEF, "")
			}
		})
	}
}
