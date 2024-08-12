package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitcoinsv/bsvd/bsvec"
	"github.com/bitcoinsv/bsvutil"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/stretchr/testify/require"
)

func TestBatchChainedTxs(t *testing.T) {

	t.Run("submit batch of chained transactions", func(t *testing.T) {
		address, privateKey := fundNewWallet(t)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		txs, err := createTxChain(privateKey, utxos[0], 30)
		require.NoError(t, err)

		request := make([]TransactionRequest, len(txs))
		for i, tx := range txs {
			request[i] = TransactionRequest{
				RawTx: hex.EncodeToString(tx.ExtendedBytes()),
			}
		}

		// Send POST request
		t.Logf("submitting batch of %d chained txs", len(txs))
		resp := postRequest[ResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		for _, txResponse := range resp {
			require.Equal(t, Status_SEEN_ON_NETWORK, txResponse.TxStatus)
		}

		time.Sleep(1 * time.Second)

		// repeat request to ensure response remains the same
		t.Logf("re-submitting batch of %d chained txs", len(txs))
		resp = postRequest[ResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		for _, txResponse := range resp {
			require.Equal(t, Status_SEEN_ON_NETWORK, txResponse.TxStatus)
		}
	})
}

func createTxChain(privateKey string, utxo0 NodeUnspentUtxo, length int) ([]*bt.Tx, error) {
	batch := make([]*bt.Tx, length)

	utxoTxID := utxo0.Txid
	utxoVout := uint32(utxo0.Vout)
	utxoSatoshis := uint64(utxo0.Amount * 1e8)
	utxoScript := utxo0.ScriptPubKey
	utxoAddress := utxo0.Address

	for i := 0; i < length; i++ {
		tx := bt.NewTx()

		err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
		if err != nil {
			return nil, fmt.Errorf("failed adding input: %v", err)
		}

		amountToSend := utxoSatoshis - feeSat

		recipientScript, err := bscript.NewP2PKHFromAddress(utxoAddress)
		if err != nil {
			return nil, fmt.Errorf("failed converting address to script: %v", err)
		}

		err = tx.PayTo(recipientScript, amountToSend)
		if err != nil {
			return nil, fmt.Errorf("failed adding output: %v", err)
		}

		// Sign the input

		wif, err := bsvutil.DecodeWIF(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode WIF: %v", err)
		}

		// Extract raw private key bytes directly from the WIF structure
		privateKeyDecoded := wif.PrivKey.Serialize()

		pk, _ := bec.PrivKeyFromBytes(bsvec.S256(), privateKeyDecoded)
		unlockerGetter := unlocker.Getter{PrivateKey: pk}
		err = tx.FillAllInputs(context.Background(), &unlockerGetter)
		if err != nil {
			return nil, fmt.Errorf("sign failed: %v", err)
		}

		batch[i] = tx

		utxoTxID = tx.TxID()
		utxoVout = 0
		utxoSatoshis = amountToSend
		utxoScript = utxo0.ScriptPubKey
	}

	return batch, nil
}
