package broadcaster

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/api"
	"github.com/bitcoin-sv/arc/lib/fees"
	"github.com/bitcoin-sv/arc/lib/keyset"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bk/base58"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/unlocker"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/spf13/viper"
)

var stdFeeDefault = &bt.Fee{
	FeeType: bt.FeeTypeStandard,
	MiningFee: bt.FeeUnit{
		Satoshis: 3,
		Bytes:    1000,
	},
	RelayFee: bt.FeeUnit{
		Satoshis: 0,
		Bytes:    1000,
	},
}

var dataFeeDefault = &bt.Fee{
	FeeType: bt.FeeTypeData,
	MiningFee: bt.FeeUnit{
		Satoshis: 3,
		Bytes:    1000,
	},
	RelayFee: bt.FeeUnit{
		Satoshis: 0,
		Bytes:    1000,
	},
}

type ArcClient interface {
	BroadcastTransaction(ctx context.Context, tx *bt.Tx, waitForStatus metamorph_api.Status, callbackURL string) (*metamorph_api.TransactionStatus, error)
	BroadcastTransactions(ctx context.Context, txs []*bt.Tx, waitForStatus metamorph_api.Status, callbackURL string, callbackToken string) ([]*metamorph_api.TransactionStatus, error)
	GetTransactionStatus(ctx context.Context, txID string) (*metamorph_api.TransactionStatus, error)
}

type Broadcaster struct {
	logger        utils.Logger
	Client        ArcClient
	FromKeySet    *keyset.KeySet
	ToKeySet      *keyset.KeySet
	Outputs       int64
	IsDryRun      bool
	IsRegtest     bool
	IsTestnet     bool
	PrintTxIDs    bool
	Consolidate   bool
	Concurrency   int
	CallbackURL   string
	BatchSize     int64
	WaitForStatus api.WaitForStatus
	BatchSend     int
	FeeQuote      *bt.FeeQuote
	summary       map[string]uint64
	summaryMu     sync.Mutex
}

func WithMiningFee(miningFeeSatPerKb int) func(fee *bt.Fee) {
	return func(fee *bt.Fee) {
		fee.RelayFee.Satoshis = miningFeeSatPerKb
		fee.MiningFee.Satoshis = miningFeeSatPerKb
	}
}

func New(logger utils.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeySet *keyset.KeySet, outputs int64, feeOpts ...func(fee *bt.Fee)) *Broadcaster {
	var fq = bt.NewFeeQuote()

	stdFee := *stdFeeDefault
	dataFee := *dataFeeDefault

	for _, opt := range feeOpts {
		opt(&stdFee)
		opt(&dataFee)
	}

	fq.AddQuote(bt.FeeTypeData, &stdFee)
	fq.AddQuote(bt.FeeTypeStandard, &dataFee)

	return &Broadcaster{
		logger:        logger,
		Client:        client,
		FromKeySet:    fromKeySet,
		ToKeySet:      toKeySet,
		Outputs:       outputs,
		IsRegtest:     true,
		IsTestnet:     true,
		BatchSize:     500,
		WaitForStatus: 0,
		FeeQuote:      fq,
		summary:       make(map[string]uint64),
	}
}

func (b *Broadcaster) ConsolidateOutputsToOriginal(ctx context.Context, txs []*bt.Tx, iteration int64) error {
	// consolidate all transactions back into the original arcUrl
	lenValidTxs := 0
	for _, tx := range txs {
		if tx.Version > 0 {
			lenValidTxs++
		}
	}
	b.logger.Infof("[%d] consolidating %d / %d transactions back into original address", iteration, lenValidTxs, len(txs))
	if b.logger.LogLevel() == int(gocore.DEBUG) && b.PrintTxIDs {
		for _, tx := range txs {
			b.logger.Debugf("[%d]    consolidating tx: %s", iteration, tx.TxID())
		}
	}

	consolidationAddress := b.FromKeySet.Address(!b.IsRegtest)
	consolidationTx := bt.NewTx()
	utxos := make([]*bt.UTXO, 0, len(txs))
	totalSatoshis := uint64(0)
	for _, tx := range txs {
		// only select transactions that succeeded, failed transactions will have a version of 0
		if tx.Version > 0 {
			utxos = append(utxos, &bt.UTXO{
				TxID:          tx.TxIDBytes(),
				Vout:          0,
				LockingScript: tx.Outputs[0].LockingScript,
				Satoshis:      tx.Outputs[0].Satoshis,
			})
			totalSatoshis += tx.Outputs[0].Satoshis
		}
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

	if err = b.ProcessTransaction(ctx, consolidationTx, iteration); err != nil {
		b.logger.Errorf("[%d] failed to process consolidation tx %s: %v", iteration, consolidationTx.TxID(), err)
		return err
	}

	b.logger.Infof("[%d] consolidation tx: %s", iteration, consolidationTx.TxID())

	return nil
}

func (b *Broadcaster) Run(ctx context.Context, concurrency int) error {

	timeStart := time.Now()

	if concurrency == 0 {
		// use the number of outputs as the default concurrency
		// which will not limit the amount of concurrent goroutines
		concurrency = int(b.BatchSize)
	}
	b.logger.Infof("Using concurrency of %d", concurrency)

	// first we create all the funding transactions, serially and wait for them to propagate
	batches := int64(math.Ceil(float64(b.Outputs) / float64(b.BatchSize)))
	b.logger.Infof("Creating %d funding txs for batches", batches)

	fundingTxs := make([]*bt.Tx, batches)
	for i := int64(0); i < b.Outputs; i += b.BatchSize {
		batchSize := b.BatchSize
		if i+batchSize > b.Outputs {
			batchSize = b.Outputs - i
		}

		iteration := (i / b.BatchSize) + 1

		fundingTx, err := b.NewFundingTransaction(batchSize, iteration)
		if err != nil {
			return err
		}
		for i := range fundingTx.Inputs {
			b.logger.Infof("[%d] funding previous tx id [%d]: %s, vout: %d", iteration, i, hex.EncodeToString(fundingTx.Inputs[i].PreviousTxID()), fundingTx.Inputs[i].PreviousTxOutIndex)
		}
		b.logger.Infof("[%d] funding tx: %s", iteration, fundingTx.TxID())

		// retry 3 times to get the funding transaction broadcast with seen on network status
		for i := 1; i <= 3; i++ {
			b.logger.Infof("retrying broadcasting funding tx: %d", i)
			status, err := b.Client.BroadcastTransaction(ctx, fundingTx, metamorph_api.Status(b.WaitForStatus), b.CallbackURL)
			if err != nil {
				b.logger.Fatalf("[%d] error broadcasting funding tx: %s", iteration, err.Error())
			}

			if status.Status == metamorph_api.Status_SEEN_ON_NETWORK {
				break
			} else if i == 3 {
				b.logger.Fatalf("[%d] funding tx not seen on network: %s", iteration, status.Status.String())
			} else {
				// wait for things to settle down
				time.Sleep(5 * time.Second)
			}
		}

		fundingTxs[iteration-1] = fundingTx
	}

	b.logger.Infof("Created %d funding batches in %0.2f seconds", batches, time.Since(timeStart).Seconds())
	time.Sleep(5 * time.Second)

	// start the timer for the transaction sending
	timeStart = time.Now()

	// loop through the number of outputs in batches of b.BatchSize (default 500)
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

			fundingTx := fundingTxs[iteration-1]

			b.logger.Infof("[%d] running batch: %d - %d", iteration, i, i+batchSize)
			err := b.runBatch(ctx, concurrency, fundingTx, iteration)
			if err != nil {
				b.logger.Infof("[%d] error running batch: %s", iteration, err.Error())
				return
			}
		}(i)
	}
	wg.Wait()

	b.logger.Infof("sent %d txs in %0.2f seconds", b.Outputs, time.Since(timeStart).Seconds())

	// print summary
	b.logger.Infof("Summary:")
	totalCount := uint64(0)
	for summaryString, count := range b.summary {
		b.logger.Infof("%s: %d", summaryString, count)
		totalCount += count
	}
	b.logger.Infof("Total: %d", totalCount)

	return nil
}

func (b *Broadcaster) runBatch(ctx context.Context, concurrency int, fundingTx *bt.Tx, iteration int64) error {
	txs := make([]*bt.Tx, len(fundingTx.Outputs)-1)
	for i := 0; i < len(fundingTx.Outputs)-1; i++ {
		u := &bt.UTXO{
			TxID:          fundingTx.TxIDBytes(),
			Vout:          uint32(i),
			LockingScript: fundingTx.Outputs[i].LockingScript,
			Satoshis:      fundingTx.Outputs[i].Satoshis,
		}
		tx, err := b.NewTransaction(b.ToKeySet, u)
		if err != nil {
			return err
		}
		txs[i] = tx

	}

	if b.IsDryRun {
		for _, tx := range txs {
			if err := b.ProcessTransaction(ctx, tx, iteration); err != nil {
				return err
			}
		}
	} else {
		if b.BatchSend > 0 {
			b.logger.Infof("[%d] Using batch size of %d in iteration", iteration, b.BatchSend)

			var wg sync.WaitGroup
			for i := 0; i < len(txs); i += b.BatchSend {
				wg.Add(1)
				j := i + b.BatchSend
				if j > len(txs) {
					j = len(txs)
				}

				b.logger.Infof("[%d]   Sending batch of %d - %d", iteration, i, j)
				go func(txs []*bt.Tx, indexStart, indexEnd int) {
					defer wg.Done()

					txStatus, err := b.Client.BroadcastTransactions(ctx, txs, metamorph_api.Status(b.WaitForStatus), b.CallbackURL, "")
					if err != nil {
						b.logger.Errorf("[%d]   batch of %d - %d failed %s", iteration, indexStart, indexEnd, err.Error())
					} else {
						b.logger.Infof("[%d]   batch of %d - %d successful", iteration, indexStart, indexEnd)
					}
					for _, res := range txStatus {
						b.processResult(res, iteration)
					}
				}(txs[i:j], i, j)
			}
			wg.Wait()
		} else {
			var wg sync.WaitGroup
			// limit the amount of concurrent goroutines
			limit := make(chan bool, concurrency)
			for i, tx := range txs {
				wg.Add(1)
				limit <- true

				go func(i int, tx *bt.Tx) {
					defer func() {
						wg.Done()
						<-limit
					}()

					if err := b.ProcessTransaction(ctx, tx, iteration); err != nil {
						b.logger.Infof("[%d] Error in %s: %s", iteration, txs[i].TxID(), err.Error())
						b.logger.Infof(txs[i].String())
						// mark the tx as invalid
						tx.Version = 0
					}
				}(i, tx)
			}
			wg.Wait()
		}
	}

	if b.Consolidate {
		if err := b.ConsolidateOutputsToOriginal(ctx, txs, iteration); err != nil {
			return err
		}
	}

	return nil
}

func (b *Broadcaster) ProcessTransaction(ctx context.Context, tx *bt.Tx, iteration int64) error {
	res, err := b.Client.BroadcastTransaction(ctx, tx, metamorph_api.Status(b.WaitForStatus), b.CallbackURL)
	if err != nil {
		return fmt.Errorf("error broadcasting transaction %s: %s", tx.TxID(), err.Error())
	}

	b.processResult(res, iteration)

	return nil
}

func (b *Broadcaster) processResult(res *metamorph_api.TransactionStatus, iteration int64) {
	if b.PrintTxIDs {
		if res.TimedOut {
			b.logger.Infof("[%d] res %s: %#v (TIMEOUT)", iteration, res.Txid, res.Status.String())
		} else if res.RejectReason != "" {
			b.logger.Infof("[%d] res %s: %#v (REJECT: %s)", iteration, res.Txid, res.Status.String(), res.RejectReason)
		} else {
			b.logger.Infof("[%d] res %s: %#v", iteration, res.Txid, res.Status.String())
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

func (b *Broadcaster) NewFundingTransaction(outputs, iteration int64) (*bt.Tx, error) {
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
			if err != nil {
				b.logger.Errorf("error sending to address: %s", err.Error())
			} else {
				b.logger.Infof("[%d] funding tx: %s:%d", iteration, txid, vout)
				break
			}

			b.logger.Infof(".")
			time.Sleep(100 * time.Millisecond)
		}
		err = tx.From(txid, vout, scriptPubKey, 100_000_000)
		if err != nil {
			return nil, err
		}
	} else {
		// live mode, we need to get the utxos from woc
		var utxos []*bt.UTXO
		utxos, err = b.FromKeySet.GetUTXOs(!b.IsTestnet)
		if err != nil {
			return nil, err
		}
		if len(utxos) == 0 {
			return nil, err
		}

		// this consumes all the utxos from the key
		err = tx.FromUTXOs(utxos...)
		if err != nil {
			return nil, err
		}

	}

	estimateFee := fees.EstimateFee(uint64(stdFeeDefault.MiningFee.Satoshis), 1, 1)
	for i := int64(0); i < outputs; i++ {
		if b.Consolidate {
			// we send triple the fee to the output arcUrl
			// this will allow us to send the change back to the original arcUrl
			_ = tx.PayTo(b.ToKeySet.Script, estimateFee*3)
		} else {
			_ = tx.PayTo(b.ToKeySet.Script, estimateFee+1) // add 1 satoshi to allow for our longer OP_RETURN
		}
	}

	_ = tx.Change(b.FromKeySet.Script, b.FeeQuote)

	unlockerGetter := unlocker.Getter{PrivateKey: b.FromKeySet.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		return nil, err
	}

	return tx, nil
}

func (b *Broadcaster) NewTransaction(key *keyset.KeySet, useUtxo *bt.UTXO) (*bt.Tx, error) {
	var err error

	tx := bt.NewTx()
	_ = tx.FromUTXOs(useUtxo)

	if b.Consolidate {
		// the output value of the utxo should be exactly 3x the fee
		// in this way we can consolidate the output back into the original arcUrl
		// even for the smallest transactions with only 1 output
		_ = tx.PayTo(key.Script, 2*useUtxo.Satoshis/3)
		_ = tx.Change(key.Script, b.FeeQuote)
	} else {
		_ = tx.AddOpReturnOutput([]byte("ARC is here ... https://bitcoin-sv.github.io/arc/"))
	}

	unlockerGetter := unlocker.Getter{PrivateKey: key.PrivateKey}
	if err = tx.FillAllInputs(context.Background(), &unlockerGetter); err != nil {
		return nil, err
	}

	return tx, nil
}

// SendToAddress Let the bitcoin node in regtest mode send some bitcoin to our arcUrl
func (b *Broadcaster) SendToAddress(address string, satoshis uint64) (string, uint32, string, error) {

	peerRpcPassword := viper.GetString("peerRpc.password")
	if peerRpcPassword == "" {
		return "", 0, "", fmt.Errorf("setting peerRpc.password not found")
	}

	peerRpcUser := viper.GetString("peerRpc.user")
	if peerRpcUser == "" {
		return "", 0, "", fmt.Errorf("setting peerRpc.user not found")
	}

	peerRpcHost := viper.GetString("peerRpc.host")
	if peerRpcHost == "" {
		return "", 0, "", fmt.Errorf("setting peerRpc.host not found")
	}
	peerRpcPort := viper.GetInt("peerRpc.port")
	if peerRpcPort == 0 {
		return "", 0, "", fmt.Errorf("setting peerRpc.port not found")
	}

	rpcURL, err := url.Parse(fmt.Sprintf("rpc://%s:%s@%s:%d", peerRpcUser, peerRpcPassword, peerRpcHost, peerRpcPort))
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to parse rpc URL: %v", err)
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
		b.logger.Fatalf("Could not create bitcoin client: %v", err)
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

	return "", 0, "", errors.New("arcUrl not found in tx")
}
