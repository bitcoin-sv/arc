package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	ec "github.com/bitcoin-sv/go-sdk/primitives/ec"
	"github.com/stretchr/testify/require"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
	"github.com/bitcoinsv/bsvutil"
	"github.com/stretchr/testify/assert"
)

func TestBatchChainedTxs(t *testing.T) {

	t.Run("submit batch of chained transactions", func(t *testing.T) {
		address, privateKey := fundNewWallet(t)

		utxos := getUtxos(t, address)
		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

		txs, err := createTxChain(privateKey, utxos[0], 20)
		require.NoError(t, err)

		request := make([]TransactionRequest, len(txs))
		for i, tx := range txs {
			rawTx, err := tx.EFHex()
			require.NoError(t, err)
			request[i] = TransactionRequest{
				RawTx: rawTx,
			}
		}

		// Send POST request
		t.Logf("submitting batch of %d chained txs", len(txs))
		resp := postRequest[TransactionResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		hasFailed := false
		for i, txResponse := range resp {
			if !assert.Equal(t, Status_SEEN_ON_NETWORK, txResponse.TxStatus, fmt.Sprintf("index: %d", i)) {
				hasFailed = true
			}
		}
		if hasFailed {
			t.FailNow()
		}

		time.Sleep(1 * time.Second)

		// repeat request to ensure response remains the same
		t.Logf("re-submitting batch of %d chained txs", len(txs))
		resp = postRequest[TransactionResponseBatch](t, arcEndpointV1Txs, createPayload(t, request), nil, http.StatusOK)
		for i, txResponse := range resp {
			if !assert.Equal(t, Status_SEEN_ON_NETWORK, txResponse.TxStatus, fmt.Sprintf("index: %d", i)) {
				hasFailed = true
			}
		}
		if hasFailed {
			t.FailNow()
		}
	})
}

func createTxChain(privateKey string, utxo0 NodeUnspentUtxo, length int) ([]*sdkTx.Transaction, error) {
	batch := make([]*sdkTx.Transaction, length)

	utxoTxID := utxo0.Txid
	utxoVout := uint32(utxo0.Vout)
	utxoSatoshis := uint64(utxo0.Amount * 1e8)
	utxoScript := utxo0.ScriptPubKey
	utxoAddress := utxo0.Address

	for i := 0; i < length; i++ {
		tx := sdkTx.NewTransaction()

		utxo, err := sdkTx.NewUTXO(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
		if err != nil {
			return nil, fmt.Errorf("failed creating UTXO: %v", err)
		}

		err = tx.AddInputsFromUTXOs(utxo)
		if err != nil {
			return nil, fmt.Errorf("failed adding input: %v", err)
		}

		amountToSend := utxoSatoshis - feeSat

		err = tx.PayToAddress(utxoAddress, amountToSend)
		if err != nil {
			return nil, fmt.Errorf("failed to pay to address: %v", err)
		}

		// Sign the input
		wif, err := bsvutil.DecodeWIF(privateKey)
		if err != nil {
			return nil, err
		}

		privateKeyDecoded := wif.PrivKey.Serialize()
		pk, _ := ec.PrivateKeyFromBytes(privateKeyDecoded)

		unlockingScriptTemplate, err := p2pkh.Unlock(pk, nil)
		if err != nil {
			return nil, err
		}

		for _, input := range tx.Inputs {
			input.UnlockingScriptTemplate = unlockingScriptTemplate
		}

		err = tx.Sign()
		if err != nil {
			return nil, err
		}

		batch[i] = tx

		utxoTxID = tx.TxID()
		utxoVout = 0
		utxoSatoshis = amountToSend
		utxoScript = utxo0.ScriptPubKey
	}

	return batch, nil
}
