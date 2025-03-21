package node_client

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	ec "github.com/bitcoin-sv/go-sdk/primitives/ec"
	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoin-sv/go-sdk/transaction/template/p2pkh"
	"github.com/bitcoinsv/bsvutil"
	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/require"
)

type UnspentOutput struct {
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

type RPCRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int64       `json:"id"`
	JSONRpc string      `json:"jsonrpc"`
}

type RPCResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Err    interface{}     `json:"error"`
}

func GetNewWalletAddress(t *testing.T, bitcoind *bitcoin.Bitcoind) (address, privateKey string) {
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

func SendToAddress(t *testing.T, bitcoind *bitcoin.Bitcoind, address string, bsv float64) (txID string) {
	t.Helper()

	txID, err := bitcoind.SendToAddress(address, bsv)
	require.NoError(t, err)

	t.Logf("sent %f to %s: %s", bsv, address, txID)

	return
}

func Generate(t *testing.T, bitcoind *bitcoin.Bitcoind, amount uint64) string {
	t.Helper()

	// run command instead
	blockHash := ExecCommandGenerate(t, bitcoind, amount)
	time.Sleep(5 * time.Second)

	t.Logf(
		"generated %d block(s): block hash: %s",
		amount,
		blockHash,
	)

	return blockHash
}

func ExecCommandGenerate(t *testing.T, bitcoind *bitcoin.Bitcoind, amount uint64) string {
	t.Helper()
	t.Logf("Amount to generate: %d", amount)

	hashes, err := bitcoind.Generate(float64(amount))
	require.NoError(t, err)

	return hashes[len(hashes)-1]
}

func InvalidateBlock(t *testing.T, bitcoind *bitcoin.Bitcoind, invHash string) {
	t.Helper()

	err := bitcoind.InvalidateBlock(invHash)
	require.NoError(t, err)
}

func GetUtxos(t *testing.T, bitcoind *bitcoin.Bitcoind, address string) []UnspentOutput {
	t.Helper()

	data, err := bitcoind.ListUnspent([]string{address})
	require.NoError(t, err)

	result := make([]UnspentOutput, len(data))

	for index, utxo := range data {
		t.Logf("UTXO Txid: %s, Amount: %f, Address: %s\n", utxo.TXID, utxo.Amount, utxo.Address)
		result[index] = UnspentOutput{
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

func CreateTxChain(privateKey string, utxo0 UnspentOutput, length int) ([]*sdkTx.Transaction, error) {
	batch := make([]*sdkTx.Transaction, length)
	const feeSat = 10
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

		utxoTxID = tx.TxID().String()
		utxoVout = 0
		utxoSatoshis = amountToSend
		utxoScript = utxo0.ScriptPubKey
	}

	return batch, nil
}

func FundNewWallet(t *testing.T, bitcoind *bitcoin.Bitcoind) (addr, privKey string) {
	t.Helper()

	addr, privKey = GetNewWalletAddress(t, bitcoind)
	SendToAddress(t, bitcoind, addr, 0.001)
	// mine a block with the transaction from above
	Generate(t, bitcoind, 1)

	return
}

func GetBlockRootByHeight(t *testing.T, bitcoind *bitcoin.Bitcoind, blockHeight int) string {
	t.Helper()
	block, err := bitcoind.GetBlockByHeight(blockHeight)
	require.NoError(t, err)

	return block.MerkleRoot
}

func GetRawTx(t *testing.T, bitcoind *bitcoin.Bitcoind, txID string) RawTransaction {
	t.Helper()

	rawTx, err := bitcoind.GetRawTransaction(txID)
	require.NoError(t, err)

	return RawTransaction{
		Hex:       rawTx.Hex,
		BlockHash: rawTx.BlockHash,
	}
}

func GetBlockDataByBlockHash(t *testing.T, bitcoind *bitcoin.Bitcoind, blockHash string) BlockData {
	t.Helper()

	block, err := bitcoind.GetBlock(blockHash)
	require.NoError(t, err)

	return BlockData{
		Height:     block.Height,
		Txs:        block.Tx,
		MerkleRoot: block.MerkleRoot,
	}
}

func CreateTx(privateKey string, address string, utxo UnspentOutput, fee ...uint64) (*sdkTx.Transaction, error) {
	return CreateTxFrom(privateKey, address, []UnspentOutput{utxo}, fee...)
}

func CreateTxFrom(privateKey string, address string, utxos []UnspentOutput, fee ...uint64) (*sdkTx.Transaction, error) {
	tx := sdkTx.NewTransaction()

	// Add an input using the UTXOs
	for _, utxo := range utxos {
		utxoTxID := utxo.Txid
		utxoVout := utxo.Vout
		utxoSatoshis := uint64(utxo.Amount * 1e8) // Convert BTC to satoshis
		utxoScript := utxo.ScriptPubKey

		u, err := sdkTx.NewUTXO(utxoTxID, utxoVout, utxoScript, utxoSatoshis)
		if err != nil {
			return nil, fmt.Errorf("failed creating UTXO: %v", err)
		}
		err = tx.AddInputsFromUTXOs(u)
		if err != nil {
			return nil, fmt.Errorf("failed adding input: %v", err)
		}
	}
	// Add an output to the address you've previously created
	recipientAddress := address

	var feeValue uint64
	if len(fee) > 0 {
		feeValue = fee[0]
	} else {
		feeValue = 20 // Set your default fee value here
	}

	amount, err := tx.TotalInputSatoshis()
	if err != nil {
		return nil, err
	}
	amountToSend := amount - feeValue

	err = tx.PayToAddress(recipientAddress, amountToSend)
	if err != nil {
		return nil, fmt.Errorf("failed to pay to address: %v", err)
	}

	// Sign the input
	wif, err := bsvutil.DecodeWIF(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WIF: %v", err)
	}

	// Extract raw private key bytes directly from the WIF structure
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

	return tx, nil
}
