package broadcaster

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/api"
	"github.com/TAAL-GmbH/arc/lib/fees"
	"github.com/TAAL-GmbH/arc/lib/keyset"
	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bk/base58"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
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

type ClientI interface {
	BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitForStatus metamorph_api.Status) (*metamorph_api.TransactionStatus, error)
	BroadcastTransactions(ctx context.Context, txs []*bt.Tx, waitForStatus metamorph_api.Status) ([]*metamorph_api.TransactionStatus, error)
	GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error)
}

type Broadcaster struct {
	Client        ClientI
	FromKeySet    *keyset.KeySet
	ToKeySet      *keyset.KeySet
	Outputs       int64
	IsDryRun      bool
	IsRegtest     bool
	PrintTxIDs    bool
	Consolidate   bool
	Concurrency   int
	BatchSize     int64
	WaitForStatus api.WaitForStatus
	BatchSend     int
	FeeQuote      *bt.FeeQuote
	summary       map[string]uint64
	summaryMu     sync.Mutex
}

func New(client ClientI, fromKeySet *keyset.KeySet, toKeySet *keyset.KeySet, outputs int64) *Broadcaster {
	var fq = bt.NewFeeQuote()
	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	return &Broadcaster{
		Client:        client,
		FromKeySet:    fromKeySet,
		ToKeySet:      toKeySet,
		Outputs:       outputs,
		IsRegtest:     true,
		BatchSize:     500,
		WaitForStatus: 0,
		FeeQuote:      fq,
		summary:       make(map[string]uint64),
	}
}

func (b *Broadcaster) ConsolidateOutputsToOriginal(ctx context.Context, txs []*bt.Tx, iteration int64) error {
	// consolidate all transactions back into the original arcUrl
	log.Printf("[%d] consolidating all transactions back into original address\n", iteration)
	consolidationAddress := b.FromKeySet.Address(!b.IsRegtest)
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
	// put half of the satoshis back into the original arcUrl
	// the rest will be returned via the change output
	err = consolidationTx.PayToAddress(consolidationAddress, totalSatoshis/2)
	if err != nil {
		return err
	}
	err = consolidationTx.ChangeToAddress(consolidationAddress, b.FeeQuote)
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.ToKeySet.PrivateKey}
	if err = consolidationTx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		return err
	}

	if err = b.ProcessTransaction(ctx, consolidationTx); err != nil {
		return err
	}

	log.Printf("[%d] consolidation tx: %s", iteration, consolidationTx.TxID())

	return nil
}

func (b *Broadcaster) Run(ctx context.Context, concurrency int) error {

	timeStart := time.Now()

	if concurrency == 0 {
		// use the number of outputs as the default concurrency
		// which will not limit the amount of concurrent goroutines
		concurrency = int(b.BatchSize)
	}
	log.Printf("Using concurrency of %d\n", concurrency)

	// loop through the number of outputs in batches of b.BatchSize (default 500)
	log.Printf("creating funding txs\n")
	var wg sync.WaitGroup
	limit := make(chan struct{}, runtime.NumCPU())
	for i := int64(0); i < b.Outputs; i += b.BatchSize {
		wg.Add(1)
		limit <- struct{}{}

		batchSize := b.BatchSize
		if i+batchSize > b.Outputs {
			batchSize = b.Outputs - i
		}

		go func(i int64) {
			defer func() {
				wg.Done()
				<-limit
			}()

			iteration := (i / b.BatchSize) + 1

			fundingTx := b.NewFundingTransaction(batchSize, iteration)
			log.Printf("[%d] outputs tx: %s\n", iteration, fundingTx.TxID())

			_, err := b.Client.BroadcastTransaction(ctx, fundingTx, metamorph_api.Status(b.WaitForStatus))
			if err != nil {
				log.Printf("[%d] error broadcasting funding tx: %s\n", iteration, err.Error())
				return
			}

			log.Printf("[%d] running batch: %d - %d\n", iteration, i, i+batchSize)
			err = b.runBatch(ctx, concurrency, fundingTx, iteration)
			if err != nil {
				log.Printf("[%d] error running batch: %s\n", iteration, err.Error())
				return
			}
		}(i)
	}
	wg.Wait()

	log.Printf("sent %d txs in %0.2f seconds\n", b.Outputs, time.Since(timeStart).Seconds())

	// print summary
	log.Printf("Summary:\n")
	totalCount := uint64(0)
	for summaryString, count := range b.summary {
		log.Printf("%s: %d\n", summaryString, count)
		totalCount += count
	}
	log.Printf("Total: %d\n", totalCount)

	return nil
}

func (b *Broadcaster) runBatch(ctx context.Context, concurrency int, fundingTx *bt.Tx, iteration int64) error {
	txs := make([]*bt.Tx, len(fundingTx.Outputs))
	for i := 0; i < len(fundingTx.Outputs); i++ {
		u := &bt.UTXO{
			TxID:          fundingTx.TxIDBytes(),
			Vout:          uint32(i),
			LockingScript: fundingTx.Outputs[i].LockingScript,
			Satoshis:      fundingTx.Outputs[i].Satoshis,
		}
		txs[i], _ = b.NewTransaction(b.ToKeySet, u)
	}

	if b.IsDryRun {
		for _, tx := range txs {
			//log.Printf("Processing tx %d / %d\n", i+1, len(b.txs))
			if err := b.ProcessTransaction(ctx, tx); err != nil {
				return err
			}
		}
	} else {
		if b.BatchSend > 0 {
			log.Printf("[%d] Using batch size of %d in iteration\n", iteration, b.BatchSend)

			var wg sync.WaitGroup
			for i := 0; i < len(txs); i += b.BatchSend {
				wg.Add(1)
				j := i + b.BatchSend
				if j > len(txs) {
					j = len(txs)
				}

				log.Printf("[%d]   Sending batch of %d - %d\n", iteration, i, j)
				go func(txs []*bt.Tx) {
					defer wg.Done()

					txStatus, err := b.Client.BroadcastTransactions(ctx, txs, metamorph_api.Status(b.WaitForStatus))
					if err != nil {
						log.Printf("Error broadcasting transactions: %s\n", err.Error())
					}
					for _, res := range txStatus {
						b.processResult(res)
					}
				}(txs[i:j])
			}
			wg.Wait()
		} else {
			var wg sync.WaitGroup
			// limit the amount of concurrent goroutines
			limit := make(chan bool, concurrency)
			for i := 0; i < len(fundingTx.Outputs); i++ {
				wg.Add(1)
				limit <- true
				go func(i int) {
					defer func() {
						wg.Done()
						<-limit
					}()

					u := &bt.UTXO{
						TxID:          fundingTx.TxIDBytes(),
						Vout:          uint32(i),
						LockingScript: fundingTx.Outputs[i].LockingScript,
						Satoshis:      fundingTx.Outputs[i].Satoshis,
					}
					txs[i], _ = b.NewTransaction(b.ToKeySet, u)

					if err := b.ProcessTransaction(ctx, txs[i]); err != nil {
						log.Printf("[%d] Error in %s: %s", iteration, txs[i].TxID(), err.Error())
						log.Println(txs[i].String())
					}
				}(i)
			}
			wg.Wait()
		}
	}

	if b.Consolidate {
		if err := b.ConsolidateOutputsToOriginal(ctx, txs, iteration); err != nil {
			panic(err)
		}
	}

	return nil
}

func (b *Broadcaster) ProcessTransaction(ctx context.Context, tx *bt.Tx) error {
	res, err := b.Client.BroadcastTransaction(ctx, tx, metamorph_api.Status(b.WaitForStatus))
	if err != nil {
		return fmt.Errorf("error broadcasting transaction %s: %s", tx.TxID(), err.Error())
	}

	b.processResult(res)

	return nil
}

func (b *Broadcaster) processResult(res *metamorph_api.TransactionStatus) {
	if b.PrintTxIDs {
		if res.TimedOut {
			log.Printf("res %s: %#v (TIMEOUT)\n", res.Txid, res.Status.String())
		} else if res.RejectReason != "" {
			log.Printf("res %s: %#v (REJECT: %s)\n", res.Txid, res.Status.String(), res.RejectReason)
		} else {
			log.Printf("res %s: %#v\n", res.Txid, res.Status.String())
		}
	}

	summaryString := "  " + res.Status.String()
	if res.TimedOut {
		summaryString += " (TIMEOUT)"
	}
	if res.RejectReason != "" {
		summaryString += " (REJECT: " + res.RejectReason + ")"
	}

	b.summaryMu.Lock()
	b.summary[summaryString]++
	b.summaryMu.Unlock()
}

func (b *Broadcaster) NewFundingTransaction(outputs, iteration int64) *bt.Tx {
	var fq = bt.NewFeeQuote()
	fq.AddQuote(bt.FeeTypeStandard, stdFee)
	fq.AddQuote(bt.FeeTypeData, dataFee)

	var err error
	addr := b.FromKeySet.Address(!b.IsRegtest)

	tx := bt.NewTx()

	if b.IsRegtest {
		var txid string
		var vout uint32
		var scriptPubKey string
		for {
			// create the first funding transaction
			txid, vout, scriptPubKey, err = b.SendToAddress(addr, 100_000_000)
			if err == nil {
				log.Printf("[%d] funding tx: %s:%d\n", iteration, txid, vout)
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
		utxos, err = b.FromKeySet.GetUTXOs(!b.IsRegtest)
		if err != nil {
			panic(err)
		}
		if len(utxos) == 0 {
			panic("no utxos for arcUrl: " + addr)
		}

		// this consumes all the utxos from the key
		err = tx.FromUTXOs(utxos...)
		if err != nil {
			panic(err)
		}

		log.Printf("[%d] funding tx: %s\n", iteration, tx.TxID())
	}

	estimateFee := fees.EstimateFee(uint64(stdFee.MiningFee.Satoshis), 1, 1)
	for i := int64(0); i < outputs; i++ {
		// we send triple the fee to the output arcUrl
		// this will allow us to send the change back to the original arcUrl
		_ = tx.PayTo(b.ToKeySet.Script, estimateFee*3)
	}

	_ = tx.Change(b.FromKeySet.Script, fq)

	unlockerGetter := unlocker.Getter{PrivateKey: b.FromKeySet.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		panic(err)
	}

	return tx
}

func (b *Broadcaster) NewTransaction(key *keyset.KeySet, useUtxo *bt.UTXO) (*bt.Tx, *bt.UTXO) {
	var err error

	tx := bt.NewTx()
	_ = tx.FromUTXOs(useUtxo)
	// the output value of the utxo should be exactly 3x the fee
	// in this way we can consolidate the output back into the original arcUrl
	// even for the smallest transactions with only 1 output
	_ = tx.PayTo(key.Script, 2*useUtxo.Satoshis/3)
	_ = tx.Change(key.Script, b.FeeQuote)

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

// SendToAddress Let the bitcoin node in regtest mode send some bitcoin to our arcUrl
func (b *Broadcaster) SendToAddress(address string, satoshis uint64) (string, uint32, string, error) {
	rpcURL, err, found := gocore.Config().GetURL("peer_rpc")
	if !found {
		log.Fatalf("Could not find peer_rpc in config: %v", err)
	}
	if err != nil {
		log.Fatalf("Could not parse peer_rpc: %v", err)
	}

	// we are only in dry run mode and will not actually send anything
	if b.IsDryRun {
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

	btTx, err := bt.NewTxFromString(tx.Hex)
	if err != nil {
		return "", 0, "", err
	}
	if err := b.ProcessTransaction(context.Background(), btTx); err != nil {
		return "", 0, "", err
	}

	for i, vout := range tx.Vout {
		if vout.ScriptPubKey.Addresses[0] == address {
			return txid, uint32(i), vout.ScriptPubKey.Hex, nil
		}
	}

	return "", 0, "", errors.New("arcUrl not found in tx")
}
