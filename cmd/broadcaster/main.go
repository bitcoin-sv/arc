package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/lib"
	"github.com/TAAL-GmbH/arc/lib/fees"
	"github.com/TAAL-GmbH/arc/lib/keyset"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/tracing"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bk/base58"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var stdFee = &bt.Fee{
	FeeType: bt.FeeTypeStandard,
	MiningFee: bt.FeeUnit{
		Satoshis: 50,
		Bytes:    1000,
	},
	RelayFee: bt.FeeUnit{
		Satoshis: 0,
		Bytes:    1000,
	},
}

var dataFee = &bt.Fee{
	FeeType: bt.FeeTypeData,
	MiningFee: bt.FeeUnit{
		Satoshis: 50,
		Bytes:    1000,
	},
	RelayFee: bt.FeeUnit{
		Satoshis: 0,
		Bytes:    1000,
	},
}

var (
	isDryRun  = false
	isRegtest = true
)

var fq = bt.NewFeeQuote()

func main() {
	sendOnChannel := flag.Int("buffer", 0, "whether to send transactions over a buffered channel and how big the buffer should be")
	dryRun := flag.Bool("dryrun", false, "whether to not send transactions to metamorph")
	consolidate := flag.Bool("consolidate", false, "whether to consolidate all output transactions back into the original")
	useKey := flag.Bool("key", false, "private key to use for funding transactions")
	keyFile := flag.String("keyfile", "", "private key from file (arc.key) to use for funding transactions")
	flag.Parse()

	args := flag.Args()
	fmt.Println(args)
	if len(args) != 1 {
		fmt.Println("usage: broadcaster [-buffer=<number of buffered channels>] [-prefund] [-dryrun] [-key] [-keyfile=<key file>] <number of transactions to send>")
		return
	}

	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	if dryRun != nil && *dryRun {
		isDryRun = true
	}

	var err error
	var xpriv string
	if useKey != nil && *useKey {
		fmt.Print("Enter xpriv: ")
		reader := bufio.NewReader(os.Stdin)
		inputKey, err := reader.ReadString('\n')
		if err != nil {
			panic("An error occurred while reading input. Please try again:" + err.Error())
		}
		xpriv = strings.TrimSpace(inputKey)
	}

	sendNrOfTransactions, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		panic(err)
	}
	if sendNrOfTransactions == 0 {
		sendNrOfTransactions = 1
	}

	ctx := context.Background()

	//
	// Create the metamorph client
	//
	var client metamorph_api.MetaMorphAPIClient
	if isDryRun {
		client = lib.NewDryRunClient()
	} else {
		addresses, _ := gocore.Config().Get("metamorphAddresses") //, "localhost:8000")

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
		}

		var cc *grpc.ClientConn
		cc, err = grpc.DialContext(ctx, addresses, tracing.AddGRPCDialOptions(opts)...)
		if err != nil {
			panic(fmt.Errorf("DIALCONTEXT: %v", err))
		}

		client = metamorph_api.NewMetaMorphAPIClient(cc)
	}

	//
	// Set the private key to use for funding transactions
	//
	var fundingKeySet *keyset.KeySet
	var receivingKeySet *keyset.KeySet

	if xpriv != "" || *keyFile != "" {
		if xpriv == "" {
			var extendedBytes []byte
			extendedBytes, err = os.ReadFile(*keyFile)
			if err != nil {
				if os.IsNotExist(err) {
					panic("arc.key not found. Please create this file with the xpriv you want to use")
				}
				panic(err.Error())
			}
			xpriv = string(extendedBytes)
		}

		fundingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/0")
		if err != nil {
			panic(err)
		}

		receivingKeySet, err = keyset.NewFromExtendedKeyStr(xpriv, "0/1")
		if err != nil {
			panic(err)
		}

		isRegtest = false
	} else {
		// create random key set
		fundingKeySet, err = keyset.New()
		if err != nil {
			panic(err)
		}
		receivingKeySet = fundingKeySet
	}

	txs := make([]*bt.Tx, sendNrOfTransactions)

	fundingTx := newFundingTransaction(fundingKeySet, receivingKeySet, sendNrOfTransactions)
	log.Printf("funding tx: %s\n", fundingTx.TxID())

	_, err = client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
		RawTx: fundingTx.Bytes(),
	})
	if err != nil {
		panic(err)
	}

	for i := int64(0); i < sendNrOfTransactions; i++ {
		u := &bt.UTXO{
			TxID:          fundingTx.TxIDBytes(),
			Vout:          uint32(i),
			LockingScript: fundingTx.Outputs[i].LockingScript,
			Satoshis:      fundingTx.Outputs[i].Satoshis,
		}
		txs[i], _ = newTransaction(receivingKeySet, u)
		log.Printf("generated tx: %s\n", txs[i].TxID())
	}

	// let the node recover
	time.Sleep(1 * time.Second)

	var wg sync.WaitGroup
	if isDryRun {
		for i, tx := range txs {
			log.Printf("Processing tx %d / %d\n", i+1, len(txs))
			if err = processTransaction(ctx, client, tx); err != nil {
				panic(err)
			}
		}
	} else if sendOnChannel != nil && *sendOnChannel > 0 {
		limit := make(chan bool, *sendOnChannel)
		for i, tx := range txs {
			wg.Add(1)
			limit <- true
			go func(i int, tx *bt.Tx) {
				defer func() {
					wg.Done()
					<-limit
				}()

				log.Printf("Processing tx %d / %d\n", i+1, len(txs))

				if err = processTransaction(ctx, client, tx); err != nil {
					panic(err)
				}
			}(i, tx)
		}
		wg.Wait()
	} else {
		for i, tx := range txs {
			wg.Add(1)
			go func(i int, tx *bt.Tx) {
				defer wg.Done()

				log.Printf("Processing tx %d / %d\n", i+1, len(txs))
				if err = processTransaction(ctx, client, tx); err != nil {
					panic(err)
				}
			}(i, tx)
		}
		wg.Wait()
	}

	if consolidate != nil && *consolidate {
		if err = consolidateOutputsToOriginal(ctx, client, fundingKeySet, receivingKeySet, txs); err != nil {
			panic(err)
		}
	}
}

func consolidateOutputsToOriginal(ctx context.Context, client metamorph_api.MetaMorphAPIClient, privKey *keyset.KeySet, sendToPrivKey *keyset.KeySet, txs []*bt.Tx) error {
	// consolidate all transactions back into the original address
	log.Println("Consolidating all transactions back into original address")
	consolidationAddress := privKey.Address(!isRegtest)
	consolidationTx := bt.NewTx()
	utxos := make([]*bt.UTXO, len(txs))
	totalSatoshis := uint64(0)
	for i, tx := range txs {
		utxos[i] = &bt.UTXO{
			TxID:          tx.TxIDBytes(),
			Vout:          0,
			LockingScript: tx.Outputs[0].LockingScript,
			Satoshis:      tx.Outputs[0].Satoshis,
		}
		totalSatoshis += tx.Outputs[0].Satoshis
	}
	err := consolidationTx.FromUTXOs(utxos...)
	if err != nil {
		return err
	}
	// put half of the satoshis back into the original address
	// the rest will be returned via the change output
	err = consolidationTx.PayToAddress(consolidationAddress, totalSatoshis/2)
	if err != nil {
		return err
	}
	err = consolidationTx.ChangeToAddress(consolidationAddress, fq)
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: sendToPrivKey.PrivateKey}
	if err = consolidationTx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		return err
	}

	if err = processTransaction(ctx, client, consolidationTx); err != nil {
		return err
	}

	return nil
}

func processTransaction(ctx context.Context, client metamorph_api.MetaMorphAPIClient, tx *bt.Tx) error {
	ctxClient, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	res, err := client.PutTransaction(ctxClient, &metamorph_api.TransactionRequest{
		RawTx: tx.Bytes(),
	})
	if err != nil {
		return err
	}
	if res.TimedOut {
		log.Printf("res %s: %#v (TIMEOUT)\n", tx.TxID(), res.Status.String())
	} else if res.RejectReason != "" {
		log.Printf("res %s: %#v (REJECT: %s)\n", tx.TxID(), res.Status.String(), res.RejectReason)
	} else {
		log.Printf("res %s: %#v\n", tx.TxID(), res.Status.String())
	}

	return nil
}

func newFundingTransaction(fromKeySet *keyset.KeySet, toKeySet *keyset.KeySet, outputs int64) *bt.Tx {
	var fq = bt.NewFeeQuote()
	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	var err error
	addr := fromKeySet.Address(!isRegtest)

	tx := bt.NewTx()

	if isRegtest {
		var txid string
		var vout uint32
		var scriptPubKey string
		for {
			// create the first funding transaction
			txid, vout, scriptPubKey, err = sendToAddress(addr, 100_000_000)
			if err == nil {
				log.Printf("init tx: %s:%d\n", txid, vout)
				break
			}

			log.Printf(".")
			time.Sleep(100 * time.Millisecond)
		}
		err = tx.From(txid, vout, scriptPubKey, 100_000_000)
		if err != nil {
			panic(err)
		}
	} else {
		// live mode, we need to get the utxos from woc
		var utxos []*bt.UTXO
		utxos, err = fromKeySet.GetUTXOs(!isRegtest)
		if err != nil {
			panic(err)
		}
		if len(utxos) == 0 {
			panic("no utxos for address: " + addr)
		}

		// this consumes all the utxos from the key
		err = tx.FromUTXOs(utxos...)
		if err != nil {
			panic(err)
		}
	}

	estimateFee := fees.EstimateFee(uint64(stdFee.MiningFee.Satoshis), 1, 1)
	for i := int64(0); i < outputs; i++ {
		// we send triple the fee to the output address
		// this will allow us to send the change back to the original address
		_ = tx.PayTo(toKeySet.Script, estimateFee*3)
	}

	_ = tx.Change(fromKeySet.Script, fq)

	unlockerGetter := unlocker.Getter{PrivateKey: fromKeySet.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		panic(err)
	}

	return tx
}

func newTransaction(key *keyset.KeySet, useUtxo *bt.UTXO) (*bt.Tx, *bt.UTXO) {
	var err error

	tx := bt.NewTx()
	_ = tx.FromUTXOs(useUtxo)
	// the output value of the utxo should be exactly 3x the fee
	// in this way we can consolidate the output back into the original address
	// even for the smallest transactions with only 1 output
	_ = tx.PayTo(key.Script, 2*useUtxo.Satoshis/3)
	_ = tx.Change(key.Script, fq)

	unlockerGetter := unlocker.Getter{PrivateKey: key.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		panic(err)
	}

	// set the utxo to use for the next transaction
	changeVout := len(tx.Outputs) - 1
	useUtxo = &bt.UTXO{
		TxID:          tx.TxIDBytes(),
		Vout:          uint32(changeVout),
		LockingScript: tx.Outputs[changeVout].LockingScript,
		Satoshis:      tx.Outputs[changeVout].Satoshis,
	}

	return tx, useUtxo
}

// Let the bitcoin node in regtest mode send some bitcoin to our address
func sendToAddress(address string, satoshis uint64) (string, uint32, string, error) {
	rpcURL, err, found := gocore.Config().GetURL("peer_rpc")
	if !found {
		log.Fatalf("Could not find peer_rpc in config: %v", err)
	}
	if err != nil {
		log.Fatalf("Could not parse peer_rpc: %v", err)
	}

	// we are only in dry run mode and will not actually send anything
	if isDryRun {
		txid := random.String(64, random.Hex)
		pubKeyHash := base58.Decode(address)
		scriptPubKey := "76a914" + hex.EncodeToString(pubKeyHash[1:21]) + "88ac"
		return txid, 0, scriptPubKey, nil
	}

	client, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		log.Fatalf("Could not create bitcoin client: %v", err)
	}

	amount := float64(satoshis) / float64(1e8)

	txid, err := client.SendToAddress(address, amount)
	if err != nil {
		return "", 0, "", err
	}

	tx, err := client.GetRawTransaction(txid)
	if err != nil {
		return "", 0, "", err
	}

	for i, vout := range tx.Vout {
		if vout.ScriptPubKey.Addresses[0] == address {
			return txid, uint32(i), vout.ScriptPubKey.Hex, nil
		}
	}

	return "", 0, "", errors.New("address not found in tx")
}
