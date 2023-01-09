package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/libsv/go-bk/bec"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/mrz1836/go-logger"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type utxo struct {
	txid         string
	vout         uint32
	scriptPubKey string
	satoshis     uint64
}

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

func main() {
	sendOnChannel := flag.Int("buffer", 0, "whether to send transactions over a buffered channel and how big the buffer should be")
	useFundingTx := flag.Bool("prefund", false, "whether to use 1 funding transaction for all inputs")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("usage: broadcaster [-buffer=<number of buffered channels>] [-prefund] <number of transactions to send>")
		return
	}

	sendNrOfTransactions, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		panic(err)
	}
	if sendNrOfTransactions == 0 {
		sendNrOfTransactions = 1
	}

	ctx := context.Background()

	addresses, _ := gocore.Config().Get("metamorphAddresses") //, "localhost:8000")

	cc, err := grpc.DialContext(ctx,
		addresses,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`), // This sets the initial balancing policy.
	)
	if err != nil {
		panic(fmt.Errorf("DIALCONTEXT: %v", err))
	}

	client := metamorph_api.NewMetaMorphAPIClient(cc)

	privKey, err := bec.NewPrivateKey(bec.S256())
	if err != nil {
		panic(err)
	}

	txs := make([]*bt.Tx, sendNrOfTransactions)

	if useFundingTx != nil && *useFundingTx {
		fundingTx := newFundingTransaction(privKey, sendNrOfTransactions)
		log.Printf("funding tx: %s\n", fundingTx.TxID())
		_, err = client.PutTransaction(ctx, &metamorph_api.TransactionRequest{
			RawTx: fundingTx.Bytes(),
		})
		if err != nil {
			panic(err)
		}

		for i := int64(0); i < sendNrOfTransactions; i++ {
			u := &utxo{
				txid:         fundingTx.TxID(),
				vout:         uint32(i),
				scriptPubKey: fundingTx.Outputs[i].LockingScript.String(),
				satoshis:     fundingTx.Outputs[i].Satoshis,
			}
			txs[i], _ = newTransaction(privKey, u)
			log.Printf("generated tx: %s\n", txs[i].TxID())
		}
	} else {
		var u *utxo
		for i := int64(0); i < sendNrOfTransactions; i++ {
			txs[i], u = newTransaction(privKey, u)
			log.Printf("generated tx: %s\n", txs[i].TxID())
		}
	}

	// let the node recover
	time.Sleep(1 * time.Second)

	var wg sync.WaitGroup

	if sendOnChannel != nil && *sendOnChannel > 0 {
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

var addr *bscript.Address

func newFundingTransaction(privKey *bec.PrivateKey, outputs int64) *bt.Tx {
	var fq = bt.NewFeeQuote()
	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	var err error
	if addr == nil {
		addr, err = bscript.NewAddressFromPublicKey(privKey.PubKey(), false)
		if err != nil {
			panic(err)
		}
	}

	var txid string
	var vout uint32
	var scriptPubKey string
	for {
		// create the first funding transaction
		txid, vout, scriptPubKey, err = sendToAddress(addr.AddressString, 100_000_000)
		if err == nil {
			log.Printf("init tx: %s:%d\n", txid, vout)
			break
		}

		log.Printf(".")
		time.Sleep(100 * time.Millisecond)
	}

	tx := bt.NewTx()
	_ = tx.From(txid, vout, scriptPubKey, 100_000_000)
	for i := int64(0); i < outputs; i++ {
		_ = tx.PayToAddress(addr.AddressString, 1000)
	}
	_ = tx.ChangeToAddress(addr.AddressString, fq)

	unlockerGetter := unlocker.Getter{PrivateKey: privKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		panic(err)
	}

	return tx
}

func newTransaction(privKey *bec.PrivateKey, useUtxo *utxo) (*bt.Tx, *utxo) {
	var fq = bt.NewFeeQuote()
	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	var err error
	if addr == nil {
		addr, err = bscript.NewAddressFromPublicKey(privKey.PubKey(), false)
		if err != nil {
			panic(err)
		}
	}

	if useUtxo == nil {
		var txid string
		var vout uint32
		var scriptPubKey string
		for {
			// create the first funding transaction
			txid, vout, scriptPubKey, err = sendToAddress(addr.AddressString, 100_000_000)
			if err == nil {
				useUtxo = &utxo{
					txid:         txid,
					vout:         vout,
					scriptPubKey: scriptPubKey,
					satoshis:     100_000_000,
				}
				log.Printf("init useUtxo: %#v\n", useUtxo)
				break
			}

			log.Printf(".")
			time.Sleep(100 * time.Millisecond)
		}
	}

	tx := bt.NewTx()
	_ = tx.From(useUtxo.txid, useUtxo.vout, useUtxo.scriptPubKey, useUtxo.satoshis)
	_ = tx.PayToAddress(addr.AddressString, 900) // let,s just pay 900, allows is to create up to 100_000 transactions
	_ = tx.ChangeToAddress(addr.AddressString, fq)

	unlockerGetter := unlocker.Getter{PrivateKey: privKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		panic(err)
	}

	// set the utxo to use for the next transaction
	changeVout := len(tx.Outputs) - 1
	useUtxo = &utxo{
		txid:         tx.TxID(),
		vout:         uint32(changeVout),
		scriptPubKey: tx.Outputs[changeVout].LockingScriptHexString(),
		satoshis:     tx.Outputs[changeVout].Satoshis,
	}

	return tx, useUtxo
}

func sendToAddress(address string, satoshis uint64) (string, uint32, string, error) {
	// // Create a new transactionHandler instance
	rpcURL, err, found := gocore.Config().GetURL("peer_1_rpc")
	if !found {
		logger.Fatalf("Could not find peer_1_rpc in config: %v", err)
	}
	if err != nil {
		logger.Fatalf("Could not parse peer_1_rpc: %v", err)
	}

	client, err := bitcoin.NewFromURL(rpcURL, false)
	if err != nil {
		logger.Fatalf("Could not create bitcoin client: %v", err)
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
