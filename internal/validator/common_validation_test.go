package validator

import (
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/require"
)

var validLockingScript = &script.Script{
	0x76, 0xa9, 0x14, 0xcd, 0x43, 0xba, 0x65, 0xce, 0x83, 0x77, 0x8e, 0xf0, 0x4b, 0x20, 0x7d, 0xe1, 0x44, 0x98, 0x44, 0x0f, 0x3b, 0xd4, 0x6c, 0x88, 0xac,
}

var opReturnLockingScript = &script.Script{
	0x00, 0x6a, 0x4c, 0x4d, 0x41, 0x50, 0x49, 0x20, 0x30, 0x2e, 0x31, 0x2e, 0x30, 0x20, 0x2d, 0x20,
}

func TestCheckTxSize(t *testing.T) {
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
			name: "valid tx size, zero MaxTxSizePolicy",
			args: args{
				txSize: 100,
				policy: &bitcoin.Settings{
					MaxTxSizePolicy: 0,
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
func TestCheckOutputs(t *testing.T) {
	type args struct {
		tx *sdkTx.Transaction
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid output",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{Satoshis: 100, LockingScript: validLockingScript}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid satoshis > max",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{Satoshis: maxSatoshis + 1, LockingScript: validLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid satoshis == 0",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{Satoshis: 0, LockingScript: validLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "valid satoshis == 0, op return",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{Satoshis: 0, LockingScript: opReturnLockingScript}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid satoshis, op return",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{Satoshis: 100, LockingScript: opReturnLockingScript}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid total satoshis",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{
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

func TestCheckInputs(t *testing.T) {
	type args struct {
		tx *sdkTx.Transaction
	}
	txIDbytes, err := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	require.NoError(t, err)
	sourceTxHash, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)
	coinbaseInput := &sdkTx.TransactionInput{SourceTXID: sourceTxHash}
	coinbaseInput.SetSourceTxOutput(&sdkTx.TransactionOutput{Satoshis: 100})
	hexTxID, _ := hex.DecodeString(coinbaseTxID)
	hash, _ := chainhash.NewHash(hexTxID)
	coinbaseInput.SourceTXID = hash

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "invalid coinbase input",
			args: args{
				tx: &sdkTx.Transaction{Inputs: []*sdkTx.TransactionInput{coinbaseInput}},
			},
			wantErr: true,
		},
		{
			name: "valid input",
			args: args{
				tx: &sdkTx.Transaction{Inputs: []*sdkTx.TransactionInput{{SourceTXID: sourceTxHash, SourceTransaction: &sdkTx.Transaction{Outputs: []*sdkTx.TransactionOutput{{Satoshis: 100}}}}}},
			},
			wantErr: false,
		},
		{
			name: "invalid input satoshis",
			args: args{
				tx: &sdkTx.Transaction{Inputs: []*sdkTx.TransactionInput{{SourceTXID: sourceTxHash, SourceTransaction: &sdkTx.Transaction{Outputs: []*sdkTx.TransactionOutput{{Satoshis: maxSatoshis + 1}}}}}},
			},
			wantErr: true,
		},
		{
			name: "invalid input satoshis",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						SourceTXID:        sourceTxHash,
						SourceTransaction: &sdkTx.Transaction{Outputs: []*sdkTx.TransactionOutput{{Satoshis: maxSatoshis - 100}}},
					}, {
						SourceTXID:        sourceTxHash,
						SourceTransaction: &sdkTx.Transaction{Outputs: []*sdkTx.TransactionOutput{{Satoshis: maxSatoshis - 100}}},
					},
					},
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

func TestSigOpsCheck(t *testing.T) {
	type args struct {
		tx     *sdkTx.Transaction
		policy *bitcoin.Settings
	}
	validUnlockingBytes, _ := hex.DecodeString("4730440220318d23e6fd7dd5ace6e8dc1888b363a053552f48ecc166403a1cc65db5e16aca02203a9ad254cb262f50c89487ffd72e8ddd8536c07f4b230d13a2ccd1435898e89b412102dd7dce95e52345704bbb4df4e4cfed1f8eaabf8260d33597670e3d232c491089")
	validUnlockingScript := script.Script(validUnlockingBytes)
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid sigops",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{LockingScript: validLockingScript}},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 4294967295,
				},
			},
			wantErr: false,
		},
		{
			name: "valid sigops with inputs",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						UnlockingScript: &validUnlockingScript,
					}},
					Outputs: []*sdkTx.TransactionOutput{{LockingScript: validLockingScript}},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 4294967295,
				},
			},
			wantErr: false,
		},

		{
			name: "valid sigops, zero MaxTxSigopsCountsPolicy",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{{LockingScript: validLockingScript}},
				},
				policy: &bitcoin.Settings{
					MaxTxSigopsCountsPolicy: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid sigops - too many sigops",
			args: args{
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{
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
				tx: &sdkTx.Transaction{
					Outputs: []*sdkTx.TransactionOutput{
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

func TestPushDataCheck(t *testing.T) {
	type args struct {
		tx *sdkTx.Transaction
	}

	validUnlockingBytes, _ := hex.DecodeString("4730440220318d23e6fd7dd5ace6e8dc1888b363a053552f48ecc166403a1cc65db5e16aca02203a9ad254cb262f50c89487ffd72e8ddd8536c07f4b230d13a2ccd1435898e89b412102dd7dce95e52345704bbb4df4e4cfed1f8eaabf8260d33597670e3d232c491089")
	validUnlockingScript := script.Script(validUnlockingBytes)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid push data",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						UnlockingScript: &validUnlockingScript,
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid push data - invalid unlocking script",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						UnlockingScript: nil,
					}},
				},
			},
			wantErr: true,
		}, {
			name: "invalid push data",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
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
