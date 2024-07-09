package validator

import (
	"encoding/hex"
	"testing"

	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/require"
)

var validLockingScript = &bscript.Script{
	0x76, 0xa9, 0x14, 0xcd, 0x43, 0xba, 0x65, 0xce, 0x83, 0x77, 0x8e, 0xf0, 0x4b, 0x20, 0x7d, 0xe1, 0x44, 0x98, 0x44, 0x0f, 0x3b, 0xd4, 0x6c, 0x88, 0xac,
}

var opReturnLockingScript = &bscript.Script{
	0x00, 0x6a, 0x4c, 0x4d, 0x41, 0x50, 0x49, 0x20, 0x30, 0x2e, 0x31, 0x2e, 0x30, 0x20, 0x2d, 0x20,
}

func Test_calculateFeesRequired(t *testing.T) {
	defaultFees := bt.NewFeeQuote()
	for _, feeType := range []bt.FeeType{bt.FeeTypeStandard, bt.FeeTypeData} {
		defaultFees.AddQuote(feeType, &bt.Fee{
			MiningFee: bt.FeeUnit{
				Satoshis: 1,
				Bytes:    1000,
			},
		})
	}

	tt := []struct {
		name string
		fees *bt.FeeQuote
		size *bt.TxSize

		expectedRequiredMiningFee uint64
	}{
		{
			name: "1.311 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 1111},

			expectedRequiredMiningFee: 1,
		},
		{
			name: "1.861 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 50, TotalDataBytes: 1811},

			expectedRequiredMiningFee: 2,
		},
		{
			name: "13.31 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 13110},

			expectedRequiredMiningFee: 13,
		},
		{
			name: "18.71 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 18510},

			expectedRequiredMiningFee: 19,
		},
		{
			name: "1.5 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 1300},

			expectedRequiredMiningFee: 2,
		},
		{
			name: "0.8 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 600},

			expectedRequiredMiningFee: 1,
		},
		{
			name: "0.5 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 200, TotalDataBytes: 300},

			expectedRequiredMiningFee: 1,
		},
		{
			name: "0.2 kb size",
			fees: defaultFees,
			size: &bt.TxSize{TotalStdBytes: 100, TotalDataBytes: 100},

			expectedRequiredMiningFee: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			requiredMiningFee, err := CalculateMiningFeesRequired(tc.size, tc.fees)
			require.NoError(t, err)
			require.Equal(t, tc.expectedRequiredMiningFee, requiredMiningFee)
		})
	}
}

func Test_checkTxSize(t *testing.T) {
	type args struct {
		txSize int
		policy *bitcoin.Settings
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid tx size",
			args: args{
				txSize: 100,
				policy: &bitcoin.Settings{
					MaxTxSizePolicy: 10000000,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid tx size",
			args: args{
				txSize: maxBlockSize + 1,
				policy: &bitcoin.Settings{
					MaxTxSizePolicy: 10000000,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkTxSize(tt.args.txSize, tt.args.policy); (err != nil) != tt.wantErr {
				t.Errorf("checkTxSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//nolint:funlen - don't need to check length of test functions
func Test_checkOutputs(t *testing.T) {
	type args struct {
		tx *bt.Tx
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid output",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{Satoshis: 100, LockingScript: validLockingScript}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid satoshis > max",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{Satoshis: maxSatoshis + 1, LockingScript: validLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid satoshis == 0",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{Satoshis: 0, LockingScript: validLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "valid satoshis == 0, op return",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{Satoshis: 0, LockingScript: opReturnLockingScript}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid satoshis, op return",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{Satoshis: 100, LockingScript: opReturnLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid total satoshis",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{
						{Satoshis: maxSatoshis - 100, LockingScript: validLockingScript},
						{Satoshis: maxSatoshis - 100, LockingScript: validLockingScript},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkOutputs(tt.args.tx); (err != nil) != tt.wantErr {
				t.Errorf("checkOutputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkInputs(t *testing.T) {
	type args struct {
		tx *bt.Tx
	}

	coinbaseInput := &bt.Input{}
	coinbaseInput.PreviousTxSatoshis = 100
	_ = coinbaseInput.PreviousTxIDAddStr(coinbaseTxID)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "invalid coinbase input",
			args: args{
				tx: &bt.Tx{Inputs: []*bt.Input{coinbaseInput}},
			},
			wantErr: true,
		},
		{
			name: "valid input",
			args: args{
				tx: &bt.Tx{Inputs: []*bt.Input{{PreviousTxSatoshis: 100}}},
			},
			wantErr: false,
		},
		{
			name: "invalid input satoshis",
			args: args{
				tx: &bt.Tx{Inputs: []*bt.Input{{PreviousTxSatoshis: maxSatoshis + 1}}},
			},
			wantErr: true,
		},
		{
			name: "invalid input satoshis",
			args: args{
				tx: &bt.Tx{
					Inputs: []*bt.Input{{
						PreviousTxSatoshis: maxSatoshis - 100,
					}, {
						PreviousTxSatoshis: maxSatoshis - 100,
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkInputs(tt.args.tx); (err != nil) != tt.wantErr {
				t.Errorf("checkInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_sigOpsCheck(t *testing.T) {
	type args struct {
		tx     *bt.Tx
		policy *bitcoin.Settings
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid sigops",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{{LockingScript: validLockingScript}},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 4294967295,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid sigops - too many sigops",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{
						{
							LockingScript: validLockingScript,
						},
						{
							LockingScript: validLockingScript,
						},
					},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 1,
				},
			},
			wantErr: true,
		},
		{
			name: "valid sigops - default policy",
			args: args{
				tx: &bt.Tx{
					Outputs: []*bt.Output{
						{
							LockingScript: validLockingScript,
						},
						{
							LockingScript: validLockingScript,
						},
					},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 4294967295,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := sigOpsCheck(tt.args.tx, tt.args.policy); (err != nil) != tt.wantErr {
				t.Errorf("sigOpsCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_pushDataCheck(t *testing.T) {
	type args struct {
		tx *bt.Tx
	}

	validUnlockingBytes, _ := hex.DecodeString("4730440220318d23e6fd7dd5ace6e8dc1888b363a053552f48ecc166403a1cc65db5e16aca02203a9ad254cb262f50c89487ffd72e8ddd8536c07f4b230d13a2ccd1435898e89b412102dd7dce95e52345704bbb4df4e4cfed1f8eaabf8260d33597670e3d232c491089")
	validUnlockingScript := bscript.Script(validUnlockingBytes)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid push data",
			args: args{
				tx: &bt.Tx{
					Inputs: []*bt.Input{{
						UnlockingScript: &validUnlockingScript,
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid push data",
			args: args{
				tx: &bt.Tx{
					Inputs: []*bt.Input{{
						UnlockingScript: validLockingScript,
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := pushDataCheck(tt.args.tx); (err != nil) != tt.wantErr {
				t.Errorf("pushDataCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
