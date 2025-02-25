package defaultvalidator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/bitcoin-sv/go-sdk/chainhash"
	"github.com/bitcoin-sv/go-sdk/script"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	feemodel "github.com/bitcoin-sv/go-sdk/transaction/fee_model"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	//tt := []struct {
	//	name  string
	//	txHex string
	//
	//	expectedError error
	//}{
	//	{
	//		name:  "valid tx",
	//		txHex: "020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000",
	//	},
	//	{
	//		name:  "invalid tx",
	//		txHex: "020000000000000000ef010f117b3f9ea4955d5c592c61838bea10096fc88ac1ad08561a9bcabd715a088200000000494830450221008fd0e0330470ac730b9f6b9baf1791b76859cbc327e2e241f3ebeb96561a719602201e73532eb1312a00833af276d636254b8aa3ecbb445324fb4c481f2a493821fb41feffffff00f2052a01000000232103b12bda06e5a3e439690bf3996f1d4b81289f4747068a5cbb12786df83ae14c18ac02a0860100000000001976a914b7b88045cc16f442a0c3dcb3dc31ecce8d156e7388ac605c042a010000001976a9147a904b8ae0c2f9d74448993029ad3c040ebdd69a88ac66000000",
	//
	//		expectedError: validation.ErrScriptExecutionFailed,
	//	},
	//}
	//
	//for _, tc := range tt {
	//	t.Run(tc.name, func(t *testing.T) {
	//		// given
	//		// extended tx
	//		tx, _ := sdkTx.NewTransactionFromHex(tc.txHex)
	//		policy := getPolicy(500)
	//		sut := New(policy, nil)
	//
	//		// when
	//		actualError := sut.ValidateTransaction(context.TODO(), tx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)
	//
	//		// then
	//
	//		if tc.expectedError != nil {
	//			require.ErrorIs(t, actualError, tc.expectedError)
	//			return
	//		}
	//		require.NoError(t, actualError)
	//	})
	//}

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

			in.SetSourceTxOutput(parentTx.Outputs[in.SourceTxOutIndex])
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
			GetRawTxsFunc: func(_ context.Context, _ validation.FindSourceFlag, _ []string) []*sdkTx.Transaction {
				return []*sdkTx.Transaction{fixture.ParentTx1}
			},
		}

		rawTx := fixture.ValidTx

		sut := New(getPolicy(5), &txFinder)

		// when
		actualError := sut.ValidateTransaction(context.TODO(), rawTx, validation.StandardFeeValidation, validation.StandardScriptValidation, false)

		// then
		//require.ErrorIs(t, actualError, ErrTxFeeTooLow)
		require.ErrorContains(t, actualError, "minimum expected fee: 5, actual fee: 1")
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
	txIDbytes, err := hex.DecodeString("4a2992fa3af9eb7ff6b94dc9e27e44f29a54ab351ee6377455409b0ebbe1f00c")
	require.NoError(t, err)
	sourceTxHash, err := chainhash.NewHash(txIDbytes)
	require.NoError(t, err)
	type args struct {
		tx       *sdkTx.Transaction
		feeModel *feemodel.SatoshisPerKilobyte
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
						SourceTXID: sourceTxHash,
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
				feeModel: &feemodel.SatoshisPerKilobyte{Satoshis: 5},
			},
			wantErr: true,
		},
		{
			name: "valid fee being paid",
			args: args{
				tx: &sdkTx.Transaction{
					Inputs: []*sdkTx.TransactionInput{{
						SourceTXID: sourceTxHash,
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
				feeModel: &feemodel.SatoshisPerKilobyte{Satoshis: 5},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkStandardFees(tt.args.tx, tt.args.feeModel); (err != nil) != tt.wantErr {
				t.Errorf("checkStandardFees() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStandardCheckFeesTxs(t *testing.T) {
	t.Run("no fee being paid", func(t *testing.T) {
		// given
		tx, err := sdkTx.NewTransactionFromHex(opReturnTx)
		require.NoError(t, err)
		sut := &feemodel.SatoshisPerKilobyte{Satoshis: 50}

		// when
		actualError := checkStandardFees(tx, sut)

		// then
		require.ErrorContains(t, actualError, ErrTxFeeTooLow.Error())
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

func TestGetUnminedAncestors(t *testing.T) {
	txMap := map[string]*sdkTx.Transaction{
		fixture.ParentTxID1:              fixture.ParentTx1,
		fixture.AncestorTxID1:            fixture.AncestorTx1,
		fixture.AncestorOfAncestorTx1ID1: fixture.AncestorOfAncestor1Tx1,
	}
	tx := fixture.ValidTx

	tcs := []struct {
		name                   string
		feeModel               *feemodel.SatoshisPerKilobyte
		mempoolAncestors       []string
		getMempoolAncestorsErr error

		expectedTxSet map[string]*sdkTx.Transaction
		expectedErr   error
	}{
		{
			name: "no unmined ancestors - valid fee",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{},
			expectedTxSet:    map[string]*sdkTx.Transaction{},
		},
		{
			name: "cumulative fees sufficient",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{fixture.AncestorTxID1},
			expectedTxSet:    map[string]*sdkTx.Transaction{"147a55659a647b9931d8d4797dfbdd2ca8d34e56940e2463f818e8801e3ff003": fixture.AncestorTx1},
		},
		{
			name: "failed to get mempool ancestors",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 5}
			}(),
			getMempoolAncestorsErr: errors.New("some error"),

			expectedErr: ErrFailedToGetMempoolAncestors,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			txFinder := &mocks.TxFinderIMock{
				GetMempoolAncestorsFunc: func(_ context.Context, _ []string) ([]string, error) {
					return tc.mempoolAncestors, tc.getMempoolAncestorsErr
				},
				GetRawTxsFunc: func(_ context.Context, _ validation.FindSourceFlag, ids []string) []*sdkTx.Transaction {
					rawTxs := make([]*sdkTx.Transaction, len(ids))
					for i, id := range ids {
						rawTx, ok := txMap[id]
						if !ok {
							t.Fatalf("tx id %s not found", id)
						}
						rawTxs[i] = rawTx
					}

					return rawTxs
				},
			}

			actualTxSet, actualError := getUnminedAncestors(context.TODO(), txFinder, tx, false)

			// then
			if tc.expectedErr != nil {
				require.ErrorIs(t, actualError, tc.expectedErr)
				return
			}

			require.NoError(t, actualError)
			require.Equal(t, tc.expectedTxSet, actualTxSet)
		})
	}
}

func TestExtendTxCheckCumulativeFees(t *testing.T) {
	txMap := map[string]*sdkTx.Transaction{
		fixture.ParentTxID1:              fixture.ParentTx1,
		fixture.AncestorTxID1:            fixture.AncestorTx1,
		fixture.AncestorOfAncestorTx1ID1: fixture.AncestorOfAncestor1Tx1,
	}
	tx := fixture.ValidTx

	tcs := []struct {
		name                   string
		feeModel               *feemodel.SatoshisPerKilobyte
		mempoolAncestors       []string
		getMempoolAncestorsErr error

		expectedErr *validation.Error
	}{
		{
			name: "no unmined ancestors - valid fee",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{},
		},
		{
			name: "no unmined ancestors - too low fee",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 50}
			}(),
			mempoolAncestors: []string{},

			expectedErr: validation.NewError(ErrTxFeeTooLow, api.ErrStatusCumulativeFees),
		},
		{
			name: "cumulative fees too low",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 50}
			}(),
			mempoolAncestors: []string{fixture.AncestorTxID1},

			expectedErr: validation.NewError(ErrTxFeeTooLow, api.ErrStatusCumulativeFees),
		},
		{
			name: "cumulative fees sufficient",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 1}
			}(),
			mempoolAncestors: []string{fixture.AncestorTxID1},
		},
		{
			name: "failed to get mempool ancestors",
			feeModel: func() *feemodel.SatoshisPerKilobyte {
				return &feemodel.SatoshisPerKilobyte{Satoshis: 5}
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
				GetRawTxsFunc: func(_ context.Context, _ validation.FindSourceFlag, ids []string) []*sdkTx.Transaction {
					rawTxs := make([]*sdkTx.Transaction, len(ids))
					for i, id := range ids {
						rawTx, ok := txMap[id]
						if !ok {
							t.Fatalf("tx id %s not found", id)
						}
						rawTxs[i] = rawTx
					}

					return rawTxs
				},
			}

			err := extendTx(context.TODO(), txFinder, tx, false)
			require.NoError(t, err)

			// when
			actualError := checkCumulativeFees(context.TODO(), txMap, tx, tc.feeModel, false)

			// then
			if tc.expectedErr == nil {
				require.NoError(t, actualError)
			} else {
				require.NotNil(t, actualError)
				assert.ErrorIs(t, actualError.Err, tc.expectedErr.Err)
				assert.Equal(t, tc.expectedErr.ArcErrorStatus, actualError.ArcErrorStatus)
			}
		})
	}
}

func TestCheckCumulativeFees(t *testing.T) {
	t.Run("chained txs", func(t *testing.T) {
		txSet := map[string]*sdkTx.Transaction{}
		var tx *sdkTx.Transaction
		var feeModel = &feemodel.SatoshisPerKilobyte{Satoshis: 1}
		var err error

		for key, value := range txSetString {
			newTx, err := sdkTx.NewTransactionFromHex(value)
			require.NoError(t, err)

			txSet[key] = newTx
		}

		tx, err = sdkTx.NewTransactionFromHex("010000000000000000ef014d576b0afb43187212360592d12dd04f63b6147d9aec39c01d5638c390d91a4d020000006a4730440220460d4b28667e6a126e3bd88056bed4e4c45077a2ae435812d577606a4d15575c0220650aecfe1483a839ef1c0ac1a0566914b44cb19692fa78541f71607873731cc54121035c14baa37bcb08808d2e384f4695a50e7db3f472f6655cc23baee855c8a0f240ffffffff77690000000000001976a914b35e5c4f05976458be36d78d7a20149f51507e3e88ac03204e0000000000001976a9145394dfb37952a362598e815e89c44d877a2b53e288aca00f0000000000001976a914a6771db04fb78fe0845311800a53ab9c0a44d8b388acb60b0000000000001976a9149b48f6e263a505930f1e6fdc157c9771492786c588ac00000000")
		require.NoError(t, err)

		vErr := checkCumulativeFees(context.TODO(), txSet, tx, feeModel, false)

		require.NotNil(t, vErr)
		require.ErrorIs(t, vErr.Err, ErrTxFeeTooLow)
		require.ErrorContains(t, vErr.Err, "minimum expected cumulative fee: 31, actual fee: 20")
	})
}

var (
	txSetString = map[string]string{
		"7cc40d25faf0625d0aa548c5e8c96d5dce7d22bdbf4b5f313f33b2a1fca81d30": "010000000000000000ef017d4ad8a21caf35335187dbe6336ead2582b77405ca95ea3e86f9a376ac36428e010000006a473044022042b31adde0ac3bcb02d0434521073d34729fcfc41b8b4846ad75549f7eae5e3802204f5673cb0089d0b7475caa316ccd8b23ea654987c44e0cedffe52797b16a689f4121030ad8bb821c95c09afcb1095ba1e5daecac233b4c7accf9a5d645185fadbe4cfcffffffff7b920000000000001976a9144cbce37df1fe2e2692010b47c2f42dcd8069b5b888ac02e4570000000000001976a9142b6bdd30e74ce66a48839237e8e92b6e79faa94888ac963a0000000000001976a914566e38ba79db2e6f59d9c63399b15e5729d02ffa88ac00000000",
		"9bc38c17da3e275178d1c796194e4453aad4d74f76847bc3cdb66f3df4ff2b77": "010000000000000000ef049e10f72baabe0aeb2128482456bfb882c929eb9441ca0632f8a76202e6c18765010000006b483045022100f79db803ffb0c988a986e3b086ec6ff021f1ccf0ccfef14398a7afebad5392bd02206741288739403ba7cd92d0773012df4cf366589fca64c735e553173d0a5c832541210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff97280000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac9e10f72baabe0aeb2128482456bfb882c929eb9441ca0632f8a76202e6c18765020000006b483045022100d8b83572efdac780864b9571233609a42d1265c881cb65e7cb5359a4ea317266022037627f2895f91861e9d4b9b09d2d6d7a4e42f294fcbec66736993f1f45e89ab541210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffffab280000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac2bad7d6fe367696b927b221ef16983f14432f24da43b396503b1035ee1ff9995010000006a47304402202d5f10e5da1ff4dbe3ca4cdd7100511c9ce8adc899e138b448076f4e7aa4c15b022022e0ad041da9ce46f6c78eeb9c1c594cd39901fac3ec17786f242895e2af9a3441210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff34860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688acada6ccdf4b285a406e7478e2d1e64b64712863b41458c3c188d90ed73551e685010000006a47304402203691681a8203ad30d6897765360a3813f32925bbfe3060c7dd5f8fc1dd5851c002206fa1a6e9ea40aa4334fccc043d7f686a655bcc7924e446de136b950772cff31e41210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff39860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac02f8240100000000001976a9148e55e19306521b1925853d5df0f8fbc3e155b87388acb6380000000000001976a914fd0d93743a6233bce87968f581bca17c09f2ad2b88ac00000000",
		"0a2aa3d468013b65208223f3c4a2bbad6137eb2ca4b843af3ea015f09e36abb3": "010000000000000000ef04f9ccbcc9b3b248872295edd4f463a66028a3a82d2b0eaf355f2156d56f3bbc1e010000006a47304402206b9252446c0c40beadf46fdbbe48074cf15f3d69f88601328dff70cac84328250220050256df5e8913ed9b781fcc2d050b9a064878cc65f9770615539a1bd18cfa65412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffff4b1e0000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac2b7990d27b100320b7c6a51bafd815fd21414fb1db038b0e63d35a3cc8147868020000006a4730440220764e985b60e03e932e6d68970a70346d08d80d47ec1176589efa0d47aaf87d6102204b6fb508cb62432c43a06d212a84ba3b60d7a8aefb99ffac4f00b39bdea2447041210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff164e0000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac301da8fca1b2333f315f4bbfbd227dce5d6dc9e8c548a50a5d62f0fa250dc47c000000006a47304402201ff1233583048e0bb5ef8f7676fe0d776d774036ef69ea0c0096cf5e6173d58f0220178b92c801c7105b786cb165f0c61b8e91b1a35ac1c984668587e4babfcd0f1d412102ed226031b4e89b348bc641ce07c9589f3fc87d7bc9ea75a0d93edcf3370260d9ffffffffe4570000000000001976a9142b6bdd30e74ce66a48839237e8e92b6e79faa94888ac9c2e2c9dcd8bfe1c3d4252cfff22e1f18fd3ff6074efe94ec92fe49527cafdb5000000006a473044022005966cb1feced76e593a881b08e9771c42a017b13311e357e4bac8e00cd94bd602206f514d5941cc00528bb21dc258e6d0f164b60a23db46dc5ac8a3e27773f81717412102ed226031b4e89b348bc641ce07c9589f3fc87d7bc9ea75a0d93edcf3370260d9ffffffff7c920000000000001976a9142b6bdd30e74ce66a48839237e8e92b6e79faa94888ac02f8240100000000001976a914e8fc63df35216a12bb8bfafc1eda1f84e81b5c6688acc8310000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac00000000",
		"8e4236ac76a3f9863eea95ca0574b78225ad6e33e6db87513335af1ca2d84a7d": "010000000000000000ef01772bfff43d6fb6cdc37b84764fd7d4aa53444e1996c7d17851273eda178cc39b000000006b48304502210098bf21c173083f704f5fb1365321eb7bf97f3a3adaf52689b2435d5966d500ee0220674ecb9eee84b320513833d633233fad1a61761625312b3add7fa2c13bc3a59d412102cae0f4cbbb40f44bbf23beab05df20fba9b041f53d5faae3edd30211fac0f4f8fffffffff8240100000000001976a9148e55e19306521b1925853d5df0f8fbc3e155b87388ac027c920000000000001976a9147b4271ce41833ac273ce8167859bf7fa168492e888ac7b920000000000001976a9144cbce37df1fe2e2692010b47c2f42dcd8069b5b888ac00000000",
		"88c7b4f34fe57762c2f4897f40d371460c10c8cce0a88ba45fd13ece5f7ee73e": "010000000000000000ef0136a6e3c2d1822ce4f716a4232f3e1e58e575497cd04bdb0426b34d2de5f56950000000006a473044022010403b51e0c5b4335fb28483b0e3805eda7fdbd6eb53fede4e901c2db949f6c30220446bbf6a5d68d904f267f6ac93165d1fe7ad4dc3f8a47976b74f33efc157c80141210207f8079c8aea3197bdb0c3cc55757076931461853b5973a20bea9c68457d2457fffffffff8240100000000001976a914b9a45a7ebbbbebf93bc9fe9e7eb36daf5fc277ec88ac04204e0000000000001976a914feb9fdc7373775c8394fe776653090cafc09d82f88ac10270000000000001976a9143cc9d7e15b7b5ac3daae45dcd146deca380da95088acb80b0000000000001976a9143f4ffc7d7b1903b01b521d2f673cedca4f593a3588ac0fa40000000000001976a914158ef65fec41d6c7bbcfc09916ea594767c2dfcb88ac00000000",
		"b3c5a7048a4d149666766d8281d3d54b347ae7802a1a9941a0797d673300d5fd": "010000000000000000ef015339f0fa4937e7282faa88bafcf5dcccdc81ccef80066b279427b78f7d4c73a7010000006a47304402203023167b537c11a9c9394027e23b63d93b45ebf65b0fb88c4417e8c9ce77f33d022061d0401462f98a77a0cd54a30526d7c5fbda5dd79e079a19899dbc324936128541210360a71004d9304e841139ef4fd0199cd6ca4b23982e7e22cc26790eaf41f9b1b2fffffffff9870500000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac03f8240100000000001976a9141c968e8ca2f47871b8544569e584eaf6d35a0e7488ac76310200000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac8a310200000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac00000000",
		"22189f8802cd9a1dc1d20e33a2ac246ed1a475ac9cce2bfade252086b904df84": "010000000000000000ef044e3074dd9ea797e34c6980eaf6bab604f22f155cee77ecaebd5b5467f1d9da16010000006b483045022100fa2b9f04e20c71f8cb520ddb89126ce32226f76c33acdc790089dc629e7cfe13022062a625ee967bacd8ccf49fc4e7f88d3b079baa875f78aa00d057a49a2d0f27ec412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffff8d3a0000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac4e3074dd9ea797e34c6980eaf6bab604f22f155cee77ecaebd5b5467f1d9da16020000006b483045022100fb455c0f605771f0518b4eea4b8a5ad45e1a39c4d3f15162473b6befbfa96e1b02201665d3b47b01745cbaef5c7686ef6c6f10588c213beb03e1e94e6e606583b45c412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffffa23a0000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988acfb17e9ec6c038b8be519bde4531c114e504912063ff79838576ec0435ebe238b010000006b483045022100e60c2e5378cd80d3abcb65d0af503784abb1f203ee1a5b3a25b111e398199bb902202b6db7544b3c2aa4f4208ab68ca1e3d4a79a34d49ad11dc6e46e7648ae8b878b4121023802ceada489c20955b4f08b9aceb6f1a96798bb1862cb0b08fba35176dfbb9dffffffff088e0000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88acfb17e9ec6c038b8be519bde4531c114e504912063ff79838576ec0435ebe238b020000006b483045022100f9f0b6c5ea3ce7cffd668f806ad24a3069e833407153f6c678f9de0623d15058022043a54b4010603e3405b8ffda937b4cfc3fa77f9454e310c270e7458835b2514d4121023802ceada489c20955b4f08b9aceb6f1a96798bb1862cb0b08fba35176dfbb9dffffffff1d8e0000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac03f8240100000000001976a91423384a3a77ddeb844b3a966c201f21fb206a559988ac23360000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac38360000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac00000000",
		"75a0f90c2c910174f58ce85926fe2c295f7ffb262cd3b68cc8426f3a5c02d02d": "010000000000000000ef0484df04b9862025defa2bce9cac75a4d16e24aca2330ed2c11d9acd02889f1822010000006b483045022100f6e7c15234a19d62f67405bcd82add0364aa9b9fb30ca5cdc4acdad8f37550f602204965be300b66f39753ef9d371a72bbcf6d3867463a7754db6c3e7e7ab7107ccb41210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff23360000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac84df04b9862025defa2bce9cac75a4d16e24aca2330ed2c11d9acd02889f1822020000006b4830450221008e78169974801b94fce2c112477d040246b341661e8c5c09e295b8943b3c10e202207bb5eeb32af34fc8c521b355493dde6e7ba34fdd438db70d8b53da56bf6a89ba41210265a7cb93ac0b3088d18af114854ddeb25e31b41a71339c7e88ffc88c7d9ad78affffffff38360000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ace31f724cd3183cf519f642625fdecf22207760247f0b3e206f3063185b8fc375000000006a473044022019dbf2608b551116d584d23999683f7d6dac6b6c36223941aab767e5dd4baf9c02202bfc4461986d84955dda0833fe549cf8eca2bb876ef869c604f760ff648e1b32412102f964a3c0a4a6b4d0983c4e39717a65796b525341c7cf39b0883aeedcfafd9da3ffffffff7c920000000000001976a9149aae8823f12f553dc947ffc555fb57b35437298f88ac5172759b8fcdc9779c75ab7f7f07bb0c30b557027ab3228c3d579173e94ed9b7000000006b483045022100902be3c937ff582c1bb1f04f8dcfb257fe43bb2342e18e57ba5ee469f15441f6022067a6adf8c0a66d2e08ff1a9bbe1bc3dd4b77341aa0d569f4877cd9cbecbc23404121029e9cd1d7158a33121654f0f26a7ee4ebed75407c77cc6bafdeb010d89df78958fffffffff8240100000000001976a9147b4271ce41833ac273ce8167859bf7fa168492e888ac03f8240100000000001976a9144fc124260f302fb5a310b0cde52e47c551e5f03c88ac617f0000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac757f0000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac00000000",
		"a20f9076d0c4fddf932d45d2a1e841148e37bd668751e2843c9f0ba1f29fc0f0": "010000000000000000ef03b3ab369ef015a03eaf43b8a42ceb3761adbba2c4f3238220653b0168d4a32a0a010000006a473044022057e4da13d99b1c5af7f1b0a987e1683a26715ee9c13c359b29adecdf2bcc871a02206c716823fdedf4c3a6b0c18be673fd56fca3c6a3d0dac1b3a781500130cb9ae1412103bc36dcb85a107589b1558d77f721453156f7363e526b59c8d8387491e89cb421ffffffffc8310000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac856454778190083bd320a5a8fa0c97ff5136cde70b406b620ee8dda5c5599842000000006b4830450221008668c2c676617f11b379ea9a7983555f386e93029e843837d41143ea9e00b128022011f27c89cb47049be7419ef4a8d25bd44b7984b07566c42bbe28550968ccb9a6412102e2e1ba788a158ed79e1ec26f5049f37cd5e10d9fbad89d220550bdbbf6c91054ffffffff7c920000000000001976a914171af9ee46b2b9058cfb120be5a3ff636faa926a88ac05420483c28ce6be764a486b707a4c619d2c720ae7a9187d775fb6a50a1e0965000000006a4730440220320177ae16eca774c4b0adbf1cfb6db70177411d9c26a84e4a0c1bb0c72bfa3c0220081876d9f3148612bcb2919b7ef095342381467bec2ded8bc67ebcd8a51e5a004121024b470be5f22458027a7d9c132a2d0621c3d7f21a04f4f4e1484b98c98d766072ffffffff7c920000000000001976a914a439cefd0f4281a3099fca842aab4873a974b30688ac02f8240100000000001976a914b9a45a7ebbbbebf93bc9fe9e7eb36daf5fc277ec88acc7310000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac00000000",
		"85e65135d70ed988c1c35814b4632871644be6d1e278746e405a284bdfcca6ad": "010000000000000000ef0166969167daeaae7776316a0bf5bef12bebcc9f58094fa42b0963a2ed75ef61d6010000006b483045022100d06a87ba47b4dc1cf1448d11b22e45c8d493149b39a2d61de9b44d0e17f8713b0220133edb6eb7cf7bbeccd262b6d0daf64172f4204e52c5b8cf681f1fbf594820ee41210306bda1013d573dd868a14bd605a2deab8cdeb09edcb6bf6c0809052c0399ea1effffffff80310200000000001976a914fd0d93743a6233bce87968f581bca17c09f2ad2b88ac03f8240100000000001976a91445d17ab2fe29cc038a2688592e54567f93ec522f88ac39860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac4e860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac00000000",
		"f142c014bec539d75cd3887ad758f4e535e423c5dbed51a227af1c9ee43b13e1": "010000000000000000ef032dd0025c3a6f42c88cb6d32c26fb7f5f292cfe2659e88cf57401912c0cf9a075010000006b483045022100c901f8bdec3f683f234969758e5c01d740e06e812230c76248d75810bbc760b002204bf18c5f02bc022578beef0b7071e97942ba08dae1fdae859e6bafdef87c4111412103bc36dcb85a107589b1558d77f721453156f7363e526b59c8d8387491e89cb421ffffffff617f0000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac2dd0025c3a6f42c88cb6d32c26fb7f5f292cfe2659e88cf57401912c0cf9a075020000006a473044022031a935fcf4a9961ad9f704a72cf7d0139b9e874be53caeb65a8b0b233ff4077002203842acfd3c210d26fabb9486dbc2d11396f05966b20fed18f0fe2e18b1bf4cbb412103bc36dcb85a107589b1558d77f721453156f7363e526b59c8d8387491e89cb421ffffffff757f0000000000001976a9141d3c80e5da9da1d88db1717727f36aa0855850c388ac2b3d8ca57dfb2d65a32fcb01328133891e9d08debfcb8823be5acaea9b2d750e000000006b483045022100c1903e943488af6f5394b2fb81b33c1d1b64c52fa87097aa5b31704eaf79dbfd02201232074cbc3bedc1a148147976dd67f88b220c4ab2608c3feccd47703aa4084e412103cb455a9276eea85e54cebea783a6b46c16177dd750b48afad2e46d3aef0bb57dffffffff7c920000000000001976a914a21adc5b90591829d0a3d206c7ad23edc55d6cde88ac03f8240100000000001976a9144fc124260f302fb5a310b0cde52e47c551e5f03c88ac22360000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac37360000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac00000000",
		"a7734c7d8fb72794276b0680efcc81dcccdcf5fcba88aa2f28e73749faf03953": "010000000000000000ef0117b242e5f36422e55624693cf22eaaec4ca20f4cad9bf69df6f4c34208bfd290000000006b483045022100b7fdc4e9877f9ba1bfc39d3100441bf481f1ff8c0bbf2d0ebb3b21d4604fd7aa0220330b851bd085024ffb1707cd0bb7fcd4d0f45071f6b7850f3b64785c762a59134121024b470be5f22458027a7d9c132a2d0621c3d7f21a04f4f4e1484b98c98d766072ffffffff00350c00000000001976a914a439cefd0f4281a3099fca842aab4873a974b30688ac03f8240100000000001976a9144fc124260f302fb5a310b0cde52e47c551e5f03c88acf9870500000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac0e880500000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac00000000",
		"1565706d7d9e14a741e9b6a66d742c55ae662e75665b566b5975bed1d77d192f": "010000000000000000ef03f0c09ff2a10b9f3c84e2518766bd378e1441e8a1d2452d93dffdc4d076900fa2010000006a47304402203d7338184437e5e60ae19d3f1a10c1ac435cd58e123c34296513d4b5e4a5a16b02203767e0d87b26cb44067d4d762a43fa787fad72b1cd8ffe560c194ce854e5dfc9412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffffc7310000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988aca89bb4ffc530128d5151b8c83eb1982d6d2e2841d1b6772361cd439ca962371a000000006b483045022100e7cfbe350b73b98185cb5a564e00904ba22445ac60211ee64aeb73232315f23e02206cd522cbfbafb2c9f048efe74437b2f82166dd17359029c919361b1a294b87e44121029e9cd1d7158a33121654f0f26a7ee4ebed75407c77cc6bafdeb010d89df78958ffffffff7c920000000000001976a9147b4271ce41833ac273ce8167859bf7fa168492e888ac710f3f65dbca311620bb13ffa84f1473393436b9796cb2d5a21aae39a706d20e000000006b483045022100dac245d22a58bc1ba9b7bdbd2290ce8b22d368ee87bff2030d5c490b24b81d23022043554b51ff1c23a2c33bfdb844063db76fb4c14e3df02cc425a3ca9ccbeeea4d4121027b7ca9e7a9df1c7cb3db3f1c383b41606bafe546d933e978fec383b6e650ff4afffffffff8240100000000001976a914723494e7c601496c69a24ac2820423b269a3e4d988ac03f8240100000000001976a914232e04305a975c649e667cb4ba546d3db18791a288ac17620000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac2b620000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac00000000",
		"4d1ad990c338561dc039ec9a7d14b6634fd02dd192053612721843fb0a6b574d": "010000000000000000ef013ee77e5fce3ed15fa48ba8e0ccc8100c4671d3407f89f4c26277e54ff3b4c788030000006a47304402205192cb2480b3cc2a69bcc6ec90bcf0394542f55906fda1ba7046ab5c558802e002202241c75b453c2e998425ddd0f1617069c72f800b9eb7086f2e4034d01ad2ada2412103829143778745570f81ff5e5273ba25e1b22498ab19ca5bb5826a3b1616e215deffffffff0fa40000000000001976a914158ef65fec41d6c7bbcfc09916ea594767c2dfcb88ac0310270000000000001976a9147a0706bd7ff7e7df12597140a5a94412c212671c88ac87130000000000001976a91473bfff78d94375dde35c480c04866d5eb2d8eeaf88ac77690000000000001976a914b35e5c4f05976458be36d78d7a20149f51507e3e88ac00000000",
		"5fc747abbd9c4c11b45c98322bbf9f560aa6d5500e46f896d053b2b5bb4c6cf8": "010000000000000000ef04e1133be49e1caf27a251eddbc523e435e5f458d77a88d35cd739c5be14c042f1010000006b483045022100be7f201a9807e526dbf32e8c875a3a971e8610d631113153c3da34692651596e02205ae8b468f2c50054a003556986a411c709baca96d6c48735c59f191331c2c4b8412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffff22360000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ace1133be49e1caf27a251eddbc523e435e5f458d77a88d35cd739c5be14c042f1020000006a473044022026311ebd92493da4e83602239fbffec7ba118f0e966b6d62cf46cf4419bbe40e02207f15bfca6b989b4d8223fd851eba69d6e696f22daef19849364be7d75bc6cadd412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffff37360000000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988aca24b0738dfc9c8927956505158cc747222cb326b22c7cab02721ece5b9d9188a000000006a47304402201b1bea6cd41f8ea074a9ab7534ebb2d6ad4d6485f369fd5defd968e1bc84b774022040a1c2c7f6a1a259e5fb282aaf09185ff60397f66b5b735efd8b8a93df080f4b4121021db5f0cd13683015cb7305b6bb8c8ff27d7cf900b01d0d7e52ca175a785755daffffffffe4570000000000001976a91437dc45a3b18c5e08994aa540313f6a400b4b6ace88ac1e30ec2ffa98ad46078f5c52d130d5114b9d659328aca84e76514a4d0d4bea23000000006b483045022100ad8d239f75ee194a48dc7b4ec92b7c27fd04c84840f402e12f14d0f22d03e05802206f3e0b950d7a510881c023a24045a2527b7235f2f5d24371d423cdc5d5106e724121027b7ca9e7a9df1c7cb3db3f1c383b41606bafe546d933e978fec383b6e650ff4afffffffff8240100000000001976a914723494e7c601496c69a24ac2820423b269a3e4d988ac03f8240100000000001976a914e8fc63df35216a12bb8bfafc1eda1f84e81b5c6688ac14620000000000001976a914f8092f50c4bf16cded1616431837d4c9ed12504088ac28620000000000001976a914f8092f50c4bf16cded1616431837d4c9ed12504088ac00000000",
		"d661ef75eda263092ba44f09589fcceb2bf1bef50b6a317677aeeada67919666": "010000000000000000ef015339f0fa4937e7282faa88bafcf5dcccdc81ccef80066b279427b78f7d4c73a7020000006a4730440220381e030b57776f1a36264fa8b55829591fc32bcd48be7a5b43a00fe96217504c0220639a0e6bfbd9ef44452ce2b81c8b67b949cd7b62907b8e7c899f46b120990a7441210360a71004d9304e841139ef4fd0199cd6ca4b23982e7e22cc26790eaf41f9b1b2ffffffff0e880500000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac03f8240100000000001976a9148cdef63b4af3b5655f77329e3f25c3f0079d0f1288ac80310200000000001976a914fd0d93743a6233bce87968f581bca17c09f2ad2b88ac95310200000000001976a914fd0d93743a6233bce87968f581bca17c09f2ad2b88ac00000000",
		"90d2bf0842c3f4f69df69bad4c0fa24cecaa2ef23c692456e52264f3e542b217": "010000000000000000ef4e1df72e17b9161f1bfb235dd43bab443b117961fe8bf9d89955ef68724a5193f8030000006b483045022100f685e67bc70c6e9dc5dbec46ed9f1e5aac8c3fbe9f660a44220e315d912d0cdc022022f0aac978dde20164d81aa005082d3edef06b81a7c0c313d9ca72ca9acd182a412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffffbde30000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac0c7aff6deae380fc93f5cf9291d4028112c9d1682078a5dda4b95ac152386689020000006b483045022100b15598ba1892a19b95313b3f0128b7b284952e1cd82a47a3027d1730241986630220331daf338250899828628d5b0eec765b9148b169001ec58bca70e138a76a1a84412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffffbce30000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acebf3fa61c0e2aca81dbe1f202c217e0c5b928c71ded970b0283e01a6a56b3084020000006b483045022100a0a8824388b2b10335a0a83bd76421c10af877882f437748acfd40cb7791125202204b6fdabf79f9586e287b25ae0ee852da3e1a5ca1d4e945d7741b2ce2df90f838412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff6e620000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acb8393deced2e4082e58b54f39bc13db74a978a1a8b02b7b5edd2bdb887ea9987030000006a473044022039ee25ca67db9f2e3aaf960a2a64ee3b8d33a50e656ea6957f2d761c3eba03fe022017cfe95e69d82ac7cfcc9be5e87e615114c02693e782865fa63e255ead91e9fd412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffffd9600000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac9d69f48312f0b7de453c54801e7e297dc1d533d2671426bacae47c73581925dd000000006b4830450221008c736459f8151d34960c64140b7852b74353f38ed84321e1bedc88fbaa4f8849022024dda6977f227e959cceb730b3e4fbe7c73d8e576dae1ef0c62e3c2f816d434c412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff68420000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac8441151afef81e8301783645df49569c4f3e3ddcca4667116f1993c0adf40d2f030000006a47304402200567a44e381d28e414e5b8d89209783d81804bfed6a03888b3c8763011ef62ff022030d407b93e03a72a6f061118ab897269c5429475df9562b75e3336c32e85067e412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff03420000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acba6184676c72dda9deee15c0696784660b14ba7e079373a53cc3fd46c3561a2d030000006b483045022100def279122de7622a82aa018a6e13596f862966dfa46e790370f4029422ebaf590220018a47725555a5941ab672eb4fd688131b65ab391f9628e306320024b4c06531412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff21240000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acb54d055c1158c66de235b5766e01ca6309d606f82cc013a80e1464afecebfa35020000006a47304402202eddfdbe0c676f7b99d46fff5bd2c58f8c2b1377c4f4b24623f7e8a56c69b83c022038bb48bc8dad36a4a069fbf60ce829edbb67b0ba51d201394630c0c5b946f169412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff21240000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac6822622592477bfb8866cba3dfa15ce16f049d8c56c7fd715e9d290ac697f0ce030000006a473044022036e5a3222441f0357c9ea4d4ef5fa9b88a3447485519cba9e27151bc9a274808022032b8d4e5459c91dce95a936239b5d41f09a6a7a9d55ac09bbc8de56506e74d89412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff21240000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac7a1d9e5d508dff3e362b8e3b31df4f3777afbec37956cc956d23cdbc31c3c76c030000006b483045022100a510607229a87c06598f541ac2caf51fa799f84231ec541a0fe937ff5c0303a50220133357211b20ece1f11bde6045b8399b7558f3b075f146a95952335f86774b5b412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff21240000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac558c519cac9000756e37f867f69bebc43c052564ba7aec331eef4997578dc171000000006a4730440220570ec5f6ab8779c872c5447b72f78a4757bedb59d2169583e1a13b23610aba0302203f4626014a21288145d1c77b8f883c67e5c443f7d0a1cf8ab5616b553b7413b1412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac70bdc5920c96e9fadd1a816f7697ace9c1b00584c1357f2a778dc1a9ba91f171000000006b483045022100c623812c4d42c4e3b5cea666c3038b703d8d0041f69041c283c85a9b03adcbe5022063d251032a70e15de6eab05511e2d9dcdf73959ee4d998e208b30f92685c9793412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac7b68545075031eb96a09868f75cdc12040bb19b1d19fb60870bcdfaaa48b7ad5000000006a473044022028ba84ef58a4d37fd52e947f64351e1c4ff99d876165754d81177527dbe3254a02206c4d82b96f882784f020eff269b8e50cce0ff4c7776f80d1248eb4091b5f0700412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac770b10a661cdc0ee64a95b45dd76286d4908bfd25ba654d9bbd6fd2f289a1142000000006a47304402200c3e950526e99e46a0abf24e40e3e1b356a62fd9fdb421177cc0f3625af949ec02205ee78a58b57cdc0a6fc6b401a48d6d2b8f3b4ab62b1410ddfe51e1211132c973412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac0401e3cf89892723ebcb18ea7b9be91f55e094cb4607ec494543088b7eb58f64000000006a473044022069cf132c4cfe9507b064aadd7623a17e7e8f913fca5fd467837ccb52ca235f9502204ecef3881336d2e43d2221c6b1217b32cf387865471fae7025eff7fcae87c0bf412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac441850f2ccff848c5f13f32a61c3f8b2b880a5bcdfc6c53b21b2b93790ca03d1000000006b483045022100c82c81cb540af860373eaba390a04bce4569ea8e99b8b3659f661960382e021a02203ae6a4828f717ab679760968d31d1ccacba49a0d74de3d04fc5cccda2696aa11412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac2f9b846a2b609cd676a5af3a639a3d72ff28a1775e98fd0602137fc0c5988ef6000000006b483045022100a0c6a53715ea8864e58a111190cce33a9cb9721b18bc7bf3c271417ecda8906b02205fad0f6f81bcce830fb9a359146e37fe286239884a6afdf5e27d8e109fb7eed4412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac45763ca4929936da18910bd25e2003f0ab33b554bafd005ca0e7f07117a543d4000000006b48304502210090aefce19fd9c5d5004192e84acc627c3b7db183347c6c461c00ddc8a1ad5a9a02207cd2da07634aed840eea8fb752a06d00e60410c970ab03bf6caeefc7a65cb6cf412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac614da0169a2b0d16c0a192427f36154cdf420521c874d5ad93ced85694d334cd000000006b483045022100908099aea47421dd6fae57d23cbd3d91811260d22223b5e0859c2a7c45341cde02203852f0e32463def3e2579f8193ee4e4ab40c723d5a3caad13a91482a171f7490412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ace0e98a54a9da86260c6db2b4d2ea807a7edeccb7d4ee0014dae73e59d93333e6000000006b4830450221008ffdb6898aaddabc3efca0d7b033a299a85ab6ebcd4ffeb889d754e79839889b02202458e798b89d5d63dfbd7f0dadde7fe36fb96b0bd7d4888831154bda0fc5a0e5412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac7edba0188f5b9a969c38a46bb9665e74d4bc0b8ebd0f205b98fa3cc18af49111000000006a4730440220116f3b7684598f437d2f6ee1d2d19f05663e607e43d7cf9324bf2a9099c903b802201e452c8deb62f70a5101cdfbe19df68caa99cc7716e05088aac3849934250d64412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac99c17dccaf4721e41c968d670cc6af96463dac2242ef457a5ff40c988fd041ce000000006b4830450221008247c38d0c7b1329cf7feb18a08837830075f9a2a36bd004e35af0adeb11282e022074ce70606dd7d3344c9a1ea22775327c27c6deff2aae2911209dd1e378b7203f412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac174a02e8b00d0c042eec3e005fdd6c3ce0a343a5055f137213d03446d5ce40f1000000006b483045022100dd8be5e536e68513538e52b0b5d0f6e759eb8725da1e33a1434f17dde6f7a6fd02205cb69840144b4f1f0ab1215acac3a7c7c033fd7d2167867f5622e3b4e49eda57412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac53cb3b2a4dfdde5d8fa242ef3352efa7365b3f9d34a932961a433c7a1e4b9a42000000006b483045022100c242777ceb6d105bf4d0d9c7ecfb1007c32129a342198a891e5498c69e83ea0202205f4409976529080bf66485d3a756d347b7488ce6e717ac39f24adbbfee22a17f412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac8fb08f51ade051cb827bcd61ae21bb3deebee9bf709a9771b4a1bf263925277c000000006a47304402204ecde81796780fd7a9e0572c9c2237daacbf64b5b1e762547f8aa45bf8cf88f502201f8d1b1114892f81b0832679ee15bf8a35225ed7633a5fbe7be172165c7f7619412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac522027c823b6e6e5112c3c08c0cdc32621589a7d331a539006bd2aeffab1a8e0000000006a4730440220122383dc31bdbdd2a6e8d103557baf44181682712d61de50e19e9cab8fa4e42902200af77eff16068681d8123305c1000d4516dd5a605564c48487241622f03feb79412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acf710d01f10e819e590fb520c134a2e265866bd4269730110fb8a76d918a1c317000000006b483045022100bf654b7396912cf82a6aa889815f230656b6e987ef0531eff2fc576af21f3e36022032dc8aa4f00e7bc89a8ec9b810b003f9ef6bfcb503f65a36a7ab5ff21494cc37412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acede821cda88b024be5056a3b40240c9e29aaeaaa29b878c9a7265b75acee77e6000000006b483045022100e00761e0d14c5743d9855922f5e8ae3c2165d5be0f27964147e8d837977dbe7f022014512bedd10066a7893270e2b7018eb8146c788e214eac54d40dc1e09eb7f4d2412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac08fba02e6c6ab79c8651c3545da51a1314dfba8bf3831b06688e49a72de1f021000000006b4830450221008b75df156d25fa56dce2943404d40db9c8ee3554fc1118c0bed9fbd848408c2002203471ee1a4f31d9ebe04cea1c879358d355e1bb4f83df2135ce9807922c768bd6412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ace9d2dcd87244fb3808ed297ae7d3e78fdc8bd3a6cb1c1f6004bade9c88bc6bd2000000006b483045022100d64c6003c4c46ef2af64235f5993ec9e19c19e09b460260e1d40c2187514fc7a022036e6779d294bd2111309fa616f60a2485a42a7b8716335690a09888788ebb782412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acfb9111faa3ef29072cd3b5f7d586f5b86377dcf8a3ac338d37b059ba085a8f15000000006b483045022100ef61759490d03388f919d475ef0fd57642ed7f17fabab4de7d7233087825aa5a02202597bd06e37b8d97c9a7b6aaf9d10e96355d200215689e26de923200b206bcbb412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac1c569c958f19d35e0593a1fc63a44fe159274924951171b3d5cfb8bf377a489f000000006a47304402202ac0c3f362c2e2dd321f24692690bf2afe46fabdb74cd99eef411eac2530b28902204b3cb86ff61c85554050272c8aae23fb603fc28fb4e3739f27baf5459b667c32412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acdbda83ad8b0647b521f2e7b6c8b931631d78f486a5604f9aa9888fcb1f670237000000006b483045022100a392d9af12db7c97d7493ef5acc215aec3d8bd649985b2d9e33107b96e904a3c022031434c5dba4669c1baa32332df0e6fab38e34bfb04f32faeac5117c1b9c7589d412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac6265c0950b6d39c088a31dccac354f516b9d82eea052c75fe9409bdc56646e49000000006a47304402201a1ddd0dd57a6e8940a516d84d96ee98c288102b30aebd2419b95c3172f608df02201647240845d91fc85da31b78f5e3e9ed74fbf13c94ef2295a2ae857b0eede691412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac8739163b89b68206a985deb3b326cf3f62fc66f897b2f7ffb89d93e80e3a118b000000006b483045022100eaf8cf047cd8eafc0cd7b3d699231ac37b0d332ce9f44482fe100479d1b1fcdf02207db949c78a3946285d01845dac469d260e4eb804267438a30f8eb76861389421412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac13fdf4783d6a5b341c83480ddefac1d3440c84aa77f104e3c325031c9b7ae836000000006b4830450221009ec51244204c0b72e3bf1da908a9a815adcbac8e3cf20ba5ca42ef9efee997690220033aac98cc086482d46244b9a181fa32a5559e4640efb092298d2a617104cb0a412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac5cf3e43eeefc86a0cbcc634a904b9529d8d2b05e2e6ac1599fdd993b695be1cc000000006b483045022100f67b826329381808d1749907a38b85a73a34e70f2bef2c8e45e6fa80103f29f502202079796c04c53321ee657d1e186b3604080fcb67ecc4936011105f282441c0b5412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac8c51768c6ba0fc7c3986b63bb5b16c343aaa2f0c21d6c1eef652c37509c94530000000006b483045022100c0674c67a246250e1631c5e421c6ca983f52f8275b542bef58bda7de9d53f7f102205d84f5d6fb2a329a34531bbd59bfe014b22fa2c13ace064dc79a567772f48688412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acd606b541bb9808c083494ced1b7f145eb5ed1cc9b07787238668284e1a4fbd23000000006b483045022100aa85b11c787ba470c91f868301c8df6c16c92ffa0dddae3f712f4fa63906633b02202ac4eb3562e581ff40e163a74f00459c79d1485f373e35028bf01cb1e636b490412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac515830b6998b53b07ed9c8730d006d1d9f57cfb24d2d3f7640583a5a19360618000000006a473044022035f1e9bd1956d33e166934ca3d80c727e986003533ee8e5b6a3f68715730b995022024ab5b93ec7850f8c9e2485b6cfa13f4fe8ef928fc4ba8ec70220bf3cfda2070412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac81fd885a4f035cf613b117788f9b633bdd60fe9b4132ef99a34f1a177b72e362000000006a47304402207538eeb4fea8e11529f87e0852ddc648b3a4b9e5802a5a5450b06d948ab5a093022042a9af2502439ecdf97765b50b6e40654f24613ac42e8a03af2af73fa9a6fe1d412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac555d0c599cb59ec0dbbf168c3eccb7e1f457dc3efa5f5a9bb7073c549b4080ba000000006a47304402202966b8a01c64a2a1d42728299337d1201250d26a731ff9dcd3fe5ae6cd0d5ad7022035e5f5c5c10f5d62f188079904669b0e869f0aef8067b8d0c04568ee5bd8e72f412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac221cf09efd3edd0787779f95c7a32a53a97a19c480c9059ecfa6cb0379b3fc75000000006a4730440220528ea8cba32d1b4e1dc69179a98376f1854d0bb0e94ad4e533f20d760eb4e415022004b3a41c59724c687fbc03a08d9902817d28d71686ff018322616931593f2c3a412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acd052ab814996daa7e0d680091e3cfabb283eb2d3891df9f6ed21b35acfd17979000000006a47304402207d375311441ce5c54b6752ea76d327626b221d804bf06b869e9da9160084749e0220755a131cc91e6cac811c6db295eb8edadf6b93a822e1c2f563fb89c32fbb83e6412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac6e2be432fdf776e5ab56d295ef55438a8fab33228e00fb0669c636f7ba398ea7000000006a473044022060c7d92e360fded6c5e9a94baaadb77a9eafc9d5797a3d5d7d0a09fe41c52b97022061c08dcddad5a55c254a15da7e73b1b5b80c91f8bf741663fdfb5d0edab85f0a412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88accd83b05ee10d57a7d54c4236aeca3acb0aae826acb50b97e3f50cc460a8e452a000000006b483045022100af1c05127e948931c6866a2db8a56686df538336771109812a5d2271630a61b602206450bc55416bd462378f30c1d994edb3ed20b5008e852523b3599562acfad66d412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac5a1a5ef723cbd330d000f92aaa3009bc1b9b2212aaa8d2cd3c8d724405786c39000000006b483045022100de08d644af7b00fe54a762c7ede6ce4e078589e5fadf70824bb2a48b246cb58d02204b512820f3c2aeb02499f376d14bf12ced2079fcfe4965f83f8f78b54aa99e00412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acc4e43bef5b5d9ef7b74681ae46388dd5f064c572ae1cca0050965d9bd0c0afed000000006b4830450221009b2feba6fb59fd8a49184e78565c5f9d1c636598e3a527a7e213db006017402202206f4c24d9ec4e03c484c94d41887f29d399d0dac069cf1070ea769efc3f43cb0b412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac3c0685ce5ca4de5318e5f98d5332842a7df7f7af2da711b2776ce8c858686a94000000006b4830450221008de0004379c8a6aa8ce493cf2ccd8ce288da979eb2062a33dd44cba63dfd63520220285740047e58ccaa19716a42efab9b49b96557da3d98f3165746e5603c54d0bb412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac731b86691425682feabbffb0ad1bb289410569f800ac157b4272dcffb3ebd4b2000000006a473044022014edf978ce57ad563f226482f391b2960ae7c375a174f78e89e3f1fb0bc4e7650220371087ac56ea277a226a91bd0f7c9f24cd89c6a28164e1a34089bc0843531956412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac6a81233a06e007fa9c364a2f76ffbbbf9d547a19af5b10d098b793ed1d07f647000000006a473044022014f720be01648bd4f77672afea6872b25fee8e54791ba326c7564a8f45de297702200e66e023ba12381bef0d35c057df7f093c0475833a39cde70a52709731e7e581412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac4b60daa3edb4c338d11a378062929a1da273a23c3172620abf5764a778c4c731000000006a47304402200bd6c332b1b52876477b14e4890287639669de8434f40df3f9a9904c7e87571d02204f10abb56e5e9bd379c0f2e8f9507ad280b0fd7cddb10d352abda66f41234600412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acdd09b6d92a0e0e215d45c8b64df230b34de43d0493a60a2f0bd22f4f8fd154bd000000006b483045022100e03de4702b506a573743d4f157e571dae3f64c7a8660cbd2a57cce55673579b6022003c0319391a37c7e274d5c989dcd053992c01d90caf12199585c7ceb5837dc89412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac1063c34a057af5f74da082e17a91207dcf4ef9a6f592970357929f7c91ad459f000000006a47304402204793186de5659abc7ec60394213a42d9847d6d859a3e6b03b33e532fdfc41fa202203daffcdd01e780dfbd5d204f11f264a3c972ae9b40ece446378c293e179f7ece412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac25d54ba6ef7af09d803e118b63e2d585269a3766392a727af2e35248e462f07c000000006b483045022100ad63da4e77794d0fd42591aca97659a1339ef603925887042e183851323c8f4a022013f3895c5a5afad22def682b6a30c1c53ac221ce2e78c6c708acfcafd767bc30412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac298b7422a1eab30ac6191bc7e1a1c3fc3fa308a14a9b32899a4a3f8ceca55ee1000000006a473044022017b3f1a511fb56fbe33226560b145f35d71ee4bafa6f1be6266f1acb14855fd0022063e38b86a8d3986850de0df9d9b669c75bea06f9b58c577ea488a1496d50bbac412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acbb10e52b078e3ca00603c70769766c8e8851943c127ddb65667c48acee859275000000006b483045022100d0e90788c3ff9b6ebbc356c197e00032190c18eb5773f7bc7c37fd81cb0cd28802201da1c0cfd58fd3c31357f1a6e81c9c531fb3bbdf22c6023ff2941bbfc6d2c67d412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acc181ff3a92f60331760bcfeb873a94d7c090780f5c6d72c310db0282562b9f1f000000006a4730440220720657346fba0c5a58b8cfa0cbc2db16b7a812754cef954789968d43c5db97d9022041754e51345c17476252835d8f229e5e0911fcf7e0f3d992d992d09623c9a1c8412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acdb4cc0374225db49fd587ac34cf59648196ba0c94750beb736d7be89fb12d7b8000000006b483045022100f9a9f81f033ed0a073c90aaf682d69a6bb07eb9ffcfb7ecb6da0e3a9f5bcc9610220161950de9a584fa55a2914974d55056f734eca8239c572025959440cdc46a415412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac9bc030542f2ddb2e7d67103c84957339796a3f27db324667cefb6fa1d2a38e0f000000006b483045022100d2ac77f87d75be2f333d5d10e4fa7bf9cb579f3961ca66322403a605aa1b794c02206bb5faa8186237c1b926959154dd84499031927a54962cddbbf5e2fd1c3a9353412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acae56da734549c7642501cb35e928c7af3178fe31b4a218fdd60dd416e5bbc094000000006a473044022069e08d655c0fe7b9a55a057ecc6bc62c785b9d8f2e41489093ad9e2ff0a5f03b0220540bc92d948d71e47aedf100d6a66f95fa5257dd09a177a4c64190120a15f651412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acc474c1fd9e0838232529f5257e29d576435fc92a4ced11d90940642582248049000000006b48304502210094baee179a14b9c6938a13497a515696f4661343374d79c2f51accc02c5737bb02201b7cfd2a985e2f7b7c204112eb1178cacb458d79db2848d13fdb71b666dd0259412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac3636c2211fa06a28b0a8692d715fe6b383bbd26659f55d2af12d27b722643770000000006a47304402202b72f0cf09fdb230866e2888e4a5ef08bdc3b83ec78b319d1128baeb3a4db33c02205c8834fe54c1f3d8f45c354f89153be7f3e391f367c0b0d1dc825e50c390eec8412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ace0e561985d865270d4b2babc7d4a4a07c2e7b3cbcdaaa79047b4ef246c167c92000000006a473044022043565df57fbda39b82e29829f4d0d90a76bed06e62aec1495ae98009e734f4ec02204b62b8bfac8a4d5a7a01a85f42e13835d881b3c9f6e55dfb306b9ac569fc0110412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88aceee476302bb70564448adae4dc84d16928a8bcce649b10e71f63ac4de89cc0c4000000006b483045022100af2aba9adfc360b96a47891d78da5663e4aecf046a3aff085aa996afa00fffaa022007b36d9b93156063cb081130e9f8054b17d9383b808c5481cd1e10ea1f90c8dc412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac61cf849f86a89e9488133a70c6690abd94579ed272493d96379a7b469d7d37f8000000006b48304502210083dfb71365eacb192fa8646fbad98154eb796febbb33ba656a22de4fd1a98cbd02203e8f32437837a71092cc8cf747a484a140d1a2a037d67201e8812cf1732dc6f3412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac170c40f8946ac4192857f52de7904d7bffceb56f78ccedebed79a6cb4601bc2d000000006a4730440220733673348bb01a6930a6324dfc00d0d5d3591964e9a43b7528fbf44a466a642b022046ddc2fc87147437f0026486a05c3982ac024b3ccddfe14ae397636b8a475c30412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acf29fb3b999b20e4fdd4d5378f953a1b0b041bd3c89287912a625ea126fa835df000000006b48304502210080c1637ccd633c078ce2e0875f5d5654ea1ea8be2cbcfe0a1e8166b243c826e402205d408066c7c8863cd7e3ca5e1d5acf4b1f77b8740859ebfbab3906ff49716ffc412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac91e11b384034bd5616a3ad70890bdec9124e7d4327ef4798728afc7cdeb6245e000000006b48304502210092817c13b1fd92a3c80f17d1742876dbd1d3eea66f1a8a3e3447791822ba9b3c02206b06c39aa9a2259a3df1f46b0895a03cc7d738fc902e5e106c8d7af4a5ba2b9b412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac00a6daa53b7c3fe14f7f01f92aff7fdbfe456fa78293f48f3cf699ca01aec539000000006b48304502210089ad3e745a0549f45457dc166bf58584969140b267b4da078937530a7ceaa9ee02201d1269930ab278f23717d0ce8ea09d66250db7636d3b476817414be4cc71af46412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac509c3d95d1f97605c4e55cf92499b865c887496cf842955e67feec05414c02e6000000006a47304402207e35086d73dbcbc81c6c863019534c1882129bfeec49b3cae3989afbfae5954f02203e2d03e4d2408f65da0dd6a75d757a3eac9aa13aca84d4edfa6fb1844551f568412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acad2da4164431ece5a22655dbb29f32f6a13a0d12fc1cb4b9aee7da500b0ebb49000000006a4730440220613d9fb52e6f1c18b058b4ad6179db83614de99d42701becb193d8ef9f6afcd3022075003c8d65890c183367ce4601e6299830316964235dbf5ce51a9990882d0c6a412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac3c7fded8a455238d3faba9fca4fe7962007a79ba60eae78a0c9a92d22a84af76000000006b483045022100cca8ceaa4c7737b2b4536825e36166ee50a043f990534df4871739011b3cb93402207ee5f6e228247719ebc17237fb0e00131895b9af6a4b53583a9f516db5da52f6412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acc01082a223062ccc40253ab750c8ade6070118230bf4bb25757bf7d4147bd0eb000000006b483045022100ce34837626c0a172580f1c586606b228754550b3a937f44d449aeb5d8abdec9f0220314cf68d92a4991c7b2e91f89245d5f41809618aa1dea71794f3bc2d8b928fe6412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff34210000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88accb93833d8859780780fb5a7cc4188c418beeb7a322f9cc5122f669355e628b7f020000006b483045022100c5f4fe4836848fafbef9e96a9b4288c05951afc01a246621ce451d8c04142a0b02205dba18de1fc811e6c20c122601d2ce4c429280300d980b7cf2556e4f56063a76412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff2f160000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac433492ed1054d3983c0cbbb03779ea2bae22d02bbe320c5c4e6eeb709172ea01020000006a4730440220275ec84781dd4be11eebfcdc634150468cdc07f50702bb39baa3fc9a1102b9cf0220457f894c40532b767cec3cca110b0594b7fa153e2605cac2449aeec980d7e420412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff28160000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88acfe2d0603280298aeeecc841de1b3547117ad9cadea1e85bb14a07f42cbff947c030000006a47304402202ab4cd167ef9ea88d992acf50f96750623914220eee3c9fc7c12d200744ac85b0220778cb6def9c68fc6ee7686459793e4baf03a4f6d48ec39f59f8dff799835efac412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffffcb150000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac9dc6873756877f8f9de5fafbff402cbfade648f0efd6292b09d85a6bbff23840030000006a47304402204205aec4ec32864e2f0790a6efd4c90c5988f8b06ad197d200e5d3ad656eae6f02207d52cd2816f71dd35c33d24b7f383a5e3b99ebada32b323170743eb3a57b00b0412103f89e5f9bbe25ea0796477b486a8ec6aff36baa3a34610c2ca6fdfe78d31a73deffffffff87130000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac0200350c00000000001976a914a439cefd0f4281a3099fca842aab4873a974b30688ac570d0000000000001976a9146d6949541ec10e77eb496d4ee817195df77a2f2e88ac00000000",
		"9599ffe15e03b10365393ba44df23244f18369f11e227b926b6967e36f7dad2b": "010000000000000000ef01fdd50033677d79a041991a2a80e77a344bd5d381826d766696144d8a04a7c5b3010000006b483045022100d14f3f6dc3de3be915864a065184cc79a54e889e4aa264ac1d3407b1e56e7b8102200180584385ea25f7bfeb27ec7861f8f10c7b03edf3705391be0db7d4cdf03425412102a6449568536e8f31e5a3f3f22018ff58e0970ff26e8f9b3bc0fe58e391e51ed3ffffffff76310200000000001976a914caa9ec451498eaa6ff6322d36abc9ee1803793a988ac03f8240100000000001976a91423384a3a77ddeb844b3a966c201f21fb206a559988ac34860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac49860000000000001976a9146b84588f97d12dfd6873c0f09dc1d5df211cc5c688ac00000000",
		"5069f5e52d4db32604db4bd07c4975e5581e3e2f23a416f7e42c82d1c2e3a636": "010000000000000000ef03f86c4cbbb5b253d096f8460e50d5a60a569fbf2b32985cb4114c9cbdab47c75f010000006b483045022100d161201f60a1e12c05b92f732c02084bebe6920466b8341476c0f1e4811c53f1022027eebea994eefe5555699ef433354fb09d2d16c21824ff5e489117b288fd51bd4121033b00509617f37d19a3e4d0bb99889fdf29ed0e67f9654df06b30791287182747ffffffff14620000000000001976a914f8092f50c4bf16cded1616431837d4c9ed12504088acf86c4cbbb5b253d096f8460e50d5a60a569fbf2b32985cb4114c9cbdab47c75f020000006a473044022004e9229b2b3b76e7c9868d79df97c5989e5523f415c30a086171165694d51c730220532b72bff250d51e1dbba39c8241f77db32e64d5ddf52ea92fc765cac0522b734121033b00509617f37d19a3e4d0bb99889fdf29ed0e67f9654df06b30791287182747ffffffff28620000000000001976a914f8092f50c4bf16cded1616431837d4c9ed12504088ac28b966bc86417f5634a58b535a6c6d8c2e80643510df6db535fb04d102dd8b0a000000006b483045022100e911a5e8b999352bea09a0921c132c4a114f2fc8e05b455d93b209d8b9e45a090220323cf47a4d552743cb933d8d1f9d2586f14ed0c41f1ed165b6fbb9ffaf275d2d412103cb455a9276eea85e54cebea783a6b46c16177dd750b48afad2e46d3aef0bb57dfffffffff8240100000000001976a914a21adc5b90591829d0a3d206c7ad23edc55d6cde88ac03f8240100000000001976a914b9a45a7ebbbbebf93bc9fe9e7eb36daf5fc277ec88ac13620000000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac28620000000000001976a91429b35c8345fc59ec01b279bf0fd2d231dcc5267a88ac00000000",
		"8b23be5e43c06e573898f73f061249504e111c53e4bd19e58b8b036cece917fb": "010000000000000000ef0468f77fa544a2e21ef1b6fefe78e677df30f53c3b71317e3b9c12402a06d74a0a000000006a47304402207d1fddb3db90f27c8f94ec7fc4f04e4d5087d34d04621417a06d3208a54e90d6022007f45d1d1ae56e1978db5c2c27bba1ff9dbbd08cbda074db940a087739390f03412103cb455a9276eea85e54cebea783a6b46c16177dd750b48afad2e46d3aef0bb57dffffffffe4570000000000001976a914a21adc5b90591829d0a3d206c7ad23edc55d6cde88ac2f197dd7d1be75596b565b66752e66ae552c746da6b6e941a7149e7d6d706515010000006b4830450221009383063a08bde06cdf40ca56d583dfd9d238d7c16897ec4f2e450b088eb98741022003fa33add68c7665d6e3d2db0fbf4ec47cc5ea8ae314b63cc136bacf611480234121023802ceada489c20955b4f08b9aceb6f1a96798bb1862cb0b08fba35176dfbb9dffffffff17620000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac2f197dd7d1be75596b565b66752e66ae552c746da6b6e941a7149e7d6d706515020000006b483045022100d78384ecf8899e62e2108389d4ec8d11354dc7aaa3828ae40b89343f409da51902200f107685d50b5e5f6400acf85edd900a27643696bb949c42281eff641efccb6b4121023802ceada489c20955b4f08b9aceb6f1a96798bb1862cb0b08fba35176dfbb9dffffffff2b620000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac08c3e7e4840bbfeea9803b6117ccc1d64a50f182faa9d633e9d73e07accc533b000000006a473044022046010956fba3309d72d27f6277e57abb3a3f2218d666b4ceec41fa2ad56405a802203ee74082dec3d59f51eb8726e8cb84a888b51eabb5be98e640a80ead53eb30e0412103cb455a9276eea85e54cebea783a6b46c16177dd750b48afad2e46d3aef0bb57dfffffffff8240100000000001976a914a21adc5b90591829d0a3d206c7ad23edc55d6cde88ac03f8240100000000001976a914e8fc63df35216a12bb8bfafc1eda1f84e81b5c6688ac088e0000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac1d8e0000000000001976a91434c37666833faf4968fb5b4e0aef417e59bbd3ca88ac00000000",
	}
)
