package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiHandlerMocks "github.com/bitcoin-sv/arc/internal/api/handler/mocks"
	"github.com/bitcoin-sv/arc/internal/blocktx/blocktx_api"
	btxMocks "github.com/bitcoin-sv/arc/internal/blocktx/mocks"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	mtmMocks "github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/validator"
	defaultvalidator "github.com/bitcoin-sv/arc/internal/validator/default"
	"github.com/bitcoin-sv/arc/pkg/api"
)

var (
	contentTypes = []string{
		echo.MIMETextPlain,
		echo.MIMEApplicationJSON,
		echo.MIMEOctetStream,
	}
	validTx                   = "0100000001358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffff02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validTxBytes, _           = hex.DecodeString(validTx)
	validExtendedTx           = "010000000000000000ef01358eb38f1f910e76b33788ff9395a5d2af87721e950ebd3d60cf64bb43e77485010000006a47304402203be8a3ba74e7b770afa2addeff1bbc1eaeb0cedf6b4096c8eb7ec29f1278752602205dc1d1bedf2cab46096bb328463980679d4ce2126cdd6ed191d6224add9910884121021358f252895263cd7a85009fcc615b57393daf6f976662319f7d0c640e6189fcffffffffc70a0000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac02bf010000000000001976a91449f066fccf8d392ff6a0a33bc766c9f3436c038a88acfc080000000000001976a914a7dcbd14f83c564e0025a57f79b0b8b591331ae288ac00000000"
	validTxID                 = "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"
	validTxParentHex          = "0100000001fbbe01d83cb1f53a63ef91c0fce5750cbd8075efef5acd2ff229506a45ab832c010000006a473044022064be2f304950a87782b44e772390836aa613f40312a0df4993e9c5123d0c492d02202009b084b66a3da939fb7dc5d356043986539cac4071372d0a6481d5b5e418ca412103fc12a81e5213e30c7facc15581ac1acbf26a8612a3590ffb48045084b097d52cffffffff02bf010000000000001976a914c2ca67db517c0c972b9a6eb1181880ed3a528e3188acc70a0000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac00000000"
	validTxParentBytes, _     = hex.DecodeString(validTxParentHex)
	validBeef1MinexTx         = "0100beef01fedcab1900080205029237d9e6a0a2a60702a1468641d249f8698e22110a956c76b2ad5230663d270504009d2aa330ab7a0a2c4035f31dfafa7f001392abd3866c34e47769eb3058f7b1c2010300b84052a6642873d4be4c4bb0fc8bb547e4dcd11bb269f5682a55ae30bcd4d9e301000043b65fce7648a97b9ac4d3f3b1ab920c66e8c163b794f1348549adfbdd7b1ddf010100d7197b83df3e1307375627e98972cd420aaf6201b005de4feab5812adbb57c7d0101002a7d7056a39c2f5a481f15ec5c5d1e9cd6bf0a310ff83f46771aa89cfd892ac201010035d8031500a4ecd8fa4dcfcd64ca591db1460e4a6e2e8b54de424064aec1bb4c010100d07c1446d7366e537426cbdacf4234c3058c2925b87ab499171d6f621e15f829010100d662aed8bf98b6971d37fc9932fdb9e965ccfb53db61bb629118177e2995080f01010000000260e0757bb7c29c082655b7bfe619f4f760ce2dd990f4438f0061d0d4b9f7040b030000006a47304402206d71dede2de1c2eb79e47e65ca36138f2c79148afbc0dc0abc1973d0b5a302f1022058ff9b054630d7d9694e17f7eb662d6839029c37bdbea10e78af52add62ad6b54121036106caa396cae56b3483498a0f19c993f9262301874c5e994fbeb4ea6f08add4ffffffff9d2aa330ab7a0a2c4035f31dfafa7f001392abd3866c34e47769eb3058f7b1c20000000015145bcbcc1c8475c237b96c782a32b8f6c71358626fffffffff1501000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac01000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac0f000000000000001976a914aad19cbf4aa09f4f65e07d74a5aaf4ec0fddd84888ac000000000100"
	validBeef1MinexTxID       = "05273d663052adb2766c950a11228e69f849d2418646a10207a6a2a0e6d93792"
	validBeef                 = "0100beef01fe636d0c0007021400fe507c0c7aa754cef1f7889d5fd395cf1f785dd7de98eed895dbedfe4e5bc70d1502ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e010b00bc4ff395efd11719b277694cface5aa50d085a0bb81f613f70313acd28cf4557010400574b2d9142b8d28b61d88e3b2c3f44d858411356b49a28a4643b6d1a6a092a5201030051a05fc84d531b5d250c23f4f886f6812f9fe3f402d61607f977b4ecd2701c19010000fd781529d58fc2523cf396a7f25440b409857e7e221766c57214b1d38c7b481f01010062f542f45ea3660f86c013ced80534cb5fd4c19d66c56e7e8c5d4bf2d40acc5e010100b121e91836fd7cd5102b654e9f72f3cf6fdbfd0b161c53a9c54b12c841126331020100000001cd4e4cac3c7b56920d1e7655e7e260d31f29d9a388d04910f1bbd72304a79029010000006b483045022100e75279a205a547c445719420aa3138bf14743e3f42618e5f86a19bde14bb95f7022064777d34776b05d816daf1699493fcdf2ef5a5ab1ad710d9c97bfb5b8f7cef3641210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013e660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000001000100000001ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e000000006a47304402203a61a2e931612b4bda08d541cfb980885173b8dcf64a3471238ae7abcd368d6402204cbf24f04b9aa2256d8901f0ed97866603d2be8324c2bfb7a37bf8fc90edd5b441210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013c660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000000"
	validBeefBytes, _         = hex.DecodeString(validBeef)
	validBeefTxID             = "157428aee67d11123203735e4c540fa1bdab3b36d5882c6f8c5ff79f07d20d1c"
	invalidBeefNoBUMPIndex    = "0100beef01fe636d0c0007021400fe507c0c7aa754cef1f7889d5fd395cf1f785dd7de98eed895dbedfe4e5bc70d1502ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e010b00bc4ff395efd11719b277694cface5aa50d085a0bb81f613f70313acd28cf4557010400574b2d9142b8d28b61d88e3b2c3f44d858411356b49a28a4643b6d1a6a092a5201030051a05fc84d531b5d250c23f4f886f6812f9fe3f402d61607f977b4ecd2701c19010000fd781529d58fc2523cf396a7f25440b409857e7e221766c57214b1d38c7b481f01010062f542f45ea3660f86c013ced80534cb5fd4c19d66c56e7e8c5d4bf2d40acc5e010100b121e91836fd7cd5102b654e9f72f3cf6fdbfd0b161c53a9c54b12c841126331020100000001cd4e4cac3c7b56920d1e7655e7e260d31f29d9a388d04910f1bbd72304a79029010000006b483045022100e75279a205a547c445719420aa3138bf14743e3f42618e5f86a19bde14bb95f7022064777d34776b05d816daf1699493fcdf2ef5a5ab1ad710d9c97bfb5b8f7cef3641210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013e660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000001000100000001ac4e164f5bc16746bb0868404292ac8318bbac3800e4aad13a014da427adce3e000000006a47304402203a61a2e931612b4bda08d541cfb980885173b8dcf64a3471238ae7abcd368d6402204cbf24f04b9aa2256d8901f0ed97866603d2be8324c2bfb7a37bf8fc90edd5b441210263e2dee22b1ddc5e11f6fab8bcd2378bdd19580d640501ea956ec0e786f93e76ffffffff013c660000000000001976a9146bfd5c7fbe21529d45803dbcf0c87dd3c71efbc288ac0000000001"
	invalidBeefLowFees        = "0100beef01fe4e6d0c001002fd9c67028ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160cfd9d6700db1332728830a58c83a5970dcd111a575a585b43b0492361ea8082f41668f8bd01fdcf3300e568706954aae516ef6df7b5db7828771a1f3fcf1b6d65389ec8be8c46057a3c01fde6190001a6028d13cc988f55c8765e3ffcdcfc7d5185a8ebd68709c0adbe37b528557b01fdf20c001cc64f09a217e1971cabe751b925f246e3c2a8e145c49be7b831eaea3e064d7501fd7806009ccf122626a20cdb054877ef3f8ae2d0503bb7a8704fdb6295b3001b5e8876a101fd3d0300aeea966733175ff60b55bc77edcb83c0fce3453329f51195e5cbc7a874ee47ad01fd9f0100f67f50b53d73ffd6e84c02ee1903074b9a5b2ac64c508f7f26349b73cca9d7e901ce006ce74c7beed0c61c50dda8b578f0c0dc5a393e1f8758af2fb65edf483afcaa68016600e32475e17bdd141d62524d0005989dd1db6ca92c6af70791b0e4802be4c5c8c1013200b88162f494f26cc3a1a4a7dcf2829a295064e93b3dbb2f72e21a73522869277a011800a938d3f80dd25b6a3a80e450403bf7d62a1068e2e4b13f0656c83f764c55bb77010d006feac6e4fea41c37c508b5bfdc00d582f6e462e6754b338c95b448df37bd342c010700bf5448356be23b2b9afe53d00cee047065bbc16d0bbcc5f80aa8c1b509e45678010200c2e37431a437ee311a737aecd3caae1213db353847f33792fd539e380bdb4d440100005d5aef298770e2702448af2ce014f8bfcded5896df5006a44b5f1b6020007aeb01010091484f513003fcdb25f336b9b56dafcb05fbc739593ab573a2c6516b344ca5320201000000027b0a1b12c7c9e48015e78d3a08a4d62e439387df7e0d7a810ebd4af37661daaa000000006a47304402207d972759afba7c0ffa6cfbbf39a31c2aeede1dae28d8841db56c6dd1197d56a20220076a390948c235ba8e72b8e43a7b4d4119f1a81a77032aa6e7b7a51be5e13845412103f78ec31cf94ca8d75fb1333ad9fc884e2d489422034a1efc9d66a3b72eddca0fffffffff7f36874f858fb43ffcf4f9e3047825619bad0e92d4b9ad4ba5111d1101cbddfe010000006a473044022043f048043d56eb6f75024808b78f18808b7ab45609e4c4c319e3a27f8246fc3002204b67766b62f58bf6f30ea608eaba76b8524ed49f67a90f80ac08a9b96a6922cd41210254a583c1c51a06e10fab79ddf922915da5f5c1791ef87739f40cb68638397248ffffffff03e8030000000000001976a914b08f70bc5010fb026de018f19e7792385a146b4a88acf3010000000000001976a9147d48635f889372c3da12d75ce246c59f4ab907ed88acf7000000000000001976a914b8fbd58685b6920d8f9a8f1b274d8696708b51b088ac00000000010001000000018ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160c000000006b483045022100e47fbd96b59e2c22be273dcacea74a4be568b3e61da7eddddb6ce43d459c4cf202201a580f3d9442d5dce3f2ced03256ca147bcd230975a6067954e22415715f4490412102b0c8980f5d2cab77c92c68ac46442feba163a9d306913f6a34911fc618c3c4e7ffffffff0188130000000000001976a9148a8c4546a95e6fc8d18076a9980d59fd882b4e6988ac0000000000"
	invalidBeefInvalidScripts = "0100beef01fe4e6d0c001002fd909002088a382ec07a8cf47c6158b68e5822852362102d8571482d1257e0b7527e1882fd91900065cb01218f2506bb51155d243e4d6b32d69d1b5f2221c52e26963cfd8cf7283201fd4948008d7a44ae384797b0ae84db0c857e8c1083425d64d09ef8bc5e2e9d270677260501fd25240060f38aa33631c8d70adbac1213e7a5b418c90414e919e3a12ced63dd152fd85a01fd1312005ff132ee64a7a0c79150a29f66ef861e552d3a05b47d6303f5d8a2b2a09bc61501fd080900cc0baf21cf06b9439dfe05dce9bdb14ddc2ca2d560b1138296ef5769851a84b301fd85040063ccb26232a6e1d3becdb47a0f19a67a562b754e8894155b3ae7bba10335ce5101fd430200e153fc455a0f2c8372885c11af70af904dcf44740b9ebf3b3e5b2234cce550bc01fd20010077d5ea69d1dcc379dde65d6adcebde1838190118a8fae928c037275e78bd87910191000263e4f31684a25169857f2788aeef603504931f92585f02c4c9e023b2aa43d1014900de72292e0b3e5eeacfa2b657bf4d46c885559b081ee78632a99b318c1148d85c01250068a5f831ca99b9e7f3720920d6ea977fd2ab52b83d1a6567dafa4c8cafd941ed0113006a0b91d83f9056b702d6a8056af6365c7da626fc3818b815dd4b0de22d05450f0108009876ce56b68545a75859e93d200bdde7880d46f39384818b259ed847a9664ddf010500990bc5e95cacbc927b5786ec39a183f983fe160d52829cf47521c7eb369771c30103004fe794e50305f590b6010a51d050bf47dfeaabfdb949c5ee0673f577a59537d70100004dad44a358aea4d8bc1917912539901f5ae44e07a4748e1a9d3018814b0759d0020100000002704273c86298166ac351c3aa9ac90a8029e4213b5f1b03c3bbf4bc5fb09cdd43010000006a4730440220398d6389e8a156a3c6c1ca355e446d844fd480193a93af832afd1c87d0f04784022050091076b8f7405b37ce6e795d1b92526396ac2b14f08e91649b908e711e2b044121030ef6975d46dbab4b632ef62fdbe97de56d183be1acc0be641d2c400ae01cf136ffffffff2f41ed6a2488ac3ba4a3c330a15fa8193af87f0192aa59935e6c6401d92dc3a00a0000006a47304402200ad9cf0dc9c90a4c58b08910740b4a8b3e1a7e37db1bc5f656361b93f412883d0220380b6b3d587103fc8bf3fe7bed19ab375766984c67ebb7d43c993bcd199f32a441210205ef4171f58213b5a2ddf16ac6038c10a2a8c3edc1e6275cb943af4bb3a58182ffffffff03e8030000000000001976a9148a8c4546a95e6fc8d18076a9980d59fd882b4e6988acf4010000000000001976a914c7662da5e0a6a179141a7872045538126f1e954288acf5000000000000001976a914765bdf10934f5aac894cf8a3795c9eeb494c013488ac0000000001000100000001088a382ec07a8cf47c6158b68e5822852362102d8571482d1257e0b7527e1882000000006a4730440220610bba9ed83a47641c34bbbcf8eeb536d2ae6cfddc7644a8c520bb747f798c3702206a23c9f45273772dd7e80ba21a5c4613d6ffe7ba1c75b729eae0cdd484fee2bd412103c0cd91af135d09f98d57e34af28e307daf36bccd4764708e8a3f7ea5cebf01a9ffffffff01c8000000000000001976a9148ce2d21f9a75e98600be76b25b91c4fef6b40bcd88ac0000000000"

	inputTxLowFees         = "0100000001fbbe01d83cb1f53a63ef91c0fce5750cbd8075efef5acd2ff229506a45ab832c010000006a473044022064be2f304950a87782b44e772390836aa613f40312a0df4993e9c5123d0c492d02202009b084b66a3da939fb7dc5d356043986539cac4071372d0a6481d5b5e418ca412103fc12a81e5213e30c7facc15581ac1acbf26a8612a3590ffb48045084b097d52cffffffff02bf010000000000001976a914c2ca67db517c0c972b9a6eb1181880ed3a528e3188acD0070000000000001976a914f1e6837cf17b485a1dcea9e943948fafbe5e9f6888ac00000000"
	inputTxLowFeesBytes, _ = hex.DecodeString(inputTxLowFees)

	defaultPolicy = &bitcoin.Settings{
		ExcessiveBlockSize:              2000000000,
		BlockMaxSize:                    512000000,
		MaxTxSizePolicy:                 100000000,
		MaxOrphanTxSize:                 1000000000,
		DataCarrierSize:                 4294967295,
		MaxScriptSizePolicy:             100000000,
		MaxOpsPerScriptPolicy:           4294967295,
		MaxScriptNumLengthPolicy:        10000,
		MaxPubKeysPerMultisigPolicy:     4294967295,
		MaxTxSigopsCountsPolicy:         4294967295,
		MaxStackMemoryUsagePolicy:       100000000,
		MaxStackMemoryUsageConsensus:    200000000,
		LimitAncestorCount:              10000,
		LimitCPFPGroupMembersCount:      25,
		MaxMempool:                      2000000000,
		MaxMempoolSizedisk:              0,
		MempoolMaxPercentCPFP:           10,
		AcceptNonStdOutputs:             true,
		DataCarrier:                     true,
		MinMiningTxFee:                  1e-8,
		MaxStdTxValidationDuration:      3,
		MaxNonStdTxValidationDuration:   1000,
		MaxTxChainValidationBudget:      50,
		ValidationClockCpu:              true,
		MinConsolidationFactor:          20,
		MaxConsolidationInputScriptSize: 150,
		MinConfConsolidationInput:       6,
		MinConsolidationInputMaturity:   6,
		AcceptNonStdConsolidationInput:  false,
	}
	testLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	txResult   = &metamorph.TransactionStatus{
		TxID:        validTxID,
		BlockHash:   "",
		BlockHeight: 0,
		Status:      "OK",
		Timestamp:   time.Now().Unix(),
	}
	beefTxResult = &metamorph.TransactionStatus{
		TxID:        validBeefTxID,
		BlockHash:   "",
		BlockHeight: 0,
		Status:      "OK",
		Timestamp:   time.Now().Unix(),
	}
	txResults = []*metamorph.TransactionStatus{
		{
			TxID:        validBeefTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
		},
		{
			TxID:        validTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
		},
	}
	txCallbackResults = []*metamorph.TransactionStatus{
		{
			TxID:        validBeefTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
			Callbacks: []*metamorph_api.Callback{
				{
					CallbackUrl: "https://callback.example.com",
				},
			},
			LastSubmitted: *timestamppb.New(time.Now()),
		},
		{
			TxID:        validTxID,
			BlockHash:   "",
			BlockHeight: 0,
			Status:      "OK",
			Timestamp:   time.Now().Unix(),
			Callbacks: []*metamorph_api.Callback{
				{
					CallbackUrl: "https://callback.example.com",
				},
			},
			LastSubmitted: *timestamppb.New(time.Now()),
		},
	}

	validExtendedTxBytes, _        = hex.DecodeString(validExtendedTx)
	errBEEFDecode                  = *api.NewErrorFields(api.ErrStatusMalformed, "error while decoding BEEF\nfailed to parse beef\ncould not read varint type: EOF")
	invalidBeefNoBUMPIndexBytes, _ = hex.DecodeString(invalidBeefNoBUMPIndex)
)

type PostTransactionsTest struct {
	name              string
	contentTypes      []string
	options           api.POSTTransactionsParams
	expectedStatus    api.StatusCode
	expectedError     error
	inputTxs          map[string]io.Reader
	bErr              string
	errorsLength      int
	expectedErrors    map[string]string
	txHandler         *mtmMocks.TransactionHandlerMock
	dv                *apiHandlerMocks.DefaultValidatorMock
	expectedErrorCode api.StatusCode
	txIDs             []string
	errBEEFDecode     api.ErrorFields
	bv                *apiHandlerMocks.BeefValidatorMock
	skipProcessing    bool
}

func TestNewDefault(t *testing.T) {
	t.Run("simple init", func(t *testing.T) {
		btxClient := &btxMocks.ClientMock{}

		dv := &apiHandlerMocks.DefaultValidatorMock{}
		bv := &apiHandlerMocks.BeefValidatorMock{}
		defaultHandler, err := NewDefault(testLogger, nil, btxClient, nil, dv, bv)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
	})
}

func TestGETPolicy(t *testing.T) {
	t.Run("default policy", func(t *testing.T) {
		// given
		btxClient := &btxMocks.ClientMock{}
		bv := &apiHandlerMocks.BeefValidatorMock{}
		dv := &apiHandlerMocks.DefaultValidatorMock{}
		sut, err := NewDefault(testLogger, nil, btxClient, defaultPolicy, dv, bv)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/policy", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		// when
		err = sut.GETPolicy(ctx)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		bPolicy := rec.Body.Bytes()
		var policyResponse api.PolicyResponse
		_ = json.Unmarshal(bPolicy, &policyResponse)

		// then
		require.NotNil(t, policyResponse)
		assert.Equal(t, uint64(1), policyResponse.Policy.MiningFee.Satoshis)
		assert.Equal(t, uint64(1000), policyResponse.Policy.MiningFee.Bytes)
		assert.Equal(t, uint64(100000000), policyResponse.Policy.Maxscriptsizepolicy)
		assert.Equal(t, uint64(4294967295), policyResponse.Policy.Maxtxsigopscountspolicy)
		assert.Equal(t, uint64(100000000), policyResponse.Policy.Maxtxsizepolicy)
		assert.False(t, policyResponse.Timestamp.IsZero())
	})
}

func TestGETHealth(t *testing.T) {
	t.Run("health check success", func(t *testing.T) {
		txHandler := &mtmMocks.TransactionHandlerMock{
			HealthFunc: func(_ context.Context) error {
				return nil
			},
		}

		btxClient := &btxMocks.ClientMock{}
		bv := &apiHandlerMocks.BeefValidatorMock{}
		dv := &apiHandlerMocks.DefaultValidatorMock{}
		sut, err := NewDefault(testLogger, txHandler, btxClient, defaultPolicy, dv, bv)
		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/health", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		// when
		err = sut.GETHealth(ctx)

		// then
		require.Nil(t, err)

		bPolicy := rec.Body.Bytes()
		var health api.Health
		_ = json.Unmarshal(bPolicy, &health)

		require.Equal(t, true, *health.Healthy)
		require.Equal(t, (*string)(nil), health.Reason)
	})

	t.Run("health check fail", func(t *testing.T) {
		txHandler := &mtmMocks.TransactionHandlerMock{
			HealthFunc: func(_ context.Context) error {
				return errors.New("some connection error")
			},
		}
		btxClient := &btxMocks.ClientMock{}
		bv := &apiHandlerMocks.BeefValidatorMock{}
		dv := &apiHandlerMocks.DefaultValidatorMock{}
		sut, err := NewDefault(testLogger, txHandler, btxClient, defaultPolicy, dv, bv)

		require.NoError(t, err)
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/health", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		// when
		err = sut.GETHealth(ctx)

		// then
		require.Nil(t, err)

		bPolicy := rec.Body.Bytes()
		var health api.Health
		_ = json.Unmarshal(bPolicy, &health)

		require.Equal(t, false, *health.Healthy)
		require.NotEqual(t, health.Reason, nil)
		require.Contains(t, *health.Reason, "some connection error")
	})
}

func TestValidateCallbackURL(t *testing.T) {
	tt := []struct {
		name        string
		callbackURL string

		expectedError error
	}{
		{
			name:          "empty callback URL",
			callbackURL:   "",
			expectedError: ErrInvalidCallbackURL,
		},
		{
			name:        "valid callback URL",
			callbackURL: "http://api.callback.com",
		},
		{
			name:          "blocked url",
			callbackURL:   "http://localhost",
			expectedError: ErrCallbackURLNotAcceptable,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCallbackURL(tc.callbackURL, []string{"http://localhost"})

			if err != nil {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGETTransactionStatus(t *testing.T) {
	tt := []struct {
		name                 string
		txHandlerStatusFound *metamorph.TransactionStatus
		txHandlerErr         error

		expectedStatus   api.StatusCode
		expectedResponse any
	}{
		{
			name: "success",
			txHandlerStatusFound: &metamorph.TransactionStatus{
				TxID:      "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
				Status:    "SEEN_ON_NETWORK",
				Timestamp: time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC).Unix(),
			},

			expectedStatus: api.StatusOK,
			expectedResponse: api.TransactionStatus{
				MerklePath:  PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				BlockHash:   PtrTo(""),
				ExtraInfo:   PtrTo(""),
				Timestamp:   time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC),
				TxStatus:    api.SEENONNETWORK,
				Txid:        "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
			},
		},
		{
			name: "success - double spend attempted",
			txHandlerStatusFound: &metamorph.TransactionStatus{
				TxID:         "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
				Status:       "DOUBLE_SPEND_ATTEMPTED",
				Timestamp:    time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC).Unix(),
				CompetingTxs: []string{"1234"},
			},

			expectedStatus: api.StatusOK,
			expectedResponse: api.TransactionStatus{
				MerklePath:   PtrTo(""),
				BlockHeight:  PtrTo(uint64(0)),
				BlockHash:    PtrTo(""),
				ExtraInfo:    PtrTo(""),
				Timestamp:    time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC),
				TxStatus:     api.DOUBLESPENDATTEMPTED,
				Txid:         "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46",
				CompetingTxs: PtrTo([]string{"1234"}),
			},
		},
		{
			name:                 "error - tx not found",
			txHandlerStatusFound: nil,
			txHandlerErr:         metamorph.ErrTransactionNotFound,

			expectedStatus:   api.ErrStatusNotFound,
			expectedResponse: *api.NewErrorFields(api.ErrStatusNotFound, "transaction not found"),
		},
		{
			name:                 "error - generic",
			txHandlerStatusFound: nil,
			txHandlerErr:         errors.New("some error"),

			expectedStatus:   api.ErrStatusGeneric,
			expectedResponse: *api.NewErrorFields(api.ErrStatusGeneric, "some error"),
		},
		{
			name:                 "error - no tx",
			txHandlerStatusFound: nil,
			txHandlerErr:         nil,

			expectedStatus:   api.ErrStatusNotFound,
			expectedResponse: *api.NewErrorFields(api.ErrStatusNotFound, "failed to find transaction"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rec, ctx := createEchoGetRequest("/v1/tx/c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46")

			txHandler := &mtmMocks.TransactionHandlerMock{
				GetTransactionStatusFunc: func(_ context.Context, _ string) (*metamorph.TransactionStatus, error) {
					return tc.txHandlerStatusFound, tc.txHandlerErr
				},
			}

			btxClient := &btxMocks.ClientMock{}
			bv := &apiHandlerMocks.BeefValidatorMock{}
			dv := &apiHandlerMocks.DefaultValidatorMock{}
			defaultHandler, err := NewDefault(testLogger, txHandler, btxClient, nil, dv, bv, WithNow(func() time.Time { return time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC) }))
			require.NoError(t, err)

			err = defaultHandler.GETTransactionStatus(ctx, "c9648bf65a734ce64614dc92877012ba7269f6ea1f55be9ab5a342a2f768cf46")
			require.NoError(t, err)

			assert.Equal(t, int(tc.expectedStatus), rec.Code)

			b := rec.Body.Bytes()

			switch v := tc.expectedResponse.(type) {
			case api.TransactionStatus:
				var txStatus api.TransactionStatus
				err = json.Unmarshal(b, &txStatus)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txStatus)
			case api.ErrorFields:
				var txErr api.ErrorFields
				err = json.Unmarshal(b, &txErr)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txErr)
			default:
				require.Fail(t, fmt.Sprintf("response type %T does not match any valid types", v))
			}
		})
	}
}

func TestPOSTTransaction(t *testing.T) { //nolint:funlen
	errFieldMissingInputs := *api.NewErrorFields(api.ErrStatusTxFormat, "arc error 460: failed to get raw transactions for parent")
	errFieldMissingInputs.Txid = PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	errFieldSubmitTx := *api.NewErrorFields(api.ErrStatusGeneric, "failed to submit tx")
	errFieldSubmitTx.Txid = PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	errFieldValidation := *api.NewErrorFields(api.ErrStatusFees, "arc error 465: transaction fee is too low\nminimum expected fee: 22500000000, actual fee: 12")
	errFieldValidation.Txid = PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118")

	errBEEFDecode := *api.NewErrorFields(api.ErrStatusMalformed, "error while decoding BEEF\ninvalid BEEF - HasBUMP flag set, but no BUMP index")
	errBEEFLowFees := *api.NewErrorFields(api.ErrStatusFees, "arc error 465: transaction fee of 0 sat is too low - minimum expected fee is 1 sat")
	errBEEFLowFees.Txid = PtrTo("8184849de6af441c7de088428073c2a9131b08f7d878d9f49c3faf6d941eb168")
	errBEEFInvalidScripts := *api.NewErrorFields(api.ErrStatusUnlockingScripts, "arc error 461: invalid script")
	errBEEFInvalidScripts.Txid = PtrTo("ea2924da32c47b9942cda5ad30d3c01610ca554ca3a9ca01b2ccfe72bf0667be")

	now := time.Date(2023, 5, 3, 10, 0, 0, 0, time.UTC)

	tt := []struct {
		name                       string
		contentType                string
		txHexString                string
		getTx                      []byte
		submitTxResponse           *metamorph.TransactionStatus
		submitTxErr                error
		validateTransactionErr     error
		validateBeefTransactionErr error

		expectedStatus   api.StatusCode
		expectedResponse any
		expectedError    error

		expectedFee uint64
	}{
		{
			name:        "empty tx - text/plain",
			contentType: contentTypes[0],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: no transaction found - empty request body"),
			expectedError:    ErrEmptyBody,
		},
		{
			name:        "invalid tx - empty payload, application/json",
			contentType: contentTypes[1],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: no transaction found - empty request body"),
			expectedError:    ErrEmptyBody,
		},
		{
			name:        "empty tx - application/octet-stream",
			contentType: contentTypes[2],

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: no transaction found - empty request body"),
			expectedError:    ErrEmptyBody,
		},
		{
			name:        "invalid mime type",
			contentType: echo.MIMEApplicationXML,
			txHexString: validTx,

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: given content-type application/xml does not match any of the allowed content-types"),
		},
		{
			name:        "invalid tx - text/plain",
			contentType: contentTypes[0],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: encoding/hex: invalid byte: U+0074 't'"),
		},
		{
			name:        "invalid json - application/json",
			contentType: contentTypes[1],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: invalid character 'e' in literal true (expecting 'r')"),
		},
		{
			name:        "invalid tx - incorrect json field, application/json",
			contentType: contentTypes[1],
			txHexString: fmt.Sprintf("{\"txHex\": \"%s\"}", validTx),

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "error parsing transactions from request: no transaction found - empty request body"),
			expectedError:    ErrEmptyBody,
		},
		{
			name:        "invalid tx - invalid hex, application/octet-stream",
			contentType: contentTypes[2],
			txHexString: "test",

			expectedStatus:   400,
			expectedResponse: *api.NewErrorFields(api.ErrStatusBadRequest, "could not read varint type: EOF"),
		},
		{
			name:        "valid tx - fees too low",
			contentType: contentTypes[0],
			txHexString: validTx,
			getTx:       validTxParentBytes,
			expectedFee: 1000,
			submitTxResponse: &metamorph.TransactionStatus{
				TxID: "",
			},
			validateTransactionErr: validator.NewError(defaultvalidator.ErrTxFeeTooLow, api.ErrStatusFees),

			expectedStatus:   465,
			expectedResponse: errFieldValidation,
			expectedError:    defaultvalidator.ErrTxFeeTooLow,
		},
		{
			name:             "valid tx - submit error",
			contentType:      contentTypes[0],
			txHexString:      validExtendedTx,
			getTx:            validTxParentBytes,
			submitTxErr:      errors.New("failed to submit tx"),
			submitTxResponse: nil,

			expectedStatus:   409,
			expectedResponse: errFieldSubmitTx,
		},
		{
			name:        "valid tx - success",
			contentType: contentTypes[0],
			txHexString: validExtendedTx,
			getTx:       inputTxLowFeesBytes,

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:        validTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				ExtraInfo:   PtrTo(""),
				MerklePath:  PtrTo(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    api.TransactionResponseTxStatusSEENONNETWORK,
				Txid:        validTxID,
			},
		},
		{
			name:        "valid tx - double spend attempted",
			contentType: contentTypes[0],
			txHexString: validExtendedTx,
			getTx:       inputTxLowFeesBytes,

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:         validTxID,
				BlockHash:    "",
				BlockHeight:  0,
				Status:       "DOUBLE_SPEND_ATTEMPTED",
				CompetingTxs: []string{"1234"},
				Timestamp:    time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:    PtrTo(""),
				BlockHeight:  PtrTo(uint64(0)),
				ExtraInfo:    PtrTo(""),
				CompetingTxs: PtrTo([]string{"1234"}),
				MerklePath:   PtrTo(""),
				Status:       200,
				Timestamp:    now,
				Title:        "OK",
				TxStatus:     "DOUBLE_SPEND_ATTEMPTED",
				Txid:         validTxID,
			},
		},
		{
			name:        "invalid BEEF - text/plain - no BUMP index",
			contentType: contentTypes[0],
			txHexString: invalidBeefNoBUMPIndex,

			expectedStatus:   463,
			expectedResponse: errBEEFDecode,
			expectedError:    ErrDecodingBeef,
		},
		{
			name:                       "invalid BEEF - text/plain - low fees",
			contentType:                contentTypes[0],
			txHexString:                invalidBeefLowFees,
			validateBeefTransactionErr: validator.NewError(defaultvalidator.ErrTxFeeTooLow, api.ErrStatusFees),

			expectedStatus:   465,
			expectedResponse: errBEEFLowFees,
		},
		{
			name:                       "invalid BEEF - text/plain - invalid scripts",
			contentType:                contentTypes[0],
			txHexString:                invalidBeefInvalidScripts,
			validateBeefTransactionErr: validator.NewError(errors.New("validation error"), api.ErrStatusUnlockingScripts),

			expectedStatus:   461,
			expectedResponse: errBEEFInvalidScripts,
		},
		{
			name:        "valid BEEF - success - text/plain",
			contentType: contentTypes[0],
			txHexString: validBeef,

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:        validBeefTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				ExtraInfo:   PtrTo(""),
				MerklePath:  PtrTo(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        validBeefTxID,
			},
		},
		{
			name:        "valid BEEF - success - application/json",
			contentType: contentTypes[1],
			txHexString: fmt.Sprintf("{\"rawTx\": \"%s\"}", validBeef),

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:        validBeefTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				ExtraInfo:   PtrTo(""),
				MerklePath:  PtrTo(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        validBeefTxID,
			},
		},
		{
			name:        "valid BEEF 1 mined tx - success - application/json",
			contentType: contentTypes[1],
			txHexString: fmt.Sprintf("{\"rawTx\": \"%s\"}", validBeef1MinexTx),

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:        validBeef1MinexTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				ExtraInfo:   PtrTo(""),
				MerklePath:  PtrTo(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        validBeef1MinexTxID,
			},
		},
		{
			name:        "valid BEEF - success - application/octet-stream",
			contentType: contentTypes[2],
			txHexString: string(validBeefBytes),

			submitTxResponse: &metamorph.TransactionStatus{
				TxID:        validBeefTxID,
				BlockHash:   "",
				BlockHeight: 0,
				Status:      "SEEN_ON_NETWORK",
				Timestamp:   time.Now().Unix(),
			},

			expectedStatus: 200,
			expectedResponse: api.TransactionResponse{
				BlockHash:   PtrTo(""),
				BlockHeight: PtrTo(uint64(0)),
				ExtraInfo:   PtrTo(""),
				MerklePath:  PtrTo(""),
				Status:      200,
				Timestamp:   now,
				Title:       "OK",
				TxStatus:    "SEEN_ON_NETWORK",
				Txid:        validBeefTxID,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// given
			policy := *defaultPolicy
			if tc.expectedFee > 0 {
				policy.MinMiningTxFee = float64(tc.expectedFee)
			}

			txHandler := &mtmMocks.TransactionHandlerMock{
				HealthFunc: func(_ context.Context) error {
					return nil
				},

				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
					if tc.getTx != nil {
						tx, _ := sdkTx.NewTransactionFromBytes(tc.getTx)

						mt := metamorph.Transaction{
							TxID:        tx.TxID().String(),
							Bytes:       tc.getTx,
							BlockHeight: 100,
						}
						return []*metamorph.Transaction{&mt}, nil
					}

					return nil, nil
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return nil, metamorph.ErrTransactionNotFound
				},

				SubmitTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					return []*metamorph.TransactionStatus{tc.submitTxResponse}, tc.submitTxErr
				},
			}

			blocktxClient := &btxMocks.ClientMock{}

			handlerStats, err := NewStats()
			urlRestrictions := []string{"skiptest"}
			tracer := attribute.KeyValue{Key: "testnet", Value: attribute.StringValue("test")}
			require.NoError(t, err)
			dv := &apiHandlerMocks.DefaultValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Transaction, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) error {
					return tc.validateTransactionErr
				},
			}

			bv := &apiHandlerMocks.BeefValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
					return nil, tc.validateBeefTransactionErr
				},
			}
			sut, err := NewDefault(testLogger, txHandler, blocktxClient, &policy, dv, bv,
				WithNow(func() time.Time { return now }),
				WithStats(handlerStats),
				WithCallbackURLRestrictions(urlRestrictions),
				WithRebroadcastExpiration(23*time.Hour),
				WithServerMaxTimeoutDefault(5*time.Second),
				WithTracer(tracer),
				WithStandardFormatSupported(true),
			)
			require.NoError(t, err)

			defer sut.Shutdown()

			inputTx := strings.NewReader(tc.txHexString)
			rec, ctx := createEchoPostRequest(inputTx, tc.contentType, "/v1/tx")

			// when
			err = sut.POSTTransaction(ctx, api.POSTTransactionParams{})

			// then
			require.NoError(t, err)

			assert.Equal(t, int(tc.expectedStatus), rec.Code)

			b := rec.Body.Bytes()

			switch v := tc.expectedResponse.(type) {
			case api.TransactionResponse:
				var txResponse api.TransactionResponse
				err = json.Unmarshal(b, &txResponse)
				require.NoError(t, err)

				assert.Equal(t, tc.expectedResponse, txResponse)
			case api.ErrorFields:
				var actualError api.ErrorFields
				err = json.Unmarshal(b, &actualError)
				require.NoError(t, err)

				expectedResp, ok := tc.expectedResponse.(api.ErrorFields)
				require.True(t, ok)
				assert.Equal(t, expectedResp.Txid, actualError.Txid)
				assert.Equal(t, expectedResp.Status, actualError.Status)
				assert.Equal(t, expectedResp.Detail, actualError.Detail)
				assert.Equal(t, expectedResp.Title, actualError.Title)
				assert.Equal(t, expectedResp.Instance, actualError.Instance)
				assert.Equal(t, expectedResp.Type, actualError.Type)
				if tc.expectedError != nil {
					assert.ErrorContains(t, errors.New(*actualError.ExtraInfo), tc.expectedError.Error())
				}
			default:
				require.Fail(t, fmt.Sprintf("response type %T does not match any valid types", v))
			}
		})
	}
}

func TestPOSTTransactions(t *testing.T) { //nolint:funlen
	tt := []PostTransactionsTest{
		{
			name:           "empty tx",
			contentTypes:   contentTypes,
			expectedStatus: api.ErrStatusBadRequest,
			options:        api.POSTTransactionsParams{},
		},
		{
			name:           "invalid parameters",
			expectedStatus: api.ErrStatusBadRequest,
			options: api.POSTTransactionsParams{
				XCallbackUrl:   PtrTo("callback.example.com"),
				XCallbackToken: PtrTo("test-token"),
				XWaitFor:       PtrTo(metamorph_api.Status_name[int32(metamorph_api.Status_ANNOUNCED_TO_NETWORK)]),
			},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain: strings.NewReader(validExtendedTx),
			},
			bErr:          "invalid callback URL\nparse \"callback.example.com\": invalid URI for request",
			expectedError: ErrInvalidCallbackURL,
		},
		{
			name:           "invalid mime type",
			expectedStatus: api.ErrStatusBadRequest,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMEApplicationXML: strings.NewReader(""),
			},
		},
		{
			name:           "invalid txs",
			expectedStatus: api.ErrStatusBadRequest,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMEApplicationXML: strings.NewReader(""),
			},
			expectedErrors: map[string]string{
				echo.MIMETextPlain:       "error parsing transactions from request: encoding/hex: invalid byte: U+0074 't'",
				echo.MIMEApplicationJSON: "error parsing transactions from request: invalid character 'e' in literal true (expecting 'r')",
				echo.MIMEOctetStream:     "",
			},
		},
		{
			name:              "valid tx - missing inputs",
			expectedStatus:    api.StatusOK,
			expectedErrorCode: api.ErrStatusTxFormat,
			options:           api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validTx + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validTx + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(validTxBytes),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				SubmitTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					var txStatuses []*metamorph.TransactionStatus
					return txStatuses, nil
				},

				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
					return nil, metamorph.ErrTransactionNotFound
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},

				HealthFunc: func(_ context.Context) error {
					return nil
				},
			},
			dv: &apiHandlerMocks.DefaultValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Transaction, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) error {
					return validator.NewError(errors.New("status format error"), api.ErrStatusTxFormat)
				},
			},
		},
		{
			name:           "valid tx",
			txIDs:          []string{validTxID},
			expectedStatus: api.StatusOK,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validExtendedTx + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validExtendedTx + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(validExtendedTxBytes),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				SubmitTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					txStatuses := []*metamorph.TransactionStatus{txResult}
					return txStatuses, nil
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},

				HealthFunc: func(_ context.Context) error {
					return nil
				},
			},
			dv: &apiHandlerMocks.DefaultValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Transaction, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) error {
					return nil
				},
			},
		},
		{
			name:           "invalid tx - beef",
			expectedStatus: api.ErrStatusMalformed,
			errBEEFDecode:  errBEEFDecode,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(invalidBeefNoBUMPIndex + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + invalidBeefNoBUMPIndex + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(invalidBeefNoBUMPIndexBytes),
			},
			expectedErrorCode: api.ErrStatusMalformed,
			txHandler: &mtmMocks.TransactionHandlerMock{
				HealthFunc: func(_ context.Context) error {
					return nil
				},
				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},
			},
		},
		{
			name:           "valid tx - beef",
			txIDs:          []string{validBeefTxID},
			expectedStatus: api.StatusOK,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validBeef + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validBeef + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(validBeefBytes),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				SubmitTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					txStatuses := []*metamorph.TransactionStatus{beefTxResult}
					return txStatuses, nil
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},

				HealthFunc: func(_ context.Context) error {
					return nil
				},
			},
			bv: &apiHandlerMocks.BeefValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
					return nil, nil
				},
			},
		},
		{
			name:           "2 valid tx - beef",
			txIDs:          []string{validBeefTxID, validBeefTxID},
			expectedStatus: api.StatusOK,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validBeef + "\n" + validBeef + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validBeef + "\"}, {\"rawTx\":\"" + validBeef + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(append(validBeefBytes, validBeefBytes...)),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
					tx, _ := sdkTx.NewTransactionFromBytes(validTxParentBytes)
					return []*metamorph.Transaction{
						{
							TxID:        tx.TxID().String(),
							Bytes:       validTxParentBytes,
							BlockHeight: 100,
						},
					}, nil
				},
				SubmitTransactionsFunc: func(_ context.Context, txs sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					var res []*metamorph.TransactionStatus
					for _, t := range txs {
						txID := t.TxID()
						if status, found := find(txResults, func(e *metamorph.TransactionStatus) bool { return e.TxID == txID.String() }); found {
							res = append(res, status)
						}
					}

					return res, nil
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},

				HealthFunc: func(_ context.Context) error {
					return nil
				},
			},
			bv: &apiHandlerMocks.BeefValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
					return nil, nil
				},
			},
		},
		{
			name:           "multiple formats - success",
			txIDs:          []string{validBeefTxID, validBeefTxID},
			expectedStatus: api.StatusOK,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validBeef + "\n" + validBeef + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validBeef + "\"}, {\"rawTx\":\"" + validBeef + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(append(validBeefBytes, validBeefBytes...)),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				GetTransactionsFunc: func(_ context.Context, _ []string) ([]*metamorph.Transaction, error) {
					tx, _ := sdkTx.NewTransactionFromBytes(validTxParentBytes)
					return []*metamorph.Transaction{
						{
							TxID:        tx.TxID().String(),
							Bytes:       validTxParentBytes,
							BlockHeight: 100,
						},
					}, nil
				},
				SubmitTransactionsFunc: func(_ context.Context, txs sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					var res []*metamorph.TransactionStatus
					for _, t := range txs {
						txID := t.TxID()
						if status, found := find(txResults, func(e *metamorph.TransactionStatus) bool { return e.TxID == txID.String() }); found {
							res = append(res, status)
						}
					}

					return res, nil
				},

				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return make([]*metamorph.TransactionStatus, 0), nil
				},

				HealthFunc: func(_ context.Context) error {
					return nil
				},
			},
			dv: &apiHandlerMocks.DefaultValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Transaction, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) error {
					return nil
				},
			},
			bv: &apiHandlerMocks.BeefValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
					return nil, nil
				},
			},
		},
		{
			name:           "skip processing transactions",
			txIDs:          []string{validBeefTxID, validTxID},
			expectedStatus: api.StatusOK,
			options:        api.POSTTransactionsParams{},
			inputTxs: map[string]io.Reader{
				echo.MIMETextPlain:       strings.NewReader(validBeef + "\n" + validBeef + "\n"),
				echo.MIMEApplicationJSON: strings.NewReader("[{\"rawTx\":\"" + validBeef + "\"}, {\"rawTx\":\"" + validBeef + "\"}]"),
				echo.MIMEOctetStream:     bytes.NewReader(append(validBeefBytes, validBeefBytes...)),
			},
			txHandler: &mtmMocks.TransactionHandlerMock{
				GetTransactionStatusesFunc: func(_ context.Context, _ []string) ([]*metamorph.TransactionStatus, error) {
					return txCallbackResults, nil
				},
				SubmitTransactionsFunc: func(_ context.Context, _ sdkTx.Transactions, _ *metamorph.TransactionOptions) ([]*metamorph.TransactionStatus, error) {
					return txCallbackResults, nil
				},
			},

			bv: &apiHandlerMocks.BeefValidatorMock{
				ValidateTransactionFunc: func(_ context.Context, _ *sdkTx.Beef, _ validator.FeeValidation, _ validator.ScriptValidation, _ int32) (*sdkTx.Transaction, error) {
					return nil, nil
				},
			},
			skipProcessing: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			//given
			btxClient := &btxMocks.ClientMock{}
			sut, err := NewDefault(testLogger, tc.txHandler, btxClient, defaultPolicy, tc.dv, tc.bv)
			require.NoError(t, err)
			//when then
			testPOSTTransactionsContentTypes(t, sut, tc)
			testPOSTTransactionsInputTxs(t, sut, tc)
			testPOSTTransactionsExpectedErrors(t, sut, tc)
		})
	}
}

func testPOSTTransactionsExpectedErrors(t *testing.T, sut *ArcDefaultHandler, tc PostTransactionsTest) {
	for contentType, expectedError := range tc.expectedErrors {
		// when
		rec, ctx := createEchoPostRequest(strings.NewReader("test"), contentType, "/v1/txs")
		err := sut.POSTTransactions(ctx, api.POSTTransactionsParams{})

		// then
		require.NoError(t, err)
		assert.Equal(t, int(tc.expectedStatus), rec.Code)

		b := rec.Body.Bytes()
		var bErr api.ErrorBadRequest
		err = json.Unmarshal(b, &bErr)
		require.NoError(t, err)

		errBadRequest := api.NewErrorFields(tc.expectedStatus, "")

		assert.Equal(t, float64(errBadRequest.Status), bErr.Status)
		assert.Equal(t, errBadRequest.Title, bErr.Title)
		if expectedError != "" {
			require.NotNil(t, bErr.ExtraInfo)
			assert.Equal(t, expectedError, *bErr.ExtraInfo)
		}
	}
}

func testPOSTTransactionsInputTxs(t *testing.T, sut *ArcDefaultHandler, tc PostTransactionsTest) {
	if tc.skipProcessing {
		//when
		callbackURL := "https://callback.example.com"
		rec, ctx := createEchoPostRequest(strings.NewReader("[{\"rawTx\":\""+validBeef+"\"}, {\"rawTx\":\""+validTx+"\"}]"), echo.MIMEApplicationJSON, "/v1/txs")
		err := sut.POSTTransactions(ctx, api.POSTTransactionsParams{
			XCallbackUrl: &callbackURL,
		})

		// then
		require.NoError(t, err)
		assert.Equal(t, int(tc.expectedStatus), rec.Code)
		assert.Equal(t, len(tc.txHandler.SubmitTransactionsCalls()), 0)

		b := rec.Body.Bytes()
		var bResponse []api.TransactionResponse
		_ = json.Unmarshal(b, &bResponse)

		require.Len(t, bResponse, len(tc.txIDs))
		for i, br := range bResponse {
			require.Equal(t, tc.txIDs[i], br.Txid)
		}
		return
	}

	for contentType, inputTx := range tc.inputTxs {
		// when
		rec, ctx := createEchoPostRequest(inputTx, contentType, "/v1/txs")
		err := sut.POSTTransactions(ctx, tc.options)

		// then
		require.NoError(t, err)
		assert.Equal(t, tc.expectedStatus, api.StatusCode(rec.Code))

		b := rec.Body.Bytes()
		var bErr []api.ErrorFields
		_ = json.Unmarshal(b, &bErr)

		if tc.errorsLength > 0 {
			require.GreaterOrEqual(t, len(bErr), tc.errorsLength)
			assert.Equal(t, tc.expectedError, bErr[0])
		}
		if tc.expectedErrorCode > 0 {
			require.GreaterOrEqual(t, len(bErr), tc.errorsLength)
			if tc.errorsLength > 0 {
				assert.Equal(t, int(tc.expectedErrorCode), bErr[0].Status)
			}
		}
		if len(tc.txIDs) > 0 {
			var bResponse []api.TransactionResponse
			_ = json.Unmarshal(b, &bResponse)

			require.Len(t, bResponse, len(tc.txIDs))
			for i, br := range bResponse {
				require.Equal(t, tc.txIDs[i], br.Txid)
			}
		}
		if tc.errBEEFDecode.Title != "" {
			b := rec.Body.Bytes()
			var bResponse api.ErrorFields
			_ = json.Unmarshal(b, &bResponse)

			require.Equal(t, errBEEFDecode, bResponse)
			require.ErrorContains(t, errors.New(*bResponse.ExtraInfo), ErrDecodingBeef.Error())
		}
	}
}

func testPOSTTransactionsContentTypes(t *testing.T, sut *ArcDefaultHandler, tc PostTransactionsTest) {
	for _, contentType := range tc.contentTypes {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/v1/txs", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		// when
		actualError := sut.POSTTransactions(ctx, tc.options)

		// then
		require.NoError(t, actualError)
		// multiple txs post always returns 200, the error code is given per tx
		assert.Equal(t, tc.expectedStatus, api.StatusCode(rec.Code))
	}
}

func createEchoPostRequest(inputTx io.Reader, contentType, target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, target, inputTx)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}

func createEchoGetRequest(target string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	return rec, ctx
}

func TestCalcFeesFromBSVPerKB(t *testing.T) {
	tests := []struct {
		name     string
		feePerKB float64
		satoshis uint64
		bytes    uint64
	}{
		{
			name:     "50 sats per KB",
			feePerKB: 0.00000050,
			satoshis: 50,
			bytes:    1000,
		},
		{
			name:     "5 sats per KB",
			feePerKB: 0.00000005,
			satoshis: 5,
			bytes:    1000,
		},
		{
			name:     "0.5 sats per KB",
			feePerKB: 0.000000005,
			satoshis: 5,
			bytes:    10000,
		},
		{
			name:     "0.01 sats per KB",
			feePerKB: 0.0000000001,
			satoshis: 1,
			bytes:    100000,
		},
		{
			name:     "0.001 sats per KB",
			feePerKB: 0.00000000001,
			satoshis: 1,
			bytes:    1000000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := calcFeesFromBSVPerKB(tt.feePerKB)
			assert.Equalf(t, tt.satoshis, got, "calcFeesFromBSVPerKB(%v)", tt.feePerKB)
			assert.Equalf(t, tt.bytes, got1, "calcFeesFromBSVPerKB(%v)", tt.feePerKB)
		})
	}
}

func TestGetTransactionOptions(t *testing.T) {
	tt := []struct {
		name   string
		params api.POSTTransactionsParams

		expectedError   error
		expectedOptions *metamorph.TransactionOptions
	}{
		{
			name:   "no options",
			params: api.POSTTransactionsParams{},

			expectedOptions: &metamorph.TransactionOptions{},
		},
		{
			name:   "max timeout",
			params: api.POSTTransactionsParams{},

			expectedOptions: &metamorph.TransactionOptions{},
		},
		{
			name: "valid callback url",
			params: api.POSTTransactionsParams{
				XCallbackUrl:   PtrTo("http://api.callme.com"),
				XCallbackToken: PtrTo("1234"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				CallbackURL:   "http://api.callme.com",
				CallbackToken: "1234",
			},
		},
		{
			name: "invalid callback url",
			params: api.POSTTransactionsParams{
				XCallbackUrl: PtrTo("api.callme.com"),
			},

			expectedError: ErrInvalidCallbackURL,
		},
		{
			name: "wait for - QUEUED",
			params: api.POSTTransactionsParams{
				XWaitFor: PtrTo("QUEUED"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_QUEUED,
			},
		},
		{
			name: "wait for - RECEIVED",
			params: api.POSTTransactionsParams{
				XWaitFor: PtrTo("RECEIVED"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_RECEIVED,
			},
		},
		{
			name: "wait for - SENT_TO_NETWORK",
			params: api.POSTTransactionsParams{
				XWaitFor: PtrTo("SENT_TO_NETWORK"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_SENT_TO_NETWORK,
			},
		},
		{
			name: "wait for - ACCEPTED_BY_NETWORK",
			params: api.POSTTransactionsParams{
				XWaitFor: PtrTo("ACCEPTED_BY_NETWORK"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
		},
		{
			name: "wait for - SEEN_ON_NETWORK",
			params: api.POSTTransactionsParams{
				XWaitFor: PtrTo("SEEN_ON_NETWORK"),
			},

			expectedOptions: &metamorph.TransactionOptions{
				WaitForStatus: metamorph_api.Status_SEEN_ON_NETWORK,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			options, actualErr := getTransactionsOptions(tc.params, make([]string, 0))

			if tc.expectedError != nil {
				require.ErrorIs(t, actualErr, tc.expectedError)
				return
			}
			require.NoError(t, actualErr)

			require.Equal(t, tc.expectedOptions, options)
		})
	}
}

func Test_handleError(t *testing.T) {
	tt := []struct {
		name        string
		submitError error

		expectedStatus api.StatusCode
		expectedArcErr *api.ErrorFields
	}{
		{
			name: "no error",

			expectedStatus: api.StatusOK,
			expectedArcErr: nil,
		},
		{
			name:        "generic error",
			submitError: errors.New("some error"),

			expectedStatus: api.ErrStatusGeneric,
			expectedArcErr: &api.ErrorFields{
				Detail:    "Transaction could not be processed",
				ExtraInfo: PtrTo("some error"),
				Title:     "Generic error",
				Type:      "https://bitcoin-sv.github.io/arc/#/errors?id=_409",
				Txid:      PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    409,
			},
		},
		{
			name: "validator error",
			submitError: &validator.Error{
				ArcErrorStatus: api.ErrStatusBadRequest,
				Err:            errors.New("validation failed"),
			},

			expectedStatus: api.ErrStatusBadRequest,
			expectedArcErr: &api.ErrorFields{
				Detail:    "The request seems to be malformed and cannot be processed",
				ExtraInfo: PtrTo("arc error 400: validation failed"),
				Title:     "Bad request",
				Type:      "https://bitcoin-sv.github.io/arc/#/errors?id=_400",
				Txid:      PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    400,
			},
		},
		{
			name: "parent not found error",
			submitError: &validator.Error{
				ArcErrorStatus: api.ErrStatusTxFormat,
				Err:            errors.New("parent transaction not found"),
			},

			expectedStatus: api.ErrStatusTxFormat,
			expectedArcErr: &api.ErrorFields{
				Detail:    "Missing input scripts: Transaction could not be transformed to extended format",
				ExtraInfo: PtrTo("arc error 460: parent transaction not found"),
				Title:     "Not extended format",
				Type:      "https://bitcoin-sv.github.io/arc/#/errors?id=_460",
				Txid:      PtrTo("a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118"),
				Status:    460,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			handler := ArcDefaultHandler{}

			tx, err := sdkTx.NewTransactionFromHex(validTx)
			require.NoError(t, err)

			ctx := context.Background()
			status, arcErr := handler.handleError(ctx, tx.TxID().String(), tc.submitError)

			require.Equal(t, tc.expectedStatus, status)
			require.Equal(t, tc.expectedArcErr, arcErr)
		})
	}
}

func Test_CurrentBlockUpdate(t *testing.T) {
	t.Run("check block height updates", func(t *testing.T) {
		bv := &apiHandlerMocks.BeefValidatorMock{}
		dv := &apiHandlerMocks.DefaultValidatorMock{}
		btxClient := &btxMocks.ClientMock{
			CurrentBlockHeightFunc: func(_ context.Context) (*blocktx_api.CurrentBlockHeightResponse, error) {
				return &blocktx_api.CurrentBlockHeightResponse{
					CurrentBlockHeight: 24,
				}, nil
			},
		}
		defaultHandler, err := NewDefault(testLogger, nil, btxClient, nil, dv, bv)
		defaultHandler.StartUpdateCurrentBlockHeight()
		time.Sleep(currentBlockUpdateInterval + 1*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, defaultHandler)
		assert.Equal(t, len(btxClient.CurrentBlockHeightCalls()), 1)
		assert.Equal(t, atomic.LoadInt32(&defaultHandler.currentBlockHeight), int32(24))
	})
}

func find[T any](arr []T, predicate func(T) bool) (T, bool) {
	for _, element := range arr {
		if predicate(element) {
			return element, true
		}
	}
	var zero T
	return zero, false
}
