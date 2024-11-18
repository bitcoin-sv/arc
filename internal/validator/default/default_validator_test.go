package defaultvalidator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/fees"
	"github.com/bitcoin-sv/arc/internal/testdata"
	validation "github.com/bitcoin-sv/arc/internal/validator"
	fixture "github.com/bitcoin-sv/arc/internal/validator/default/testdata"
	"github.com/bitcoin-sv/arc/internal/validator/mocks"
	"github.com/bitcoin-sv/arc/pkg/api"
)

var validLockingScript = &script.Script{
	0x76, 0xa9, 0x14, 0xcd, 0x43, 0xba, 0x65, 0xce, 0x83, 0x77, 0x8e, 0xf0, 0x4b, 0x20, 0x7d, 0xe1, 0x44, 0x98, 0x44, 0x0f, 0x3b, 0xd4, 0x6c, 0x88, 0xac,
}

const (
	opReturnTx = "010000000000000000ef01478a4ac0c8e4dae42db983bc720d95ed2099dec4c8c3f2d9eedfbeb74e18cdbb1b0100006b483045022100b05368f9855a28f21d3cb6f3e278752d3c5202f1de927862bbaaf5ef7d67adc50220728d4671cd4c34b1fa28d15d5cd2712b68166ea885522baa35c0b9e399fe9ed74121030d4ad284751daf629af387b1af30e02cf5794139c4e05836b43b1ca376624f7fffffffff10000000000000001976a9140c77a935b45abdcf3e472606d3bc647c5cc0efee88ac01000000000000000070006a0963657274696861736822314c6d763150594d70387339594a556e374d3948565473446b64626155386b514e4a406164386337373536356335363935353261626463636634646362353537376164633936633866613933623332663630373865353664666232326265623766353600000000"
	runTx      = "010000000000000000ef0288e59c195e017a9606fcaa21ae75ae670b8d1042380db5eb1860dff6868d349d010000006a4730440220771f717cab9acf745b2448b057b720913c503989262a5291edfd00a7a151fa5e02200d5c5cdd0b9320a796ba7c4e196ff04d5d7be8e7ca069c9af59bb8a2da5dfb41412102028571938947eeceeefac38f0a59f460ea57dc2922047240c1a777cb02261936ffffffff11010000000000001976a91428566dfea52b366fa3f545f7e4ab4392d48ddaae88ac19cb57677947f90549a8b7a207563fe254edce80c042e3ddf06e84e78e6e0934010000006a473044022036bffed646b47f6becea192696b3bf4c4bbee80c29cbc79a9e598c6dce895d3502205e5bc389e805d05b23684469666d8cc81ad3635445df6e8a344d27962016ce94412102213568f72dc2aa813f0154b80d5492157e5c47e69ce0d0ec421d8e3fdb1cde6affffffff404b4c00000000001976a91428c115c42ec654230f1666637d2e72808b1ff46d88ac030000000000000000b1006a0372756e0105004ca67b22696e223a312c22726566223a5b5d2c226f7574223a5b5d2c2264656c223a5b2231376238623534616237363066306635363230393561316664336432306533353865623530653366383638626535393230346462386333343939363337323135225d2c22637265223a5b5d2c2265786563223a5b7b226f70223a2243414c4c222c2264617461223a5b7b22246a6967223a307d2c2264657374726f79222c5b5d5d7d5d7d404b4c00000000001976a91488c05fb97867cab4f4875e5cd4c96929c15f1ca988acf4000000000000001976a9149f4fa07a87b9169f2a66a0456c0c8d4f1209504f88ac00000000"
)

func TestValidator(t *testing.T) {
	t.Parallel()

	t.Run("valid tx", func(t *testing.T) {
		// given
		// extended tx
		tx, _ := sdkTx.NewTransactionFromHex("020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000")
		policy := getPolicy(500)
		sut := New(policy, nil)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.NoError(t, actualError)
	})

	t.Run("invalid tx", func(t *testing.T) {
		// given
		// extended tx
		tx, _ := sdkTx.NewTransactionFromHex("020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00e40b5402000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000")
		policy := getPolicy(500)
		sut := New(policy, nil)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.Error(t, actualError, "Validation should have returned an error")
		if actualError != nil {
			require.ErrorContains(t, actualError, validation.ErrScriptExecutionFailed.Error())
		}
	})

	t.Run("low fee error", func(t *testing.T) {
		// given
		// extended tx
		tx, _ := sdkTx.NewTransactionFromHex("010000000000000000ef01a7968c39fe10ae04686061ab99dc6774f0ebbd8679e521e6fc944d919d9d19a1020000006a4730440220318d23e6fd7dd5ace6e8dc1888b363a053552f48ecc166403a1cc65db5e16aca02203a9ad254cb262f50c89487ffd72e8ddd8536c07f4b230d13a2ccd1435898e89b412102dd7dce95e52345704bbb4df4e4cfed1f8eaabf8260d33597670e3d232c491089ffffffff44040000000000001976a914cd43ba65ce83778ef04b207de14498440f3bd46c88ac013a040000000000001976a9141754f52fc862c7a6106c964c35db7d92a57fec2488ac00000000")
		policy := getPolicy(500)
		sut := New(policy, nil)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.Error(t, actualError)
	})

	t.Run("valid tx 2", func(t *testing.T) {
		// given
		// extended tx
		tx, _ := sdkTx.NewTransactionFromHex("010000000000000000ef01a7968c39fe10ae04686061ab99dc6774f0ebbd8679e521e6fc944d919d9d19a1020000006a4730440220318d23e6fd7dd5ace6e8dc1888b363a053552f48ecc166403a1cc65db5e16aca02203a9ad254cb262f50c89487ffd72e8ddd8536c07f4b230d13a2ccd1435898e89b412102dd7dce95e52345704bbb4df4e4cfed1f8eaabf8260d33597670e3d232c491089ffffffff44040000000000001976a914cd43ba65ce83778ef04b207de14498440f3bd46c88ac013a040000000000001976a9141754f52fc862c7a6106c964c35db7d92a57fec2488ac00000000")
		policy := getPolicy(5)
		sut := New(policy, nil)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.NoError(t, actualError)
	})

	t.Run("valid tx multi", func(t *testing.T) {
		// given
		// All of these transactions should pass...
		txs := []string{
			"020000000000000000ef021c2bff8cb2e37f9018ee6e47512492ee65fa2012ce6c5998b6a2e9583974dabc010000008b473044022029d0a05f2601ee89d63e7a61a8f5877f20e7a48214d3aa6e8421bb938feec8a80220785478ad3019ec91c5545199539ccfd5704021f1c962becd48e0264f7e16de86c32102207d0891b88c096f1f8503a684c387b4f9440c80707118ec14227adadd15db7820c8925e7b008668089d3ae1fc1cf450f7f45f0b4af43cd7d30b84446ecb374d6dffffffff8408000000000000fd6103a914179b4c7a45646a509473df5a444b6e18b723bd148876a9142e0fa8744508c13de3fe065d7ed2016370cc433f88ac6a4d2d037b227469746c65223a2246726f672043617274656c202331373935222c226465736372697074696f6e223a2246726f6773206d75737420756e69746520746f2064657374726f7920746865206c697a617264732e20446f20796f75206861766520776861742069742074616b65733f222c22696d616765223a22623a2f2f61353333663036313134353665333438326536306136666433346337663165366265393365663134303261396639363139313539306334303534326230306335222c226e756d626572223a313739352c22736572696573223a333639302c2273636f7265223a2235392e3131222c2272616e6b223a333033382c22726172697479223a22436f6d6d6f6e222c2261747472696275746573223a5b7b2274726169745f74797065223a224261636b67726f756e64222c2276616c7565223a225465616c204a756d626c65222c22636f756e74223a3131352c22726172697479223a22556e636f6d6d6f6e227d2c7b2274726169745f74797065223a2246726f67222c2276616c7565223a22526574726f20426c7565222c22636f756e74223a3433322c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a22426f6479222c2276616c7565223a22507572706c6520466c616e6e656c222c22636f756e74223a36342c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a224d6f757468222c2276616c7565223a224e6f204d6f757468204974656d222c22636f756e74223a313335382c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a2245796573222c2276616c7565223a224f72616e676520457965205061746368222c22636f756e74223a3130332c22726172697479223a2252617265227d2c7b2274726169745f74797065223a2248656164222c2276616c7565223a2250657420436869636b222c22636f756e74223a36392c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a2248616e64222c2276616c7565223a224e6f2048616e64204974656d222c22636f756e74223a3939322c22726172697479223a22436f6d6d6f6e227d5d7d215b80a60dc756a488066fa95b90cceec4fd731ef489d51047b41e7aa5a95bf0040000006a47304402203951e4ebccaa652e360d8b2fab2ea9936a1eec19f27d6a1d9791c32b4e46540e02202529a8af4795bcf7dfe9dbb4826bb9f1467cc255de947e8c07a5961287aa713e41210253fe24fd82a07d02010d9ca82f99870c0e5e7402a9b26c9d25ae753e40754c4dffffffff96191d44000000001976a914b522239693bae79c76208eed6fbab62b0e1fba2e88ac0544ca0203000000001976a9142e0fa8744508c13de3fe065d7ed2016370cc433f88ac8408000000000000fd6103a914179b4c7a45646a509473df5a444b6e18b723bd148876a91497e5faf26e48d9015269c2592c6e4886ac2d161288ac6a4d2d037b227469746c65223a2246726f672043617274656c202331373935222c226465736372697074696f6e223a2246726f6773206d75737420756e69746520746f2064657374726f7920746865206c697a617264732e20446f20796f75206861766520776861742069742074616b65733f222c22696d616765223a22623a2f2f61353333663036313134353665333438326536306136666433346337663165366265393365663134303261396639363139313539306334303534326230306335222c226e756d626572223a313739352c22736572696573223a333639302c2273636f7265223a2235392e3131222c2272616e6b223a333033382c22726172697479223a22436f6d6d6f6e222c2261747472696275746573223a5b7b2274726169745f74797065223a224261636b67726f756e64222c2276616c7565223a225465616c204a756d626c65222c22636f756e74223a3131352c22726172697479223a22556e636f6d6d6f6e227d2c7b2274726169745f74797065223a2246726f67222c2276616c7565223a22526574726f20426c7565222c22636f756e74223a3433322c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a22426f6479222c2276616c7565223a22507572706c6520466c616e6e656c222c22636f756e74223a36342c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a224d6f757468222c2276616c7565223a224e6f204d6f757468204974656d222c22636f756e74223a313335382c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a2245796573222c2276616c7565223a224f72616e676520457965205061746368222c22636f756e74223a3130332c22726172697479223a2252617265227d2c7b2274726169745f74797065223a2248656164222c2276616c7565223a2250657420436869636b222c22636f756e74223a36392c22726172697479223a22436f6d6d6f6e227d2c7b2274726169745f74797065223a2248616e64222c2276616c7565223a224e6f2048616e64204974656d222c22636f756e74223a3939322c22726172697479223a22436f6d6d6f6e227d5d7de4d41e00000000001976a91497df51a1dea118bd689099125b42d75e48d2f5ec88ac30e51700000000001976a91484c9b30c0e3529a6d260b361f70902f962d4b77088acec93e340000000001976a914863f485dae59224cc5993b26bf50da2e7c368c8a88ac00000000",
			"010000000000000000ef01452fcd2374c548a6bac1aa76ae8efe6bde1986a8c1d67b8523eea24510769b83020000006a47304402202e032a7595a57ffd7b904814b03b971dffa62adcbb233d0eb55e0520ee385d6402205f8fbe55c1a056f5b712df4e13747dd6520d11d40760b86f22fa3e89383148834121021dc87a5ec40540d21076ecb615440eccecb36c1c6fa950f81cab6d51745ad613ffffffffc3030000000000001976a914ca5134130f26388f871071433024742449e3431688ac0396000000000000001976a91425ede77d31c4791504fd5121f70772722118744e88ac0000000000000000f5006a4cf17b0a2020202020202020202020206f7267616e697a6174696f6e3a204861737465204172636164650a202020202020202020202020636f6e746573743a204c6f73742c0a2020202020202020202020206c6576656c3a204e616e6f2c0a2020202020202020202020206576656e74547970653a20696e7075742c0a2020202020202020202020206576656e7449643a2030326665373330362d656137372d343736652d626462612d3666626134353330303061352c0a20202020202020202020202076616c75653a2036342c0a202020202020202020202020636f73743a2032313336390a202020202020202020207d7d14030000000000001976a91431302ded0a12c8c0559951ac9315685f97e592df88ac00000000",
			"010000000000000000ef018f06f2c9a3109dc1f69ab0f37a3c155a2db6928c3cc79c0270640f2571f261d1020000006a473044022013f11686546b575711b68e9194c74787f36a2028cbbab0afc974bf6ab6807f0b02207e9bc0134bca25413bd14bc84cb9316f9188a3cf49dbf9829ed9e60bb730d5d3412103b9ac16dfb008350c1a6eeb8e25c8455beab90f7cc328b0194059a6a87622139fffffffff9f030000000000001976a9142bce53f35d8bed5fce79be8140e679ce2e62e11588ac0396000000000000001976a9145721fc851ee528b2059eb7af160ffa8e511e62f388ac0000000000000000fdff00006a4cfb7b0a2020202020202020202020206f7267616e697a6174696f6e3a204861737465204172636164650a202020202020202020202020636f6e746573743a20527566662052756e6e65722c0a2020202020202020202020206c6576656c3a204e616e6f2c0a2020202020202020202020206576656e74547970653a20696e7075742c0a2020202020202020202020206576656e7449643a2065303238396363392d363137362d343666342d393430352d3763316137653538333266622c0a20202020202020202020202076616c75653a2036383934382c0a202020202020202020202020636f73743a2032313336390a202020202020202020207d7df0020000000000001976a914afdba4a0962bf2ff5e6b62580a247e8e29f97d3788ac00000000",
		}

		for txIndex, txStr := range txs {
			tx, err := sdkTx.NewTransactionFromHex(txStr)
			require.NoError(t, err, "Could not parse tx hex")
			policy := getPolicy(5)
			sut := New(policy, nil)

			// when
			actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

			// then
			require.NoError(t, actualError, "Failed to validate tx %d", txIndex)
		}
	})

	t.Run("valid from file", func(t *testing.T) {
		// given
		f, err := os.Open("testdata/1.bin")
		require.NoError(t, err, "Failed to open file")
		defer f.Close()

		tx := &sdkTx.Transaction{}
		_, err = tx.ReadFrom(f)
		require.NoError(t, err, "Failed to read tx from reader")

		parentHexes := []string{
			"010000000115e8e47a1bf08cbb37513cc5f54894cae1ba8c2fbcc95213ab40bcfece140be9030000006b483045022100cc6ebfaeeb001a9148ef482a73c394f0a7b82d8c9d9e695af921015766c0f34e0220717e975e05e680463581de58d32736f427ac98eb26c4fd851b7f1d8633b98513412102efb53ff430d849a88636d90d777cb53db5dd83f8fe907a6b52662003443546aeffffffff02550c0000000000001976a9142b235c633316792ccd322bfed5ef77ffcdbabcf588ac9a020000000000001976a91405186ff0711831d110ca96ddfc47816b5a31900d88ac00000000",
		}

		for index, in := range tx.Inputs {
			parentHex := parentHexes[index]

			parentTx, err := sdkTx.NewTransactionFromHex(parentHex)
			require.NoError(t, err, "Could not parse parent tx hex")

			in.SetPrevTxFromOutput(parentTx.Outputs[in.SourceTxOutIndex])
		}

		policy := getPolicy(5)
		sut := New(policy, nil)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.NoError(t, actualError, "Failed to validate tx")
	})

	t.Run("valid Raw Format tx - expect success", func(t *testing.T) {
		// given
		txFinder := mocks.TxFinderIMock{
			GetRawTxsFunc: func(_ context.Context, _ validation.FindSourceFlag, _ []string) ([]*sdkTx.Transaction, error) {
				res := []*sdkTx.Transaction{fixture.ParentTx1}
				return res, nil
			},
		}

		rawTx := fixture.ValidTx

		sut := New(getPolicy(5), &txFinder)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), rawTx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		require.NoError(t, actualError)
	})
}

func getPolicy(satoshisPerKB uint64) *bitcoin.Settings {
	var policy *bitcoin.Settings

	_ = json.Unmarshal([]byte(testdata.DefaultPolicy), &policy)

	policy.MinMiningTxFee = float64(satoshisPerKB) / 1e8
	return policy
}

// no need to extensively test this function, it's just calling isFeePaidEnough
func TestStandardCheckFees(t *testing.T) {
	type args struct {
		tx       *sdkTx.Transaction
		feeModel *fees.SatoshisPerKilobyte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no fee being paid",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						SourceTransaction: &sdkTx.Transaction{
							Outputs: []*sdkTx.TransactionOutput{{
								Satoshis: 100,
							}},
						},
					}},
					Outputs: []*sdkTx.TransactionOutput{{
						Satoshis:      100,
						LockingScript: validLockingScript,
					}},
				},
				feeModel: &fees.SatoshisPerKilobyte{Satoshis: 5},
			},
			wantErr: true,
		},
		{
			name: "valid fee being paid",
			args: args{
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
				feeModel: &fees.SatoshisPerKilobyte{Satoshis: 5},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := standardCheckFees(tt.args.tx, tt.args.feeModel); (err != nil) != tt.wantErr {
				t.Errorf("standardCheckFees() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStandardCheckFeesTxs(t *testing.T) {
	t.Run("no fee being paid", func(t *testing.T) {
		// given
		tx, err := sdkTx.NewTransactionFromHex(opReturnTx)
		require.NoError(t, err)
		sut := &fees.SatoshisPerKilobyte{Satoshis: 50}

		// when
		actualError := standardCheckFees(tx, sut)

		// then
		require.Nil(t, actualError)
	})
}

func TestCheckScripts(t *testing.T) {
	t.Run("valid op_return tx", func(t *testing.T) {
		// given
		tx, err := sdkTx.NewTransactionFromHex(opReturnTx)
		require.NoError(t, err)

		// when
		actualError := checkScripts(tx)

		// then
		require.NoError(t, actualError)
	})

	t.Run("valid run tx", func(t *testing.T) {
		// given
		tx, err := sdkTx.NewTransactionFromHex(runTx)
		require.NoError(t, err)

		// when
		actualError := checkScripts(tx)

		// then
		require.NoError(t, actualError)
	})
}

func BenchmarkValidator(b *testing.B) {
	// extended tx
	tx, _ := sdkTx.NewTransactionFromHex("020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000")
	policy := getPolicy(500)
	sut := New(policy, nil)

	for i := 0; i < b.N; i++ {
		_ = sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)
	}
}

func TestFeeCalculation(t *testing.T) {
	// given
	tx, err := sdkTx.NewTransactionFromHex("010000000000000000ef03778462c25ddb306d312b422885446f26e3e0455e493a4d81daffe06961aae985c80000006a473044022001762f052785e65bc38512c77712e026088caee394122fe9dff95c577b16dfdf022016de0b27ea5068151ed19b9685f21164c794c23acdb9a407169bc65cb3bb857b412103ee7da140fd1e2385ef2e8eba1340cc87c55387f361449807eb6c15dcbb7f1109ffffffff7bd53001000000001976a9145f2410d051d4722f637395d00f5c0c4a8818e2d388ac7a629df9166996224ebbe6225388c8a0f6cbc21853e831cf52764270ac5f37ec000000006a473044022006a82dd662f9b21bfa2cd770a222bf359031ba02c72c6cbb2122c0cf31b7bd93022034d674785bd89bf5b4d9b59851f4342cc1058da4a05fd13b31984423c79c8a2f412103ee7da140fd1e2385ef2e8eba1340cc87c55387f361449807eb6c15dcbb7f1109ffffffffd0070000000000001976a9145f2410d051d4722f637395d00f5c0c4a8818e2d388ac7a629df9166996224ebbe6225388c8a0f6cbc21853e831cf52764270ac5f37ec010000006b483045022100f6340e82cd38b4e99d5603433a260fbc5e2b5a6978f75c60335401dc2e86f82002201d816a3b2219811991b767fa7902a3d3c54c03a7d2f6a6d23745c9c586ac7352412103ee7da140fd1e2385ef2e8eba1340cc87c55387f361449807eb6c15dcbb7f1109ffffffff05020000000000001976a9145f2410d051d4722f637395d00f5c0c4a8818e2d388ac0b1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288ac1e000000000000001976a91498a2231556226331b456cd326f9085cbaff6240288acfbdd3001000000001976a9145f2410d051d4722f637395d00f5c0c4a8818e2d388ac00000000")
	require.NoError(t, err)
	policy := getPolicy(50)
	sut := New(policy, nil)

	// when
	err = sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

	// then
	t.Log(err)
}

func TestNeedExtension(t *testing.T) {
	tcs := []struct {
		name           string
		txHex          string
		feeOpt         validation.FeeValidation
		scriptOpt      validation.ScriptValidation
		expectedResult bool
	}{
		{
			name:           "raw hex - expect true",
			txHex:          testdata.TX1RawString,
			feeOpt:         validation.StandardFeeValidation,
			scriptOpt:      validation.StandardScriptValidation,
			expectedResult: true,
		},
		{
			name:           "ef hex - expect false",
			txHex:          "020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000",
			feeOpt:         validation.StandardFeeValidation,
			scriptOpt:      validation.StandardScriptValidation,
			expectedResult: false,
		},
		{
			name:           "raw hex, skip fee and script validation - expect false",
			txHex:          testdata.TX1RawString,
			feeOpt:         validation.NoneFeeValidation,
			scriptOpt:      validation.NoneScriptValidation,
			expectedResult: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)

			// when
			actualResult := needsExtension(tx, tc.feeOpt, tc.scriptOpt)

			// then
			require.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestCumulativeCheckFees(t *testing.T) {
	txMap := map[string]*sdkTx.Transaction{
		fixture.ParentTxID1:              fixture.ParentTx1,
		fixture.AncestorTxID1:            fixture.AncestorTx1,
		fixture.AncestorOfAncestorTx1ID1: fixture.AncestorOfAncestor1Tx1,
	}

	tcs := []struct {
		name                   string
		feeModel               *fees.SatoshisPerKilobyte
		mempoolAncestors       []string
		getMempoolAncestorsErr error
		getRawTxsErr           error

		expectedErr *validation.Error
	}{
		{
			name: "no unmined ancestors - valid fee",
			feeModel: func() *fees.SatoshisPerKilobyte {
				return &fees.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{},
		},
		{
			name: "no unmined ancestors - too low fee",
			feeModel: func() *fees.SatoshisPerKilobyte {
				return &fees.SatoshisPerKilobyte{Satoshis: 50}
			}(),
			mempoolAncestors: []string{},

			expectedErr: validation.NewError(ErrTxFeeTooLow, api.ErrStatusCumulativeFees),
		},
		{
			name: "cumulative fees too low",
			feeModel: func() *fees.SatoshisPerKilobyte {
				return &fees.SatoshisPerKilobyte{Satoshis: 50}
			}(),
			mempoolAncestors: []string{fixture.AncestorTxID1},

			expectedErr: validation.NewError(ErrTxFeeTooLow, api.ErrStatusCumulativeFees),
		},
		{
			name: "cumulative fees sufficient",
			feeModel: func() *fees.SatoshisPerKilobyte {
				return &fees.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{fixture.AncestorTxID1},
		},
		{
			name: "failed to get mempool ancestors",
			feeModel: func() *fees.SatoshisPerKilobyte {
				return &fees.SatoshisPerKilobyte{Satoshis: 5}
			}(),
			getMempoolAncestorsErr: errors.New("some error"),

			expectedErr: validation.NewError(
				ErrFailedToGetMempoolAncestors,
				api.ErrStatusCumulativeFees),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			txFinder := &mocks.TxFinderIMock{
				GetMempoolAncestorsFunc: func(_ context.Context, _ []string) ([]string, error) {
					return tc.mempoolAncestors, tc.getMempoolAncestorsErr
				},
				GetRawTxsFunc: func(_ context.Context, _ validation.FindSourceFlag, ids []string) ([]*sdkTx.Transaction, error) {
					rawTxs := make([]*sdkTx.Transaction, len(ids))
					for i, id := range ids {
						rawTx, ok := txMap[id]
						if !ok {
							t.Fatalf("tx id %s not found", id)
						}
						rawTxs[i] = rawTx
					}

					return rawTxs, tc.getRawTxsErr
				},
			}
			tx := fixture.ValidTx

			err := extendTx(context.TODO(), txFinder, tx, false)
			require.NoError(t, err)

			// when
			actualError := cumulativeCheckFees(context.TODO(), txFinder, tx, tc.feeModel, false)

			// then
			if tc.expectedErr == nil {
				require.Nil(t, actualError)
			} else {
				require.NotNil(t, actualError)
				assert.ErrorIs(t, actualError.Err, tc.expectedErr.Err)
				assert.Equal(t, tc.expectedErr.ArcErrorStatus, actualError.ArcErrorStatus)
			}
		})
	}
}
