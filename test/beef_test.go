package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/pkg/api"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

func TestBeef(t *testing.T) {
	testCases := []struct {
		name           string
		expectedStatus metamorph_api.Status
	}{
		{
			name:           "valid beef",
			expectedStatus: metamorph_api.Status_SEEN_ON_NETWORK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			address, privateKey := getNewWalletAddress(t)
			dstAddress, _ := getNewWalletAddress(t)

			generate(t, 200)
			txID := sendToAddress(t, address, 0.001)
			hash := generate(t, 1)

			beef, tx := prepareBeef(t, txID, hash, address, dstAddress, privateKey)

			body := api.POSTTransactionJSONRequestBody{
				RawTx: beef,
			}

			waitForStatus := api.WaitForStatus(tc.expectedStatus)
			params := &api.POSTTransactionParams{XWaitForStatus: &waitForStatus}

			// when
			response, err := arcClient.POSTTransactionWithResponse(context.Background(), params, body)

			// then
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode())
			require.NotNil(t, response.JSON200)
			require.Equal(t, tc.expectedStatus.String(), response.JSON200.TxStatus, "status not SEEN_ON_NETWORK")

			// when
			generate(t, 10)
			t.Log("waiting for 15s to give ARC time to perform the status update on DB")
			time.Sleep(15 * time.Second)

			statusResponse, err := arcClient.GETTransactionStatusWithResponse(context.Background(), tx.TxID())

			// then
			require.NoError(t, err)
			require.Equal(t, metamorph_api.Status_MINED.String(), *statusResponse.JSON200.TxStatus)
		})
	}
}

func TestBeef_Fail(t *testing.T) {
	testCases := []struct {
		name                 string
		beefStr              string
		expectedErrCode      int
		expectedErrMsgDetail string
	}{
		{
			name:                 "invalid beef - unverifiable merkle roots",
			beefStr:              "0100beef01fe4e6d0c001002fd9c67028ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160cfd9d6700db1332728830a58c83a5970dcd111a575a585b43b0492361ea8082f41668f8bd01fdcf3300e568706954aae516ef6df7b5db7828771a1f3fcf1b6d65389ec8be8c46057a3c01fde6190001a6028d13cc988f55c8765e3ffcdcfc7d5185a8ebd68709c0adbe37b528557b01fdf20c001cc64f09a217e1971cabe751b925f246e3c2a8e145c49be7b831eaea3e064d7501fd7806009ccf122626a20cdb054877ef3f8ae2d0503bb7a8704fdb6295b3001b5e8876a101fd3d0300aeea966733175ff60b55bc77edcb83c0fce3453329f51195e5cbc7a874ee47ad01fd9f0100f67f50b53d73ffd6e84c02ee1903074b9a5b2ac64c508f7f26349b73cca9d7e901ce006ce74c7beed0c61c50dda8b578f0c0dc5a393e1f8758af2fb65edf483afcaa68016600e32475e17bdd141d62524d0005989dd1db6ca92c6af70791b0e4802be4c5c8c1013200b88162f494f26cc3a1a4a7dcf2829a295064e93b3dbb2f72e21a73522869277a011800a938d3f80dd25b6a3a80e450403bf7d62a1068e2e4b13f0656c83f764c55bb77010d006feac6e4fea41c37c508b5bfdc00d582f6e462e6754b338c95b448df37bd342c010700bf5448356be23b2b9afe53d00cee047065bbc16d0bbcc5f80aa8c1b509e45678010200c2e37431a437ee311a737aecd3caae1213db353847f33792fd539e380bdb4d440100005d5aef298770e2702448af2ce014f8bfcded5896df5006a44b5f1b6020007aeb01010091484f513003fcdb25f336b9b56dafcb05fbc739593ab573a2c6516b344ca5320201000000027b0a1b12c7c9e48015e78d3a08a4d62e439387df7e0d7a810ebd4af37661daaa000000006a47304402207d972759afba7c0ffa6cfbbf39a31c2aeede1dae28d8841db56c6dd1197d56a20220076a390948c235ba8e72b8e43a7b4d4119f1a81a77032aa6e7b7a51be5e13845412103f78ec31cf94ca8d75fb1333ad9fc884e2d489422034a1efc9d66a3b72eddca0fffffffff7f36874f858fb43ffcf4f9e3047825619bad0e92d4b9ad4ba5111d1101cbddfe010000006a473044022043f048043d56eb6f75024808b78f18808b7ab45609e4c4c319e3a27f8246fc3002204b67766b62f58bf6f30ea608eaba76b8524ed49f67a90f80ac08a9b96a6922cd41210254a583c1c51a06e10fab79ddf922915da5f5c1791ef87739f40cb68638397248ffffffff03e8030000000000001976a914b08f70bc5010fb026de018f19e7792385a146b4a88acf3010000000000001976a9147d48635f889372c3da12d75ce246c59f4ab907ed88acf7000000000000001976a914b8fbd58685b6920d8f9a8f1b274d8696708b51b088ac00000000010001000000018ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160c000000006b483045022100e47fbd96b59e2c22be273dcacea74a4be568b3e61da7eddddb6ce43d459c4cf202201a580f3d9442d5dce3f2ced03256ca147bcd230975a6067954e22415715f4490412102b0c8980f5d2cab77c92c68ac46442feba163a9d306913f6a34911fc618c3c4e7ffffffff0188130000000000001976a9148a8c4546a95e6fc8d18076a9980d59fd882b4e6988ac0000000000",
			expectedErrCode:      int(api.ErrStatusFees),
			expectedErrMsgDetail: "Fees are too low",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			arcClient, err := api.NewClientWithResponses(arcEndpoint)
			require.NoError(t, err)

			generate(t, 100)

			body := api.POSTTransactionJSONRequestBody{
				RawTx: tc.beefStr,
			}

			response, err := arcClient.POSTTransactionWithResponse(context.Background(), nil, body)
			require.NoError(t, err)
			require.Equal(t, tc.expectedErrCode, response.StatusCode())
			require.NotNil(t, response.JSON465)
			require.Equal(t, tc.expectedErrMsgDetail, response.JSON465.Detail)
		})
	}
}

func prepareBeef(t *testing.T, inputTxID, blockHash, fromAddress, toAddress, privateKey string) (string, *bt.Tx) {
	rawTx := getRawTx(t, inputTxID)
	t.Logf("rawTx: %+v", rawTx)
	require.Equal(t, blockHash, rawTx.BlockHash, "block hash mismatch")

	blockData := getBlockDataByBlockHash(t, blockHash)
	t.Logf("blockdata: %+v", blockData)

	merkleHashes, txIndex := prepareMerkleHashesAndTxIndex(t, blockData.Txs, inputTxID)
	merkleTree := bc.BuildMerkleTreeStoreChainHash(merkleHashes)
	bump, err := bc.NewBUMPFromMerkleTreeAndIndex(blockData.Height, merkleTree, txIndex)
	require.NoError(t, err, "error creating BUMP from merkle hashes")

	merkleRootFromBump, err := bump.CalculateRootGivenTxid(inputTxID)
	require.NoError(t, err, "error calculating merkle root from bump")
	t.Logf("merkleroot from bump: %s", merkleRootFromBump)
	require.Equal(t, blockData.MerkleRoot, merkleRootFromBump, "merkle roots mismatch")

	utxos := getUtxos(t, fromAddress)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	tx, err := createTx(privateKey, toAddress, utxos[0])
	require.NoError(t, err, "could not create tx")
	t.Logf("tx created, hex: %s", tx.String())

	beef := buildBeefString(t, rawTx.Hex, bump, tx)
	t.Logf("beef created, hex: %s", beef)

	return beef, tx
}

func prepareMerkleHashesAndTxIndex(t *testing.T, txs []string, txID string) ([]*chainhash.Hash, uint64) {
	var merkleHashes []*chainhash.Hash
	var txIndex uint64

	for i, txid := range txs {
		if txid == txID {
			txIndex = uint64(i)
		}
		h, err := chainhash.NewHashFromStr(txid)
		require.NoError(t, err)

		merkleHashes = append(merkleHashes, h)
	}

	return merkleHashes, txIndex
}

func buildBeefString(t *testing.T, inputTxHex string, bump *bc.BUMP, newTx *bt.Tx) string {
	versionMarker := "0100beef"
	nBumps := "01"
	bumpData, err := bump.String()
	require.NoError(t, err, "could not get bump string")

	nTransactions := "02"
	rawParentTx := inputTxHex
	parentHasBump := "01"
	parentBumpIndex := "00"
	rawTx := newTx.String()
	hasBump := "00"

	beef := versionMarker +
		nBumps +
		bumpData +
		nTransactions +
		rawParentTx +
		parentHasBump +
		parentBumpIndex +
		rawTx +
		hasBump

	return beef
}
