package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ordishs/go-bitcoin"
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

var (
	lastNodeBlockHeight uint64
	initialBlockHeight  uint64
	bitcoind            *bitcoin.Bitcoind
)

func init() {

	var err error
	bitcoind, err = bitcoin.New("node2", 18332, "bitcoin", "bitcoin", false)
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
		lastNodeBlockHeight = uint64(info.Blocks + 100)
		initialBlockHeight = lastNodeBlockHeight
		time.Sleep(5 * time.Second)
	}

	info, err = bitcoind.GetInfo()
	if err != nil {
		log.Fatalln(err)
	}

	initialBlockHeight = uint64(info.Blocks)
	lastNodeBlockHeight = uint64(info.Blocks)

}

func getNewWalletAddress(t *testing.T) (address, privateKey string) {

	fmt.Println("this is getting called")
	t.Helper()
	cmd := exec.Command("bash", "-c", "bitcoin-cli -rpcconnect=node2 -rpcport=18332 -rpcuser=bitcoin -rpcpassword=bitcoin getnewaddress")
	fmt.Println("Executing:", cmd.String())
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}


	//scripthash
	
	fmt.Println("Result: " + out.String())
	require.NoError(t, err)
	address = strings.TrimSpace(out.String())

	// scripthash, err = utils.AddressToScriptHash(address, false)
	require.NoError(t, err)

	// Dump the private key of the wallet
	dumpCmd := exec.Command("bash", "-c", fmt.Sprintf("bitcoin-cli -rpcconnect=node2 -rpcport=18332 -rpcuser=bitcoin -rpcpassword=bitcoin dumpprivkey %s", address))
	var dumpOut bytes.Buffer
	dumpCmd.Stdout = &dumpOut
	err = dumpCmd.Run()
	require.NoError(t, err)

	privateKey = strings.TrimSpace(dumpOut.String())

	// Create an account alias for the wallet address
	aliasCmd := exec.Command("bash", "-c", fmt.Sprintf("bitcoin-cli -rpcconnect=node2 -rpcport=18332 -rpcuser=bitcoin -rpcpassword=bitcoin setaccount %s %s", address, address))
	var aliasOut bytes.Buffer
	aliasCmd.Stdout = &aliasOut
	err = aliasCmd.Run()
	require.NoError(t, err)

	t.Logf("new wallet created: %s", address)

	return
}

func sendToAddress(t *testing.T, address string, bsv float64) (txID string) {
	t.Helper()

	cmdStr := fmt.Sprintf("bitcoin-cli -rpcconnect=node2 -rpcport=18332 -rpcuser=bitcoin -rpcpassword=bitcoin sendtoaddress %s %f", address, bsv)
	fmt.Println("Executing command:", cmdStr) // Log the exact command being executed

	cmd := exec.Command("bash", "-c", cmdStr)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr // Capture standard error

	err := cmd.Run()

	// Print the output and error messages
	fmt.Println("Output:", out.String())
	fmt.Println("Error:", stderr.String())

	require.NoError(t, err)
	txID = strings.TrimSpace(out.String())

	t.Logf("sent %f to %s: %s", bsv, address, txID)

	return
}

func generate(t *testing.T, amount uint64, address string) string {
	t.Helper()

	// run command instead
	blockHash := execCommandGenerate(t, amount, address)

	time.Sleep(5 * time.Second)

	t.Logf(
		"generated %d block(s): block hash: %s",
		amount,
		blockHash,
	)

	time.Sleep(1 * time.Second)

	return blockHash
}

func execCommandGenerate(t *testing.T, amount uint64, address string) string {
	t.Helper()
	fmt.Println("Amount to generate:", amount)

	cmd := exec.Command(
		"bitcoin-cli",
		"-rpcconnect=node2",
		"-rpcport=18332",
		"-rpcuser=bitcoin",
		"-rpcpassword=bitcoin",
		"generatetoaddress",
		fmt.Sprintf("%d", amount),
		address,
	)
	// Rest of your function...
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Command failed with error:", err)
		fmt.Println("Stderr:", stderr.String())
		t.FailNow() // or return, based on your requirement
	}
	require.NoError(t, err)

	var hashes []string

	err = json.Unmarshal(stdout.Bytes(), &hashes)
	require.NoError(t, err)

	return hashes[len(hashes)-1]
}

func getUnspentUtxos(t *testing.T, address string) []NodeUnspentUtxo {
	t.Helper()

	// Run the command
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`bitcoin-cli -rpcconnect=node2 -rpcport=18332 -rpcuser=bitcoin -rpcpassword=bitcoin listunspent 1 9999999 '["%s"]'`, address))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	require.NoError(t, err)

	// Parse the JSON output
	var data []NodeUnspentUtxo
	err = json.Unmarshal(out.Bytes(), &data)
	require.NoError(t, err)

	for _, utxo := range data {
		fmt.Printf("UTXO Txid: %s, Amount: %f, Address: %s\n", utxo.Txid, utxo.Amount, utxo.Address)
	}

	// Create a map for fast lookup
	dataMap := make(map[string]NodeUnspentUtxo)
	for _, item := range data {
		dataMap[item.Txid] = item
	}

	return data

}
