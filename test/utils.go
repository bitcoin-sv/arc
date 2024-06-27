package test

import (
	"context"
	"fmt"
	"math/rand"
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

type NodeUnspentUtxo struct {
	Txid          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Account       string  `json:"account"`
	ScriptPubKey  string  `json:"scriptPubKey"`
	Amount        float64 `json:"amount"`
	Confirmations int     `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
	Solvable      bool    `json:"solvable"`
	Safe          bool    `json:"safe"`
}

type RawTransaction struct {
	Hex       string `json:"hex"`
	BlockHash string `json:"blockhash,omitempty"`
}

type BlockData struct {
	Height     uint64   `json:"height"`
	Txs        []string `json:"txs"`
	MerkleRoot string   `json:"merkleroot"`
}

func getNewWalletAddress(t *testing.T) (address, privateKey string) {
	address, err := bitcoind.GetNewAddress()
	require.NoError(t, err)
	t.Logf("new address: %s", address)

	privateKey, err = bitcoind.DumpPrivKey(address)
	require.NoError(t, err)
	t.Logf("new private key: %s", privateKey)

	accountName := "test-account"
	err = bitcoind.SetAccount(address, accountName)
	require.NoError(t, err)

	t.Logf("account %s created", accountName)

	return
}

func sendToAddress(t *testing.T, address string, bsv float64) (txID string) {
	t.Helper()

	txID, err := bitcoind.SendToAddress(address, bsv)
	require.NoError(t, err)

	t.Logf("sent %f to %s: %s", bsv, address, txID)

	return
}

func generate(t *testing.T, amount uint64) string {
	t.Helper()

	// run command instead
	blockHash := execCommandGenerate(t, amount)

	time.Sleep(5 * time.Second)

	t.Logf(
		"generated %d block(s): block hash: %s",
		amount,
		blockHash,
	)

	time.Sleep(1 * time.Second)

	return blockHash
}

func execCommandGenerate(t *testing.T, amount uint64) string {
	t.Helper()
	t.Logf("Amount to generate: %d", amount)

	hashes, err := bitcoind.Generate(float64(amount))
	require.NoError(t, err)

	return hashes[len(hashes)-1]
}

func getUtxos(t *testing.T, address string) []NodeUnspentUtxo {
	t.Helper()

	data, err := bitcoind.ListUnspent([]string{address})
	require.NoError(t, err)

	result := make([]NodeUnspentUtxo, len(data))

	for index, utxo := range data {
		t.Logf("UTXO Txid: %s, Amount: %f, Address: %s\n", utxo.TXID, utxo.Amount, utxo.Address)
		result[index] = NodeUnspentUtxo{
			Txid:          utxo.TXID,
			Vout:          utxo.Vout,
			Address:       utxo.Address,
			ScriptPubKey:  utxo.ScriptPubKey,
			Amount:        utxo.Amount,
			Confirmations: int(utxo.Confirmations),
		}
	}

	return result
}

func getBlockRootByHeight(t *testing.T, blockHeight int) string {
	t.Helper()
	block, err := bitcoind.GetBlockByHeight(blockHeight)
	require.NoError(t, err)

	return block.MerkleRoot
}

func getRawTx(t *testing.T, txID string) RawTransaction {
	t.Helper()

	rawTx, err := bitcoind.GetRawTransaction(txID)
	require.NoError(t, err)

	return RawTransaction{
		Hex:       rawTx.Hex,
		BlockHash: rawTx.BlockHash,
	}
}

func getBlockDataByBlockHash(t *testing.T, blockHash string) BlockData {
	t.Helper()

	block, err := bitcoind.GetBlock(blockHash)
	require.NoError(t, err)

	return BlockData{
		Height:     block.Height,
		Txs:        block.Tx,
		MerkleRoot: block.MerkleRoot,
	}
}

func createTx(privateKey string, address string, utxo NodeUnspentUtxo, fee ...uint64) (*bt.Tx, error) {
	tx := bt.NewTx()

	// Add an input using the first UTXO
	utxoTxID := utxo.Txid
	utxoVout := utxo.Vout
	utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
	utxoScript := utxo.ScriptPubKey

	err := tx.From(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
	if err != nil {
		return nil, fmt.Errorf("failed adding input: %v", err)
	}

	// Add an output to the address you've previously created
	recipientAddress := address

	var feeValue uint64
	if len(fee) > 0 {
		feeValue = fee[0]
	} else {
		feeValue = 20 // Set your default fee value here
	}
	amountToSend := utxoSatoshis - feeValue

	recipientScript, err := bscript.NewP2PKHFromAddress(recipientAddress)
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

	return tx, nil
}

func generateRandomString(length int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
