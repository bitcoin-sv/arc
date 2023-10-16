package test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/ordishs/go-bitcoin"
	"github.com/stretchr/testify/require"
)

const (
	host     = "node2"
	port     = 18332
	user     = "bitcoin"
	password = "bitcoin"
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

var (
	bitcoind *bitcoin.Bitcoind
)

func init() {

	var err error
	bitcoind, err = bitcoin.New(host, port, user, password, false)
	if err != nil {
		log.Fatalln("Failed to create bitcoind instance:", err)
	}

	info, err := bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

	if info.Blocks < 100 {
		_, err = bitcoind.Generate(100)
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(5 * time.Second)
	}

	info, err = bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

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
	fmt.Println("Amount to generate:", amount)

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
		fmt.Printf("UTXO Txid: %s, Amount: %f, Address: %s\n", utxo.TXID, utxo.Amount, utxo.Address)
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
