package fees

import (
	"testing"

	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
)

var validLockingScript = &script.Script{
	0x76, 0xa9, 0x14, 0xcd, 0x43, 0xba, 0x65, 0xce, 0x83, 0x77, 0x8e, 0xf0, 0x4b, 0x20, 0x7d, 0xe1, 0x44, 0x98, 0x44, 0x0f, 0x3b, 0xd4, 0x6c, 0x88, 0xac,
}

func TestComputeFee(t *testing.T) {

	tests := []struct {
		name         string
		satsPerKB    uint64
		txSize       uint64
		tx           *sdkTx.Transaction
		estimatedFee uint64
	}{
		{
			name:         "compute fee based on tx size",
			satsPerKB:    50,
			txSize:       100,
			estimatedFee: 5,
		},
		{
			name:      "compute fee based on tx size",
			satsPerKB: 50,
			tx: &sdkTx.Transaction{
				Inputs: []*sdkTx.TransactionInput{{
					SourceTransaction: &sdkTx.Transaction{
						Outputs: []*sdkTx.TransactionOutput{{
							Satoshis: 150,
						}},
					},
					UnlockingScript: validLockingScript,
				}},
				Outputs: []*sdkTx.TransactionOutput{{
					Satoshis:      100,
					LockingScript: validLockingScript,
				}},
			},
			estimatedFee: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fee1, fee2 uint64
			var err error
			feeModel := SatoshisPerKilobyte{Satoshis: tt.satsPerKB}

			// test ComputeFee
			if tt.tx != nil {
				fee1, err = feeModel.ComputeFee(tt.tx)
				assert.NoError(t, err)
				assert.Equal(t, tt.estimatedFee, fee1)
			}

			// test ComputeFeeBasedOnSize
			if tt.txSize != 0 {
				fee2, err = feeModel.ComputeFeeBasedOnSize(tt.txSize)
			} else {
				fee2, err = feeModel.ComputeFeeBasedOnSize(uint64(tt.tx.Size()))

				// compare the results from both methods
				assert.Equal(t, fee1, fee2)
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.estimatedFee, fee2)
		})
	}
}
