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

const (
	opReturnTx = "010000000000000000ef01478a4ac0c8e4dae42db983bc720d95ed2099dec4c8c3f2d9eedfbeb74e18cdbb1b0100006b483045022100b05368f9855a28f21d3cb6f3e278752d3c5202f1de927862bbaaf5ef7d67adc50220728d4671cd4c34b1fa28d15d5cd2712b68166ea885522baa35c0b9e399fe9ed74121030d4ad284751daf629af387b1af30e02cf5794139c4e05836b43b1ca376624f7fffffffff10000000000000001976a9140c77a935b45abdcf3e472606d3bc647c5cc0efee88ac01000000000000000070006a0963657274696861736822314c6d763150594d70387339594a556e374d3948565473446b64626155386b514e4a406164386337373536356335363935353261626463636634646362353537376164633936633866613933623332663630373865353664666232326265623766353600000000"
	runTx      = "010000000000000000ef0288e59c195e017a9606fcaa21ae75ae670b8d1042380db5eb1860dff6868d349d010000006a4730440220771f717cab9acf745b2448b057b720913c503989262a5291edfd00a7a151fa5e02200d5c5cdd0b9320a796ba7c4e196ff04d5d7be8e7ca069c9af59bb8a2da5dfb41412102028571938947eeceeefac38f0a59f460ea57dc2922047240c1a777cb02261936ffffffff11010000000000001976a91428566dfea52b366fa3f545f7e4ab4392d48ddaae88ac19cb57677947f90549a8b7a207563fe254edce80c042e3ddf06e84e78e6e0934010000006a473044022036bffed646b47f6becea192696b3bf4c4bbee80c29cbc79a9e598c6dce895d3502205e5bc389e805d05b23684469666d8cc81ad3635445df6e8a344d27962016ce94412102213568f72dc2aa813f0154b80d5492157e5c47e69ce0d0ec421d8e3fdb1cde6affffffff404b4c00000000001976a91428c115c42ec654230f1666637d2e72808b1ff46d88ac030000000000000000b1006a0372756e0105004ca67b22696e223a312c22726566223a5b5d2c226f7574223a5b5d2c2264656c223a5b2231376238623534616237363066306635363230393561316664336432306533353865623530653366383638626535393230346462386333343939363337323135225d2c22637265223a5b5d2c2265786563223a5b7b226f70223a2243414c4c222c2264617461223a5b7b22246a6967223a307d2c2264657374726f79222c5b5d5d7d5d7d404b4c00000000001976a91488c05fb97867cab4f4875e5cd4c96929c15f1ca988acf4000000000000001976a9149f4fa07a87b9169f2a66a0456c0c8d4f1209504f88ac00000000"
)

func Test_CalculateFeesRequired(t *testing.T) {
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

func Test_checkScripts(t *testing.T) {
	t.Run("valid op_return tx", func(t *testing.T) {
		tx, err := bt.NewTxFromString(opReturnTx)
		require.NoError(t, err)

		in := tx.Inputs[0]
		prevOutput := &bt.Output{
			Satoshis:      in.PreviousTxSatoshis,
			LockingScript: in.PreviousTxScript,
		}

		err = CheckScript(tx, 0, prevOutput)
		require.NoError(t, err)
	})

	t.Run("valid run tx", func(t *testing.T) {
		tx, err := bt.NewTxFromString(runTx)
		require.NoError(t, err)

		in := tx.Inputs[0]
		prevOutput := &bt.Output{
			Satoshis:      in.PreviousTxSatoshis,
			LockingScript: in.PreviousTxScript,
		}

		err = CheckScript(tx, 0, prevOutput)
		require.NoError(t, err)
	})
}
