package broadcaster

import (
	"bufio"
	"container/list"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/bitcoin-sv/arc/internal/keyset"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-bt/v2/bscript"
	"github.com/libsv/go-bt/v2/unlocker"
)

const (
	maxInputsDefault         = 100
	batchSizeDefault         = 20
	isTestnetDefault         = true
	millisecondsPerSecond    = 1000
	resultsIterationsDefault = 50
)

type UtxoClient interface {
	GetUTXOs(mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
	GetUTXOsList(mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error)
	GetBalance(mainnet bool, address string) (int64, int64, error)
	TopUp(mainnet bool, address string) error
}

type RateBroadcaster struct {
	logger                         *slog.Logger
	client                         ArcClient
	fundingKeyset                  *keyset.KeySet
	receivingKeyset                *keyset.KeySet
	isTestnet                      bool
	callbackURL                    string
	callbackToken                  string
	fullStatusUpdates              bool
	feeQuote                       *bt.FeeQuote
	utxoClient                     UtxoClient
	standardMiningFee              bt.FeeUnit
	responseWriter                 io.Writer
	responseWriteIterationInterval int

	shutdown         chan struct{}
	shutdownComplete chan struct{}

	maxInputs int
	batchSize int
}

func WithFees(miningFeeSatPerKb int) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		var fq = bt.NewFeeQuote()

		newStdFee := *stdFeeDefault
		newDataFee := *dataFeeDefault

		newStdFee.MiningFee.Satoshis = miningFeeSatPerKb
		newDataFee.MiningFee.Satoshis = miningFeeSatPerKb

		fq.AddQuote(bt.FeeTypeData, &newStdFee)
		fq.AddQuote(bt.FeeTypeStandard, &newDataFee)

		broadcaster.feeQuote = fq
	}
}

func WithBatchSize(batchSize int) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.batchSize = batchSize
	}
}

func WithMaxInputs(maxInputs int) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.maxInputs = maxInputs
	}
}

func WithIsTestnet(isTestnet bool) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.isTestnet = isTestnet
	}
}

func WithCallback(callbackURL string, callbackToken string) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.callbackURL = callbackURL
		broadcaster.callbackToken = callbackToken
	}
}

func WithFullstatusUpdates(fullStatusUpdates bool) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.fullStatusUpdates = fullStatusUpdates
	}
}

func WithStoreWriter(storeWriter io.Writer, resultIterations int) func(broadcaster *RateBroadcaster) {
	return func(broadcaster *RateBroadcaster) {
		broadcaster.responseWriter = storeWriter
		broadcaster.responseWriteIterationInterval = resultIterations
	}
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, toKeyset *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *RateBroadcaster)) (*RateBroadcaster, error) {
	broadcaster := &RateBroadcaster{
		logger:                         logger,
		client:                         client,
		fundingKeyset:                  fromKeySet,
		receivingKeyset:                toKeyset,
		isTestnet:                      isTestnetDefault,
		feeQuote:                       bt.NewFeeQuote(),
		utxoClient:                     utxoClient,
		batchSize:                      batchSizeDefault,
		maxInputs:                      maxInputsDefault,
		responseWriter:                 nil,
		responseWriteIterationInterval: resultsIterationsDefault,

		shutdown:         make(chan struct{}, 10),
		shutdownComplete: make(chan struct{}, 10),
	}

	for _, opt := range opts {
		opt(broadcaster)
	}

	standardFee, err := broadcaster.feeQuote.Fee(bt.FeeTypeStandard)
	if err != nil {
		return nil, err
	}

	broadcaster.standardMiningFee = standardFee.MiningFee

	return broadcaster, nil
}

// Payback sends all funds currently held on the receiving address back to the funding address
func (b *RateBroadcaster) Payback() error {
	utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.receivingKeyset.Script, b.receivingKeyset.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	tx := bt.NewTx()
	txSatoshis := uint64(0)
	batchSatoshis := uint64(0)
	if err != nil {
		return err
	}

	txs := make([]*bt.Tx, 0, b.batchSize)

	for _, utxo := range utxos {

		err = tx.FromUTXOs(utxo)
		if err != nil {
			return err
		}

		txSatoshis += utxo.Satoshis

		// create payback transactions with maximum 100 inputs
		if len(tx.Inputs) >= b.maxInputs {
			batchSatoshis += txSatoshis

			err = b.payToFundingKeySet(tx, txSatoshis, b.receivingKeyset)
			if err != nil {
				return err
			}

			txs = append(txs, tx)

			tx = bt.NewTx()
			txSatoshis = 0
		}

		if len(txs) == b.batchSize {
			err = b.submitTxs(txs, metamorph_api.Status_SEEN_ON_NETWORK, false)
			if err != nil {
				return err
			}
			b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))

			batchSatoshis = 0
			txs = make([]*bt.Tx, 0, b.batchSize)
			time.Sleep(time.Millisecond * 100)
		}
	}

	if len(tx.Inputs) > 0 {
		batchSatoshis += txSatoshis

		err = b.payToFundingKeySet(tx, txSatoshis, b.receivingKeyset)
		if err != nil {
			return err
		}

		txs = append(txs, tx)
	}

	if len(txs) > 0 {
		err = b.submitTxs(txs, metamorph_api.Status_SEEN_ON_NETWORK, false)
		if err != nil {
			return err
		}

		b.logger.Info("paid back satoshis", slog.Uint64("satoshis", batchSatoshis))
	}

	return nil
}

func (b *RateBroadcaster) calculateFeeSat(tx *bt.Tx) uint64 {
	size, err := tx.EstimateSizeWithTypes()
	if err != nil {
		return 0
	}
	varIntUpper := bt.VarInt(tx.OutputCount()).UpperLimitInc()
	if varIntUpper == -1 {
		return 0
	}

	changeOutputFee := varIntUpper
	changeP2pkhByteLen := uint64(8 + 1 + 25)

	totalBytes := size.TotalStdBytes + changeP2pkhByteLen

	miningFeeSat := float64(totalBytes*uint64(b.standardMiningFee.Satoshis)) / float64(b.standardMiningFee.Bytes)

	sFees := uint64(math.Ceil(miningFeeSat))
	txFees := sFees + uint64(changeOutputFee)

	return txFees
}

func (b *RateBroadcaster) payToFundingKeySet(tx *bt.Tx, totalSatoshis uint64, payingKeyset *keyset.KeySet) error {

	err := tx.PayTo(b.fundingKeyset.Script, totalSatoshis-b.calculateFeeSat(tx))
	if err != nil {
		return err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: payingKeyset.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return err
	}

	return nil
}

func (b *RateBroadcaster) splitToFundingKeyset(tx *bt.Tx, splitSatoshis uint64, requestedSatoshis uint64, requestedOutputs int) (addedOutputs int, err error) {

	if requestedSatoshis > splitSatoshis {
		return 0, fmt.Errorf("requested satoshis %d greater than satoshis to be split %d", requestedSatoshis, splitSatoshis)
	}

	counter := 0

	remaining := int64(splitSatoshis)
	for remaining > int64(requestedSatoshis) && counter < requestedOutputs {
		if uint64(remaining)-requestedSatoshis < b.calculateFeeSat(tx) {
			break
		}

		err := tx.PayTo(b.fundingKeyset.Script, requestedSatoshis)
		if err != nil {
			return 0, err
		}

		remaining -= int64(requestedSatoshis)
		counter++

	}

	fee := b.calculateFeeSat(tx)
	err = tx.PayTo(b.fundingKeyset.Script, uint64(remaining)-fee)
	if err != nil {
		return 0, err
	}

	unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
	err = tx.FillAllInputs(context.Background(), &unlockerGetter)
	if err != nil {
		return 0, err
	}

	return counter, nil
}

func (b *RateBroadcaster) submitTxs(txs []*bt.Tx, expectedStatus metamorph_api.Status, skipFeeValidation bool) error {
	resp, err := b.client.BroadcastTransactions(context.Background(), txs, expectedStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, skipFeeValidation)
	if err != nil {
		return err
	}

	for _, res := range resp {
		if res.Status != expectedStatus {
			return fmt.Errorf("transaction does not have expected status %s, but %s", expectedStatus.String(), res.Status.String())
		}
	}

	return nil
}

func (b *RateBroadcaster) createConsolidationTxs(utxos *list.List, satoshiMap map[string]uint64) ([][]*bt.Tx, error) {
	tx := bt.NewTx()
	txSatoshis := uint64(0)
	txsConsolidationBatches := make([][]*bt.Tx, 0)
	txsConsolidation := make([]*bt.Tx, 0)
	const consolidateBatchSize = 20

	var next *list.Element
	for front := utxos.Front(); front != nil; front = next {
		next = front.Next()

		utxos.Remove(front)

		utxo, ok := front.Value.(*bt.UTXO)
		if !ok {
			return nil, errors.New("failed to parse value to utxo")
		}

		txSatoshis += utxo.Satoshis
		if next == nil {
			if len(tx.Inputs) > 0 {
				err := tx.FromUTXOs(utxo)
				if err != nil {
					return nil, err
				}

				fee := b.calculateFeeSat(tx)
				err = tx.PayTo(b.fundingKeyset.Script, txSatoshis-fee)
				if err != nil {
					return nil, err
				}
				unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
				err = tx.FillAllInputs(context.Background(), &unlockerGetter)
				if err != nil {
					return nil, err
				}

				txsConsolidation = append(txsConsolidation, tx)
				satoshiMap[tx.TxID()] = txSatoshis
			}

			if len(txsConsolidation) > 0 {
				txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			}
			break
		}

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		if len(tx.Inputs) >= b.maxInputs {
			err = tx.PayTo(b.fundingKeyset.Script, txSatoshis)
			if err != nil {
				return nil, err
			}
			unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
			err = tx.FillAllInputs(context.Background(), &unlockerGetter)
			if err != nil {
				return nil, err
			}

			txsConsolidation = append(txsConsolidation, tx)

			fmt.Println(tx.String())

			satoshiMap[tx.TxID()] = txSatoshis
			tx = bt.NewTx()
			txSatoshis = 0
		}

		if len(txsConsolidation) >= consolidateBatchSize {
			txsConsolidationBatches = append(txsConsolidationBatches, txsConsolidation)
			txsConsolidation = make([]*bt.Tx, 0)
		}
	}

	return txsConsolidationBatches, nil
}

func (b *RateBroadcaster) Consolidate() error {
	utxoSet, err := b.utxoClient.GetUTXOsList(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
	if err != nil {
		return fmt.Errorf("failed to get utxos: %v", err)
	}

	if utxoSet.Len() == 1 {
		b.logger.Info("utxos already consolidated")
		return nil
	}

	submitBatchTicker := time.NewTicker(5 * time.Second)

	responseCh := make(chan *metamorph_api.TransactionStatus, 200000)
	errCh := make(chan error, 100)

	satoshiMap := map[string]uint64{}

outerLoop:
	for {
		select {
		case <-submitBatchTicker.C:

			b.logger.Info("consolidating outputs", slog.Int("outputs", utxoSet.Len()))

			consolidationTxsBatches, err := b.createConsolidationTxs(utxoSet, satoshiMap)
			if err != nil {
				return fmt.Errorf("failed to create consolidation txs: %v", err)
			}

			if len(consolidationTxsBatches) == 0 {
				b.logger.Info("finished consolidation")
				break outerLoop
			}

			for _, batch := range consolidationTxsBatches {
				b.sendTxsBatchAsync(batch, responseCh, errCh, false, metamorph_api.Status_SEEN_ON_NETWORK)
				time.Sleep(200 * time.Millisecond)
			}

		case responseErr := <-errCh:
			b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))

		case res := <-responseCh:

			if res.Status == metamorph_api.Status_REJECTED || res.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
				bytes, err := json.Marshal(res)
				if err != nil {
					b.logger.Error("failed to decode res", slog.String("err", err.Error()))
				}

				return fmt.Errorf("consolidation tx was not successful: %s", string(bytes))
			}

			txIDBytes, err := hex.DecodeString(res.Txid)
			if err != nil {
				b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
				continue
			}

			newUtxo := &bt.UTXO{
				TxID:          txIDBytes,
				Vout:          0,
				LockingScript: b.fundingKeyset.Script,
				Satoshis:      satoshiMap[res.Txid],
			}

			delete(satoshiMap, res.Txid)

			utxoSet.PushBack(newUtxo)
		}
	}

	return nil
}

func (b *RateBroadcaster) CreateUtxos(requestedOutputs int, requestedSatoshisPerOutput uint64) error {

	requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

	confirmed, unconfirmed, err := b.utxoClient.GetBalance(!b.isTestnet, b.fundingKeyset.Address(!b.isTestnet))
	if err != nil {
		return err
	}

	if unconfirmed > 0 {
		return fmt.Errorf("total balance not confirmed yet")
	}

	balance := confirmed + unconfirmed

	if requestedOutputsSatoshis > balance {
		return fmt.Errorf("requested total of satoshis %d exceeds balance on funding keyset %d", requestedOutputsSatoshis, balance)
	}

	utxoSet := make([]*bt.UTXO, 0, requestedOutputs)

requestedOutputsLoop:
	for len(utxoSet) < requestedOutputs {

		utxoSet = make([]*bt.UTXO, 0, requestedOutputs)
		greater := make([]*bt.UTXO, 0)

		utxos, err := b.utxoClient.GetUTXOs(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
		if err != nil {
			return err
		}

		for _, utxo := range utxos {
			if len(utxoSet) >= requestedOutputs {
				break requestedOutputsLoop
			}

			// collect right sized utxos
			if utxo.Satoshis == requestedSatoshisPerOutput {
				utxoSet = append(utxoSet, utxo)
				continue
			}

			// collect uxtos which are greater than requested
			if utxo.Satoshis > requestedSatoshisPerOutput {
				greater = append(greater, utxo)
			}
		}

		b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))

		// create splitting txs
		txsSplitBatches, err := b.splitOutputs(requestedOutputs, requestedSatoshisPerOutput, utxoSet, greater)
		if err != nil {
			return err
		}

		for _, batch := range txsSplitBatches {
			err = b.submitTxs(batch, metamorph_api.Status_SEEN_ON_NETWORK, false)
			if err != nil {
				return err
			}

			// do not performance test ARC when creating the utxos
			time.Sleep(100 * time.Millisecond)
		}
	}

	b.logger.Info("utxo set", slog.Int("ready", len(utxoSet)), slog.Int("requested", requestedOutputs), slog.Uint64("satoshis", requestedSatoshisPerOutput), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))

	return nil
}

func (b *RateBroadcaster) splitOutputs(requestedOutputs int, requestedSatoshisPerOutput uint64, utxoSet []*bt.UTXO, greater []*bt.UTXO) ([][]*bt.Tx, error) {
	txsSplitBatches := make([][]*bt.Tx, 0)
	txsSplit := make([]*bt.Tx, 0)
	outputs := len(utxoSet)
	var err error
	for _, utxo := range greater {
		if outputs >= requestedOutputs {
			break
		}

		tx := bt.NewTx()
		err = tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		addedOutputs, err := b.splitToFundingKeyset(tx, utxo.Satoshis, requestedSatoshisPerOutput, requestedOutputs-outputs)
		if err != nil {
			return nil, err
		}

		outputs += addedOutputs

		txsSplit = append(txsSplit, tx)

		if len(txsSplit) == b.batchSize {
			txsSplitBatches = append(txsSplitBatches, txsSplit)
			txsSplit = make([]*bt.Tx, 0)
		}
	}

	if len(txsSplit) > 0 {
		txsSplitBatches = append(txsSplitBatches, txsSplit)
	}
	return txsSplitBatches, nil
}

func (b *RateBroadcaster) StartRateBroadcaster(rateTxsPerSecond int, limit int, wg *sync.WaitGroup) error {

	_, unconfirmed, err := b.utxoClient.GetBalance(!b.isTestnet, b.fundingKeyset.Address(!b.isTestnet))
	if err != nil {
		return err
	}
	if unconfirmed > 0 {
		return fmt.Errorf("key balance has unconfirmed amount %d sat", unconfirmed)
	}

	utxoSet, err := b.utxoClient.GetUTXOsList(!b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet))
	if err != nil {
		return fmt.Errorf("failed to get utxos: %v", err)
	}

	b.logger.Info("starting broadcasting", slog.Int("rate [txs/s]", rateTxsPerSecond), slog.Int("batch size", b.batchSize), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))

	submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(b.batchSize)

	if submitBatchesPerSecond > millisecondsPerSecond {
		return fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, b.batchSize, submitBatchesPerSecond)
	}

	if utxoSet.Len() < b.batchSize {
		return fmt.Errorf("size of utxo set %d is smaller than requested batch size %d - create more utxos first", utxoSet.Len(), b.batchSize)
	}

	satoshiMap := map[string]uint64{}

	submitBatchInterval := time.Duration(millisecondsPerSecond/float64(submitBatchesPerSecond)) * time.Millisecond
	submitBatchTicker := time.NewTicker(submitBatchInterval)

	logSummaryTicker := time.NewTicker(3 * time.Second)

	responseCh := make(chan *metamorph_api.TransactionStatus, 100)
	errCh := make(chan error, 100)

	var writer *bufio.Writer
	if b.responseWriter != nil {
		writer = bufio.NewWriter(b.responseWriter)
		if err != nil {
			return err
		}
		err = writeJsonArrayStart(writer)
		if err != nil {
			return err
		}
	}

	resultsMap := map[metamorph_api.Status]int64{}
	totalTxs := 0

	counter := 0
	go func() {

		defer func() {
			b.shutdownComplete <- struct{}{}
			wg.Done()
		}()

		for {
			select {
			case <-b.shutdown:
				if writer == nil {
					return
				}

				err = writeJsonArrayFinish(writer)
				if err != nil {
					b.logger.Error("failed to write json array finish", slog.String("err", err.Error()))
				}

				err = writer.Flush()
				if err != nil {
					b.logger.Error("failed flush writer", slog.String("err", err.Error()))
				}

				return
			case <-submitBatchTicker.C:

				txs, err := b.createSelfPayingTxs(utxoSet, satoshiMap)
				if err != nil {
					b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
					b.shutdown <- struct{}{}
					continue
				}

				b.sendTxsBatchAsync(txs, responseCh, errCh, false, metamorph_api.Status_STORED)

			case responseErr := <-errCh:
				b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
			case <-logSummaryTicker.C:

				b.logger.Info("summary",
					slog.String("funding address", b.fundingKeyset.Address(!b.isTestnet)),
					slog.Int64(metamorph_api.Status_STORED.String(), resultsMap[metamorph_api.Status_STORED]),
					slog.Int("utxo set length", utxoSet.Len()),
					slog.Int("total", totalTxs),
				)
			case res := <-responseCh:

				txIDBytes, err := hex.DecodeString(res.Txid)
				if err != nil {
					b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
					continue
				}

				newUtxo := &bt.UTXO{
					TxID:          txIDBytes,
					Vout:          0,
					LockingScript: b.fundingKeyset.Script,
					Satoshis:      satoshiMap[res.Txid],
				}

				delete(satoshiMap, res.Txid)

				utxoSet.PushBack(newUtxo)

				resultsMap[res.Status]++
				counter++

				totalTxs += 1

				if limit > 0 && totalTxs >= limit {
					b.logger.Info("limit reached", slog.Int("total", totalTxs), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))
					b.shutdown <- struct{}{}
				}

				// if writer is given, store the response in the writer
				if writer != nil {
					counter, err = b.writeResponse(writer, res, counter)
					if err != nil {
						b.logger.Error("failed to write response", slog.String("err", err.Error()))
					}
				}
			}
		}
	}()

	return nil
}

func writeJsonArrayStart(writer *bufio.Writer) error {
	_, err := fmt.Fprint(writer, "[")
	if err != nil {
		return fmt.Errorf("failed to print response: %v", err)
	}

	return nil
}

func writeJsonArrayFinish(writer *bufio.Writer) error {
	_, err := fmt.Fprint(writer, "]")
	if err != nil {
		return fmt.Errorf("failed to print response: %v", err)
	}

	return nil
}

func (b *RateBroadcaster) writeResponse(writer *bufio.Writer, res *metamorph_api.TransactionStatus, counter int) (int, error) {

	resBytes, err := json.Marshal(res)
	if err != nil {
		return counter, fmt.Errorf("failed to marshal response: %v", err)
	}

	_, err = fmt.Fprintf(writer, ",%s\n", string(resBytes))
	if err != nil {
		return counter, fmt.Errorf("failed to print response: %v", err)
	}

	if counter%b.responseWriteIterationInterval == 0 {
		err = writer.Flush()
		if err != nil {
			return 0, fmt.Errorf("failed flush writer: %v", err)
		}

		counter = 0
	}

	return counter, nil
}

func (b *RateBroadcaster) Shutdown() {
	b.logger.Info("shutting down broadcaster")

	b.shutdown <- struct{}{}
	<-b.shutdownComplete
}

func (b *RateBroadcaster) createSelfPayingTxs(utxos *list.List, satoshiMap map[string]uint64) ([]*bt.Tx, error) {
	txs := make([]*bt.Tx, 0, b.batchSize)

	var next *list.Element
	for front := utxos.Front(); front != nil; front = next {
		next = front.Next()
		utxos.Remove(front)

		if next == nil {
			return nil, errors.New("list of remaining utxos is empty")
		}

		utxo, ok := next.Value.(*bt.UTXO)
		if !ok {
			return nil, errors.New("failed to parse value to utxo")
		}

		tx := bt.NewTx()

		err := tx.FromUTXOs(utxo)
		if err != nil {
			return nil, err
		}

		fee := b.calculateFeeSat(tx)

		if utxo.Satoshis <= fee {
			continue
		}

		err = tx.PayTo(b.fundingKeyset.Script, utxo.Satoshis-fee)
		if err != nil {
			return nil, err
		}

		unlockerGetter := unlocker.Getter{PrivateKey: b.fundingKeyset.PrivateKey}
		err = tx.FillAllInputs(context.Background(), &unlockerGetter)
		if err != nil {
			return nil, err
		}

		satoshiMap[tx.TxID()] = tx.Outputs[0].Satoshis

		txs = append(txs, tx)

		if len(txs) >= b.batchSize {
			break
		}
	}

	return txs, nil
}

func (b *RateBroadcaster) sendTxsBatchAsync(txs []*bt.Tx, resultCh chan *metamorph_api.TransactionStatus, errCh chan error, skipFeeValidation bool, waitForStatus metamorph_api.Status) {
	go func() {
		resp, err := b.client.BroadcastTransactions(context.Background(), txs, waitForStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, skipFeeValidation)
		if err != nil {
			errCh <- err
		}

		for _, res := range resp {
			resultCh <- res
		}
	}()
}
