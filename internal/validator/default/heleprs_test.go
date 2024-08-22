package defaultvalidator

import (
	"context"
	"testing"

	"github.com/bitcoin-sv/arc/internal/validator"
	"github.com/bitcoin-sv/arc/internal/validator/default/testdata"
	"github.com/bitcoin-sv/arc/internal/validator/mocks"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
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
			foundTransactions: []validator.RawTx{testdata.ParentTx1},
			expectedErr:       errParentNotFound,
		},
		{
			name:              "tx finder returns rubbish",
			txHex:             testdata.ValidTxRawHex,
			foundTransactions: []validator.RawTx{testdata.ParentTx1, testdata.RandomTx1},
			expectedErr:       errParentNotFound,
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
			// when
			txFinder := mocks.TxFinderIMock{
				GetRawTxsFunc: func(ctx context.Context, sf validator.FindSourceFlag, ids []string) ([]validator.RawTx, error) {
					return tc.foundTransactions, nil
				},
			}

			tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)

			// then
			err := extendTx(context.TODO(), &txFinder, tx)

			// assert
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
		name                string
		txHex               string
		foundTransactionsFn func(iteration int) []validator.RawTx
		expectedErr         error
	}{
		{
			name:  "cannot find all parents",
			txHex: testdata.ValidTxRawHex,
			foundTransactionsFn: func(i int) []validator.RawTx {
				return []validator.RawTx{testdata.ParentTx1}
			},
			expectedErr: errParentNotFound,
		},
		{
			name:  "tx finder returns rubbish",
			txHex: testdata.ValidTxRawHex,
			foundTransactionsFn: func(i int) []validator.RawTx {
				return []validator.RawTx{testdata.ParentTx1, testdata.RandomTx1}
			},
			expectedErr: errParentNotFound,
		},
		{
			name:  "with unmined parents",
			txHex: testdata.ValidTxRawHex,
			foundTransactionsFn: func(i int) []validator.RawTx {
				if i == 0 {
					p1 := testdata.ParentTx1
					p2 := testdata.ParentTx2

					return []validator.RawTx{
						{
							TxID:    p1.TxID,
							Bytes:   p1.Bytes,
							IsMined: false,
						},
						{
							TxID:    p2.TxID,
							Bytes:   p2.Bytes,
							IsMined: true,
						},
					}
				}

				if i == 1 {
					return []validator.RawTx{testdata.AncestorTx1, testdata.AncestorTx2}
				}

				panic("too many calls")
			},
		},
		{
			name:  "with mined parents only",
			txHex: testdata.ValidTxRawHex,
			foundTransactionsFn: func(i int) []validator.RawTx {
				return []validator.RawTx{testdata.ParentTx1, testdata.ParentTx2}
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// when

			var getTxsCounter int = 0
			var counterPtr *int = &getTxsCounter

			txFinder := mocks.TxFinderIMock{
				GetRawTxsFunc: func(ctx context.Context, sf validator.FindSourceFlag, ids []string) ([]validator.RawTx, error) {
					iteration := *counterPtr
					*counterPtr = iteration + 1
					return tc.foundTransactionsFn(iteration), nil
				},
			}

			tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)

			// then
			res, err := getUnminedAncestors(context.TODO(), &txFinder, tx)

			// assert
			require.Equal(t, tc.expectedErr, err)

			if tc.expectedErr == nil {
				expectedUnminedAncestors := make([]validator.RawTx, 0)

				for i := 0; i < getTxsCounter; i++ {
					for _, t := range tc.foundTransactionsFn(i) {
						if !t.IsMined {
							expectedUnminedAncestors = append(expectedUnminedAncestors, t)
						}
					}
				}

				require.Len(t, res, len(expectedUnminedAncestors))
			}

		})
	}
}
