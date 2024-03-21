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

	"github.com/bitcoin-sv/arc/pkg/keyset"
	"github.com/bitcoin-sv/arc/pkg/metamorph/metamorph_api"
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
	GetUTXOs(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) ([]*bt.UTXO, error)
	GetUTXOsWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) ([]*bt.UTXO, error)
	GetUTXOsList(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string) (*list.List, error)
	GetUTXOsListWithRetries(ctx context.Context, mainnet bool, lockingScript *bscript.Script, address string, constantBackoff time.Duration, retries uint64) (*list.List, error)
	GetBalance(ctx context.Context, mainnet bool, address string) (int64, int64, error)
	GetBalanceWithRetries(ctx context.Context, mainnet bool, address string, constantBackoff time.Duration, retries uint64) (int64, int64, error)
	TopUp(ctx context.Context, mainnet bool, address string) error
}

type RateBroadcaster struct {
	logger                         *slog.Logger
	client                         ArcClient
	fundingKeyset                  *keyset.KeySet
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

	mu         sync.RWMutex
	satoshiMap map[string]uint64
	totalTxs   int64
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

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, fromKeySet *keyset.KeySet, utxoClient UtxoClient, opts ...func(p *RateBroadcaster)) (*RateBroadcaster, error) {
	broadcaster := &RateBroadcaster{
		logger:                         logger,
		client:                         client,
		fundingKeyset:                  fromKeySet,
		isTestnet:                      isTestnetDefault,
		feeQuote:                       bt.NewFeeQuote(),
		utxoClient:                     utxoClient,
		batchSize:                      batchSizeDefault,
		maxInputs:                      maxInputsDefault,
		responseWriter:                 nil,
		responseWriteIterationInterval: resultsIterationsDefault,

		shutdown:         make(chan struct{}, 10),
		shutdownComplete: make(chan struct{}, 10),
		satoshiMap:       map[string]uint64{},
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

func (b *RateBroadcaster) Consolidate(ctx context.Context) error {
	_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return err
	}
	if math.Abs(float64(unconfirmed)) > 0 {
		return fmt.Errorf("key with address %s balance has unconfirmed amount %d sat", b.fundingKeyset.Address(!b.isTestnet), unconfirmed)
	}

	utxoSet, err := b.utxoClient.GetUTXOsListWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return fmt.Errorf("failed to get utxos: %v", err)
	}

	if utxoSet.Len() == 1 {
		b.logger.Info("utxos already consolidated")
		return nil
	}

	satoshiMap := map[string]uint64{}
	b.logger.Info("consolidating outputs", slog.Int("outputs", utxoSet.Len()))
	consolidationTxsBatches, err := b.createConsolidationTxs(utxoSet, satoshiMap)
	if err != nil {
		return fmt.Errorf("failed to create consolidation txs: %v", err)
	}

	for len(consolidationTxsBatches) > 0 {

		b.logger.Info("consolidating outputs", slog.Int("outputs", utxoSet.Len()))

		for _, batch := range consolidationTxsBatches {
			time.Sleep(1000 * time.Millisecond) // do not performance test ARC
			resp, err := b.client.BroadcastTransactions(context.Background(), batch, metamorph_api.Status_SEEN_ON_NETWORK, b.callbackURL, b.callbackToken, b.fullStatusUpdates, false)
			if err != nil {
				return fmt.Errorf("failed to broadcast consolidation txs: %v", err)
			}

			for _, res := range resp {
				if res.Status == metamorph_api.Status_REJECTED || res.Status == metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL {
					return fmt.Errorf("consolidation tx %s was not successful: %s", res.Txid, res.Status.String())
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

		b.logger.Info("consolidating outputs", slog.Int("outputs", utxoSet.Len()))

		consolidationTxsBatches, err = b.createConsolidationTxs(utxoSet, satoshiMap)
		if err != nil {
			return fmt.Errorf("failed to create consolidation txs: %v", err)
		}
	}

	return nil
}

func (b *RateBroadcaster) CreateUtxos(ctx context.Context, requestedOutputs int, requestedSatoshisPerOutput uint64) error {

	requestedOutputsSatoshis := int64(requestedOutputs) * int64(requestedSatoshisPerOutput)

	confirmed, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
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

		utxos, err := b.utxoClient.GetUTXOsWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
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

	// Todo: create txs with maximum number of outputs per tx

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

func (b *RateBroadcaster) StartRateBroadcaster(ctx context.Context, rateTxsPerSecond int, limit int64, wg *sync.WaitGroup) error {

	_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return err
	}
	if math.Abs(float64(unconfirmed)) > 0 {
		return fmt.Errorf("key with address %s balance has unconfirmed amount %d sat", b.fundingKeyset.Address(!b.isTestnet), unconfirmed)
	}

	utxoSet, err := b.utxoClient.GetUTXOsWithRetries(ctx, !b.isTestnet, b.fundingKeyset.Script, b.fundingKeyset.Address(!b.isTestnet), 1*time.Second, 5)
	if err != nil {
		return fmt.Errorf("failed to get utxos: %v", err)
	}

	b.logger.Info("starting broadcasting", slog.Int("rate [txs/s]", rateTxsPerSecond), slog.Int("batch size", b.batchSize), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))

	submitBatchesPerSecond := float64(rateTxsPerSecond) / float64(b.batchSize)

	if submitBatchesPerSecond > millisecondsPerSecond {
		return fmt.Errorf("submission rate %d [txs/s] and batch size %d [txs] result in submission frequency %.2f greater than 1000 [/s]", rateTxsPerSecond, b.batchSize, submitBatchesPerSecond)
	}

	if len(utxoSet) < b.batchSize {
		return fmt.Errorf("size of utxo set %d is smaller than requested batch size %d - create more utxos first", len(utxoSet), b.batchSize)
	}

	utxoCh := make(chan *bt.UTXO, len(utxoSet))

	for _, utxo := range utxoSet {
		utxoCh <- utxo
	}

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

				txs, err := b.createSelfPayingTxs(utxoCh)
				if err != nil {
					b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
					b.shutdown <- struct{}{}
					continue
				}

				b.sendTxsBatchAsyncBroadcast(txs, responseCh, errCh, utxoCh, false, metamorph_api.Status_STORED, limit)

			case <-logSummaryTicker.C:

				b.mu.Lock()
				totalTxs := b.totalTxs
				b.mu.Unlock()

				b.logger.Info("summary",
					slog.String("funding address", b.fundingKeyset.Address(!b.isTestnet)),
					slog.Int("utxo set length", len(utxoCh)),
					slog.Int64("total", totalTxs),
				)

			case responseErr := <-errCh:
				b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
				if writer == nil {
					continue
				}

				counter++
				err = b.write(&counter, writer, fmt.Sprintf(",%s\n", responseErr.Error()))
				if err != nil {
					b.logger.Error("failed to write", slog.String("err", err.Error()))
					continue
				}
			case res := <-responseCh:
				resultsMap[res.Status]++
				if writer == nil {
					continue
				}
				// if writer is given, store the response in the writer
				counter++
				resBytes, err := json.Marshal(res)
				if err != nil {
					b.logger.Error("failed to marshal response", slog.String("err", err.Error()))
					continue
				}

				err = b.write(&counter, writer, fmt.Sprintf(",%s\n", string(resBytes)))
				if err != nil {
					b.logger.Error("failed to write", slog.String("err", err.Error()))
					continue
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

func (b *RateBroadcaster) write(counter *int, writer *bufio.Writer, content string) error {
	_, err := fmt.Fprint(writer, content)
	if err != nil {
		return fmt.Errorf("failed to print response: %v", err)
	}

	if *counter%b.responseWriteIterationInterval == 0 {
		err = writer.Flush()
		if err != nil {
			return fmt.Errorf("failed flush writer: %v", err)
		}

		*counter = 0
	}

	return nil
}

func (b *RateBroadcaster) Shutdown() {
	b.logger.Info("shutting down broadcaster")

	b.shutdown <- struct{}{}
	<-b.shutdownComplete
}

func (b *RateBroadcaster) createSelfPayingTxs(utxos chan *bt.UTXO) ([]*bt.Tx, error) {
	txs := make([]*bt.Tx, 0, b.batchSize)

	for utxo := range utxos {
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

		b.mu.Lock()
		b.satoshiMap[tx.TxID()] = tx.Outputs[0].Satoshis
		b.mu.Unlock()
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

func (b *RateBroadcaster) sendTxsBatchAsyncBroadcast(txs []*bt.Tx, resultCh chan *metamorph_api.TransactionStatus, errCh chan error, utxoCh chan *bt.UTXO, skipFeeValidation bool, waitForStatus metamorph_api.Status, limit int64) {
	go func() {
		resp, err := b.client.BroadcastTransactions(context.Background(), txs, waitForStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, skipFeeValidation)
		if err != nil {
			errCh <- err
		}

		for _, res := range resp {
			resultCh <- res

			txIDBytes, err := hex.DecodeString(res.Txid)
			if err != nil {
				b.logger.Error("failed to decode txid", slog.String("err", err.Error()))
				continue
			}
			b.mu.Lock()
			newUtxo := &bt.UTXO{
				TxID:          txIDBytes,
				Vout:          0,
				LockingScript: b.fundingKeyset.Script,
				Satoshis:      b.satoshiMap[res.Txid],
			}

			delete(b.satoshiMap, res.Txid)
			b.totalTxs++
			if limit > 0 && b.totalTxs >= limit {
				b.logger.Info("limit reached", slog.Int64("total", b.totalTxs), slog.String("address", b.fundingKeyset.Address(!b.isTestnet)))
				b.shutdown <- struct{}{}
			}
			b.mu.Unlock()

			utxoCh <- newUtxo
		}
	}()
}
