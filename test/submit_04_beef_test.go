//go:build e2e

package test

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	safe "github.com/ccoveille/go-safecast"
	"github.com/libsv/go-bc"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/node_client"
)

func TestBeef(t *testing.T) {
	t.Run("valid beef with unmined parents - response for the tip, callback for each", func(t *testing.T) {
		// given

		address, privateKey := node_client.GetNewWalletAddress(t, bitcoind)
		dstAddress, _ := node_client.GetNewWalletAddress(t, bitcoind)

		txID := node_client.SendToAddress(t, bitcoind, address, 0.002)
		hash := node_client.Generate(t, bitcoind, 1)

		beef, middleTx, tx, expectedCallbacks := prepareBeef(t, txID, hash, address, dstAddress, privateKey)

		callbackReceivedChan := make(chan *TransactionResponse, expectedCallbacks) // do not block callback server responses
		callbackErrChan := make(chan error, expectedCallbacks)

		lis, err := net.Listen("tcp", ":9000")
		require.NoError(t, err)
		mux := http.NewServeMux()
		defer func() {
			err = lis.Close()
			require.NoError(t, err)
		}()

		callbackURL, token := registerHandlerForCallback(t, callbackReceivedChan, callbackErrChan, nil, mux)
		defer func() {
			t.Log("closing channels")

			close(callbackReceivedChan)
			close(callbackErrChan)
		}()

		go func() {
			t.Logf("starting callback server")
			err = http.Serve(lis, mux)
			if err != nil {
				t.Log("callback server stopped")
			}
		}()

		waitForStatusTimeoutSeconds := 30

		// when
		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: beef}), map[string]string{
			"X-WaitFor":       StatusSeenOnNetwork,
			"X-CallbackUrl":   callbackURL,
			"X-CallbackToken": token,
			"X-MaxTimeout":    strconv.Itoa(waitForStatusTimeoutSeconds),
		}, http.StatusOK)

		// then
		require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

		node_client.Generate(t, bitcoind, 1)

		statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx.TxID())
		statusResp := getRequest[TransactionResponse](t, statusURL)
		require.Equal(t, StatusMined, statusResp.TxStatus)

		// verify callbacks for both unmined txs in BEEF
		lastTxCallbackReceived := false
		middleTxCallbackReceived := false

		for i := 0; i < expectedCallbacks; i++ {
			select {
			case status := <-callbackReceivedChan:
				if status.Txid == middleTx.TxID().String() {
					require.Equal(t, StatusMined, status.TxStatus)
					middleTxCallbackReceived = true
				} else if status.Txid == tx.TxID().String() {
					require.Equal(t, StatusMined, status.TxStatus)
					lastTxCallbackReceived = true
				} else {
					t.Fatalf("received unknown status for txid: %s", status.Txid)
				}
			case err := <-callbackErrChan:
				t.Fatalf("callback received - failed to parse callback %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("callback exceeded timeout")
			}
		}

		require.Equal(t, true, lastTxCallbackReceived)
		require.Equal(t, true, middleTxCallbackReceived)
	})
}

func TestBeef_Fail(t *testing.T) {
	testCases := []struct {
		name                 string
		beefStr              string
		expectedErrCode      int
		expectedErrMsgDetail string
		expectedErrTxID      string
	}{
		{
			name:                 "invalid beef - transaction with no fees",
			beefStr:              "0100beef01fe4e6d0c001002fd9c67028ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160cfd9d6700db1332728830a58c83a5970dcd111a575a585b43b0492361ea8082f41668f8bd01fdcf3300e568706954aae516ef6df7b5db7828771a1f3fcf1b6d65389ec8be8c46057a3c01fde6190001a6028d13cc988f55c8765e3ffcdcfc7d5185a8ebd68709c0adbe37b528557b01fdf20c001cc64f09a217e1971cabe751b925f246e3c2a8e145c49be7b831eaea3e064d7501fd7806009ccf122626a20cdb054877ef3f8ae2d0503bb7a8704fdb6295b3001b5e8876a101fd3d0300aeea966733175ff60b55bc77edcb83c0fce3453329f51195e5cbc7a874ee47ad01fd9f0100f67f50b53d73ffd6e84c02ee1903074b9a5b2ac64c508f7f26349b73cca9d7e901ce006ce74c7beed0c61c50dda8b578f0c0dc5a393e1f8758af2fb65edf483afcaa68016600e32475e17bdd141d62524d0005989dd1db6ca92c6af70791b0e4802be4c5c8c1013200b88162f494f26cc3a1a4a7dcf2829a295064e93b3dbb2f72e21a73522869277a011800a938d3f80dd25b6a3a80e450403bf7d62a1068e2e4b13f0656c83f764c55bb77010d006feac6e4fea41c37c508b5bfdc00d582f6e462e6754b338c95b448df37bd342c010700bf5448356be23b2b9afe53d00cee047065bbc16d0bbcc5f80aa8c1b509e45678010200c2e37431a437ee311a737aecd3caae1213db353847f33792fd539e380bdb4d440100005d5aef298770e2702448af2ce014f8bfcded5896df5006a44b5f1b6020007aeb01010091484f513003fcdb25f336b9b56dafcb05fbc739593ab573a2c6516b344ca5320201000000027b0a1b12c7c9e48015e78d3a08a4d62e439387df7e0d7a810ebd4af37661daaa000000006a47304402207d972759afba7c0ffa6cfbbf39a31c2aeede1dae28d8841db56c6dd1197d56a20220076a390948c235ba8e72b8e43a7b4d4119f1a81a77032aa6e7b7a51be5e13845412103f78ec31cf94ca8d75fb1333ad9fc884e2d489422034a1efc9d66a3b72eddca0fffffffff7f36874f858fb43ffcf4f9e3047825619bad0e92d4b9ad4ba5111d1101cbddfe010000006a473044022043f048043d56eb6f75024808b78f18808b7ab45609e4c4c319e3a27f8246fc3002204b67766b62f58bf6f30ea608eaba76b8524ed49f67a90f80ac08a9b96a6922cd41210254a583c1c51a06e10fab79ddf922915da5f5c1791ef87739f40cb68638397248ffffffff03e8030000000000001976a914b08f70bc5010fb026de018f19e7792385a146b4a88acf3010000000000001976a9147d48635f889372c3da12d75ce246c59f4ab907ed88acf7000000000000001976a914b8fbd58685b6920d8f9a8f1b274d8696708b51b088ac00000000010001000000018ae36502fdc82837319362c488fb9cb978e064daf600bbfc48389663fc5c160c000000006b483045022100e47fbd96b59e2c22be273dcacea74a4be568b3e61da7eddddb6ce43d459c4cf202201a580f3d9442d5dce3f2ced03256ca147bcd230975a6067954e22415715f4490412102b0c8980f5d2cab77c92c68ac46442feba163a9d306913f6a34911fc618c3c4e7ffffffff0188130000000000001976a9148a8c4546a95e6fc8d18076a9980d59fd882b4e6988ac0000000000",
			expectedErrCode:      465,
			expectedErrMsgDetail: "Fees are insufficient",
			expectedErrTxID:      "8184849de6af441c7de088428073c2a9131b08f7d878d9f49c3faf6d941eb168",
		},
		{
			name:                 "invalid beef - a parent transaction is not paying any fees and has not been mined",
			beefStr:              "0100beef01fe04ef0c000b02a002f6a80f2499f95740f59ebdc571176b98e86069155e77e82611b47bd5c25df858a1009d9ba131668c3eed6bf3e8f267899f144552594c90d80d7af50217db85dbc13c015100ef33d13a7ad910d0aca1e55d860b8d20df39c5574f00fba0e8547d3400d708c20129003485f53e6cd6888d7278f94b0d2e0503f54a54902775e594dc5c07ce336e9ffa0115001d979c682a27b17d5b7b451c2abad5674e343bf13d5207f52cc1cf714412bd22010b0085439ca62dd73d63cd1eadfffdc8d0f8be22b0e6a6f76ec58b321e1c923f9ab00104009a75a8096fec85ce509c7f7b9d24373add68bfd2ba7c849af7c8505d173937cf0103000aca68c2935ec67ddd5a6479c3cd436af4afd439e8b0da8d91a269db648c391f0100009eaba9fab53f84b4b2f5682c5f12e8f7b25b3e42f25783f496bdf70bd490105e0101000aa97c84b328ada74b6730fbd1ae2f47830f7e4c423f7aa7301c294c6de06de2010100b88a9a5cf074d82b74a04c46dc5220ad225c3698639c712c35f31c100b4b3e3c010100e5cbdafb9927402a0b1a9272c726f20c393a44b988bf57d2f8dd0b368aeede770402000000012742e88dfccfb3725cf712731a3f1862f90ecffce335a3e80d631ae53cf7e917020000006a47304402200ad60196c94fc663b348c9648fa09d2863fbd6290db7713eb0b62fb65ac9e807022047b837347caf8e6cdb84e043c39f0fb00077d4a00b11c32de2f0082c27d6773d41210332f4393bcc413ef86d1fc52bf77666d209e5c921aabb310689f4c8aec1177294ffffffff037c660000000000001976a91416d68175be77ca6882f41aa186d58f07bd4db6db88ac75330100000000001976a914547c0e5c36cf182f71d00aafb7a6f373921f7f4b88ac9f5e7805000000001976a9148149d5c6491692c3e7dadea4528621ecc16116a588ac0000000001000200000001f6a80f2499f95740f59ebdc571176b98e86069155e77e82611b47bd5c25df858020000006a473044022025ac85f4faa096b5004d965235143910500b0642eb10410ca386a86fefd9da58022053f595c645c28081d430125c2c0034d8613dad22a52c08c16892a8915a78844141210257164edc9fc13707d4224e07dcaadaeb9a18892d6c1e17a6164ff234a3eb0511ffffffff037c660000000000001976a9146f61debe4bb8c0ab04815278c6c34917133f7f8f88ac75330100000000001976a9140331fb4cd4f6e5ef3d01f9d03b9e7eea3fba47b788ac14c47605000000001976a9148e54d097cd7c27469e93732ca1a4a1cd1f7fe61188ac0000000000010000000135d76fa5478f9e856970c6b0ab37834ae791af153fdc16de3f88dc90df4fec70000000006b483045022100bcdd2462b2e0959450034a5dd3ea4c8fa25e1ed344eff565be65ecc196c34dc6022067e50168f47b657496f485674a973b95963c1b061627dcd1145a83df9b7682ea4121032fbe910b6a1bd73f6e71c521df96f52c0f435f602bc32838483c657f7ac9dc49ffffffff017c660000000000001976a9146f61debe4bb8c0ab04815278c6c34917133f7f8f88ac00000000000100000001e242501cd4a687c379d3dfeaed8b9d8a1815f69e324fba1bda738b522361f35e000000006a473044022038458481ee9c894e1ca9d62890eb3321b29c9ce20f57b02e42eeac56bdc0d7ef02203cf36cd63077193756459136236234fad1a1ae2835fdfc96cc7ab727283aba5f4121032fbe910b6a1bd73f6e71c521df96f52c0f435f602bc32838483c657f7ac9dc49ffffffff017a660000000000001976a9146f61debe4bb8c0ab04815278c6c34917133f7f8f88ac0000000000",
			expectedErrCode:      465,
			expectedErrMsgDetail: "Fees are insufficient",
			expectedErrTxID:      "5ef36123528b73da1bba4f329ef615188a9d8bedeadfd379c387a6d41c5042e2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postRequest[ErrorFee](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: tc.beefStr}), nil, tc.expectedErrCode)
			require.Equal(t, tc.expectedErrMsgDetail, resp.Detail)
			require.Equal(t, tc.expectedErrTxID, resp.Txid)
		})
	}
}

func prepareBeef(t *testing.T, inputTxID, blockHash, fromAddress, toAddress, privateKey string) (string, *sdkTx.Transaction, *sdkTx.Transaction, int) {
	expectedCallbacks := 0

	rawTx := node_client.GetRawTx(t, bitcoind, inputTxID)
	t.Logf("rawTx: %+v", rawTx)
	require.Equal(t, blockHash, rawTx.BlockHash, "block hash mismatch")

	blockData := node_client.GetBlockDataByBlockHash(t, bitcoind, blockHash)
	t.Logf("blockdata: %+v", blockData)

	merkleHashes, txIndex := prepareMerkleHashesAndTxIndex(t, blockData.Txs, inputTxID)
	merkleTree := bc.BuildMerkleTreeStoreChainHash(merkleHashes)
	bump, err := bc.NewBUMPFromMerkleTreeAndIndex(blockData.Height, merkleTree, txIndex)
	require.NoError(t, err, "error creating BUMP from merkle hashes")

	merkleRootFromBump, err := bump.CalculateRootGivenTxid(inputTxID)
	require.NoError(t, err, "error calculating merkle root from bump")
	t.Logf("merkleroot from bump: %s", merkleRootFromBump)
	require.Equal(t, blockData.MerkleRoot, merkleRootFromBump, "merkle roots mismatch")

	utxos := node_client.GetUtxos(t, bitcoind, fromAddress)
	require.True(t, len(utxos) > 0, "No UTXOs available for the address")

	middleAddress, middlePrivKey := node_client.GetNewWalletAddress(t, bitcoind)
	middleTx, err := node_client.CreateTx(privateKey, middleAddress, utxos[0])
	require.NoError(t, err, "could not create middle tx for beef")
	t.Logf("middle tx created, hex: %s, txid: %s", middleTx.String(), middleTx.TxID())
	expectedCallbacks++

	middleUtxo := node_client.UnspentOutput{
		Txid:         middleTx.TxID().String(),
		Vout:         0,
		ScriptPubKey: middleTx.Outputs[0].LockingScriptHex(),
		Amount:       float64(middleTx.Outputs[0].Satoshis) / 1e8, // satoshis to BSV
	}

	tx, err := node_client.CreateTx(middlePrivKey, toAddress, middleUtxo)
	require.NoError(t, err, "could not create tx")
	t.Logf("tx created, hex: %s, txid: %s", tx.String(), tx.TxID())
	expectedCallbacks++

	beef := buildBeefString(t, rawTx.Hex, bump, middleTx, tx)
	t.Logf("beef created, hex: %s", beef)

	return beef, middleTx, tx, expectedCallbacks
}

func prepareMerkleHashesAndTxIndex(t *testing.T, txs []string, txID string) ([]*chainhash.Hash, uint64) {
	var merkleHashes []*chainhash.Hash
	var txIndex uint64

	for i, txid := range txs {
		if txid == txID {
			ind, err := safe.ToUint64(i)
			require.NoError(t, err)
			txIndex = ind
		}
		h, err := chainhash.NewHashFromStr(txid)
		require.NoError(t, err)

		merkleHashes = append(merkleHashes, h)
	}

	return merkleHashes, txIndex
}

func buildBeefString(t *testing.T, inputTxHex string, bump *bc.BUMP, middleTx, newTx *sdkTx.Transaction) string {
	versionMarker := "0100beef"
	nBumps := "01"
	bumpData, err := bump.String()
	require.NoError(t, err, "could not get bump string")

	nTransactions := "03"
	rawParentTx := inputTxHex
	parentHasBump := "01"
	parentBumpIndex := "00"
	middleRawTx := middleTx.String()
	middleHasBump := "00"
	rawTx := newTx.String()
	hasBump := "00"

	beef := versionMarker +
		nBumps +
		bumpData +
		nTransactions +
		rawParentTx +
		parentHasBump +
		parentBumpIndex +
		middleRawTx +
		middleHasBump +
		rawTx +
		hasBump

	return beef
}
