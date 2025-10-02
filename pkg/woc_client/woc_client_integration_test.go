//go:build woc

package woc_client

import (
	"context"
	"testing"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/require"
)

func Test_GetBalanceFromWoC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tt := []struct {
		name          string
		address       string
		responseOk    bool
		responseBody  any
		expectedError error
		expected      sdkTx.UTXOs
	}{
		{
			name:       "response OK",
			address:    testnetAddr,
			responseOk: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sut := New(false, WithURL("https://api.whatsonchain.com/v1/bsv/"))

			// when
			_, _, err := sut.GetBalance(context.TODO(), tc.address)
			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_GetUTXOsFromWoC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tt := []struct {
		name          string
		address       string
		responseOk    bool
		responseBody  any
		expectedError error
		expected      sdkTx.UTXOs
	}{
		{
			name:       "response OK",
			address:    testnetAddr,
			responseOk: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sut := New(false, WithURL("https://api.whatsonchain.com/v1/bsv/"))
			// when
			unspent, err := sut.GetUTXOs(context.TODO(), tc.address)
			// then
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			require.Len(t, unspent, 10000)

			// marshal and write unspent to a JSON file at the end of the test
			data, mErr := json.MarshalIndent(unspent, "", "  ")
			require.NoError(t, mErr)

			outPath := filepath.Join(t.TempDir(), "unspent.json")
			wErr := os.WriteFile(outPath, data, 0o644)
			require.NoError(t, wErr)
			t.Logf("wrote unspent UTXOs to %s", outPath)

		})
	}
}
