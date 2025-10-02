package broadcaster

import (
	"context"
	cRand "crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/pkg/keyset"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

var (
	ErrFailedToGetBalance       = errors.New("failed to get balance")
	ErrKeyHasUnconfirmedBalance = errors.New("key has unconfirmed balance")
	ErrFailedToGetUTXOs         = errors.New("failed to get utxos")
	ErrTooSmallUTXOSet          = errors.New("utxo set is too small")
	ErrFailedToAddInput         = errors.New("failed to add input")
	ErrFailedToAddOutput        = errors.New("failed to add output")
	ErrNotEnoughUTXOs           = errors.New("not enough utxos with sufficient funds left")
	ErrNotEnoughUTXOsForBatch   = errors.New("not enough utxos with sufficient funds left for another batch")
	ErrFailedToFillInputs       = errors.New("failed to fill inputs")
)

type UTXORateBroadcaster struct {
	Broadcaster
	totalTxs        int64
	connectionCount int64
	utxoCh          chan *sdkTx.UTXO
	wg              sync.WaitGroup
	satoshiMap      sync.Map
	ks              *keyset.KeySet
	limit           int64
	ticker          Ticker
}

type Ticker interface {
	GetTickerCh() <-chan time.Time
}

func NewRateBroadcaster(logger *slog.Logger, client ArcClient, ks *keyset.KeySet, utxoClient UtxoClient, limit int64, ticker Ticker, opts ...func(p *Broadcaster)) (*UTXORateBroadcaster, error) {
	b, err := NewBroadcaster(logger, client, utxoClient, opts...)
	if err != nil {
		return nil, err
	}
	rb := &UTXORateBroadcaster{
		Broadcaster:     b,
		utxoCh:          nil,
		wg:              sync.WaitGroup{},
		satoshiMap:      sync.Map{},
		ks:              ks,
		totalTxs:        0,
		connectionCount: 0,
		limit:           limit,
		ticker:          ticker,
	}

	return rb, nil
}

func (b *UTXORateBroadcaster) Initialize(ctx context.Context) error {
	const wocBackoff = 10 * time.Second
	const wocRetries = 5

	_, unconfirmed, err := b.utxoClient.GetBalanceWithRetries(ctx, b.ks.Address(!b.isTestnet), wocBackoff, wocRetries)
	if err != nil {
		return errors.Join(ErrFailedToGetBalance, err)
	}
	if math.Abs(float64(unconfirmed)) > 0 {
		return errors.Join(ErrKeyHasUnconfirmedBalance, fmt.Errorf("address %s, unconfirmed amount %d", b.ks.Address(!b.isTestnet), unconfirmed))
	}

	utxoSet, err := b.utxoClient.GetUTXOsWithRetries(b.ctx, b.ks.Address(!b.isTestnet), wocBackoff, wocRetries)
	if err != nil {
		return errors.Join(ErrFailedToGetUTXOs, err)
	}

	if len(utxoSet) < b.batchSize {
		return errors.Join(ErrTooSmallUTXOSet, fmt.Errorf("size of utxo set %d is smaller than requested batch size %d - create more utxos first", len(utxoSet), b.batchSize))
	}

	b.utxoCh = make(chan *sdkTx.UTXO, 100000)
	for _, utxo := range utxoSet {
		b.utxoCh <- utxo
	}

	return nil
}

func (b *UTXORateBroadcaster) Start() {
	tickerCh := b.ticker.GetTickerCh()
	errCh := make(chan error, 100)

	b.logger.Info("start broadcasting")

	b.wg.Add(1)
	go func() {
		defer func() {
			b.wg.Done()
		}()

	outerLoop:
		for {
			select {
			case <-b.ctx.Done():
				return
			case <-tickerCh:
				b.logger.Debug("tick")

				txs, err := b.createSelfPayingTxs()
				if err != nil {
					b.logger.Error("failed to create self paying txs", slog.String("err", err.Error()))
					break outerLoop
				}

				if b.limit > 0 && atomic.LoadInt64(&b.totalTxs) >= b.limit {
					b.logger.Info("limit reached", slog.Int64("total", atomic.LoadInt64(&b.totalTxs)), slog.Int64("limit", b.limit))
					break outerLoop
				}

				b.broadcastBatchAsync(txs, errCh, b.waitForStatus)

			case responseErr := <-errCh:
				b.logger.Error("failed to submit transactions", slog.String("err", responseErr.Error()))
			}
		}
	}()
}

func (b *UTXORateBroadcaster) createSelfPayingTxs() (sdkTx.Transactions, error) {
	txs := make(sdkTx.Transactions, 0, b.batchSize)

utxoLoop:
	for {
		select {
		case <-b.ctx.Done():
			return txs, nil
		case utxo := <-b.utxoCh:
			tx, err := b.createSelfPayingTx(utxo)
			if err != nil {
				return nil, err
			}
			if tx == nil {
				continue
			}
			txs = append(txs, tx)

			if len(txs) >= b.batchSize {
				break utxoLoop
			}
		}
	}

	return txs, nil
}

func (b *UTXORateBroadcaster) createSelfPayingTx(utxo *sdkTx.UTXO) (*sdkTx.Transaction, error) {
	tx := sdkTx.NewTransaction()
	amount := utxo.Satoshis

	err := tx.AddInputsFromUTXOs(utxo)
	if err != nil {
		return nil, errors.Join(ErrFailedToAddInput, err)
	}

	if b.opReturn != "" {
		err = tx.AddOpReturnOutput([]byte(b.opReturn))
		if err != nil {
			return nil, fmt.Errorf("failed to add OP_RETURN output: %v", err)
		}
	}

	if b.sizeJitterMax > 0 {
		err = addRandomDataInputs(b, &tx, &amount)
		if err != nil {
			return nil, err
		}
	}

	fee, err := ComputeFee(tx, b.feeModel)
	if err != nil {
		return nil, err
	}

	if amount <= fee {
		if len(b.utxoCh) == 0 {
			return nil, ErrNotEnoughUTXOs
		}

		if len(b.utxoCh) < b.batchSize {
			return nil, ErrNotEnoughUTXOsForBatch
		}

		return nil, nil
	}
	amount -= fee

	err = PayTo(tx, b.ks.Script, amount)
	if err != nil {
		return nil, errors.Join(ErrFailedToAddOutput, err)
	}

	err = SignAllInputs(tx, b.ks.PrivateKey)
	if err != nil {
		return nil, errors.Join(ErrFailedToFillInputs, err)
	}

	b.satoshiMap.Store(tx.TxID(), tx.Outputs[0].Satoshis)
	return tx, nil
}

func addRandomDataInputs(b *UTXORateBroadcaster, tx **sdkTx.Transaction, amount *uint64) error {
	// Add additional inputs to the transaction
	const maxRandomInputs = 10
	randInt, err := cRand.Int(cRand.Reader, big.NewInt(maxRandomInputs))
	if err != nil {
		return fmt.Errorf("failed to generate random number: %v", err)
	}
	numOfInputs := randInt.Int64()

	for i := int64(0); i < numOfInputs; i++ {
		additionalUtxo, ok := <-b.utxoCh
		if !ok {
			return ErrNotEnoughUTXOsForBatch
		}

		err = (*tx).AddInputsFromUTXOs(additionalUtxo)
		if err != nil {
			return errors.Join(ErrFailedToAddInput, err)
		}

		*amount += additionalUtxo.Satoshis
	}

	// Add additional OP_RETURN with random data to the transaction

	randJitter, err := cRand.Int(cRand.Reader, big.NewInt(b.sizeJitterMax))
	if err != nil {
		return fmt.Errorf("failed to generate random number: %v", err)
	}
	dataSize := randJitter.Int64()

	randomBytes := make([]byte, dataSize)
	_, err = cRand.Read(randomBytes)
	if err != nil {
		return fmt.Errorf("failed to fill OP_RETURN with random bytes: %v", err)
	}

	testHeader := []byte(" sizeJitter random bytes - ")
	if b.opReturn != "" {
		testHeader = append([]byte(b.opReturn), testHeader...)
	}

	err = (*tx).AddOpReturnOutput(append(testHeader, randomBytes...))
	if err != nil {
		return fmt.Errorf("failed to add OP_RETURN output: %v", err)
	}
	return nil
}

func (b *UTXORateBroadcaster) broadcastBatchAsync(txs sdkTx.Transactions, errCh chan error, waitForStatus metamorph_api.Status) {
	b.wg.Go(func() {
		ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
		defer cancel()

		atomic.AddInt64(&b.connectionCount, 1)

		resp, err := b.client.BroadcastTransactions(ctx, txs, waitForStatus, b.callbackURL, b.callbackToken, b.fullStatusUpdates, false)
		if err != nil {
			// In case of error, put utxos back in the channel
			b.putUTXOSBackInChannel(txs)
			if errors.Is(err, context.Canceled) {
				atomic.AddInt64(&b.connectionCount, -1)
				return
			}
			errCh <- err
		}

		atomic.AddInt64(&b.connectionCount, -1)
		b.putNewUTXOSInChannel(resp)
	})
}
func (b *UTXORateBroadcaster) putUTXOSBackInChannel(txs sdkTx.Transactions) {
	for _, tx := range txs {
		for _, input := range tx.Inputs {
			unusedUtxo := &sdkTx.UTXO{
				TxID:          input.SourceTXID,
				Vout:          0,
				LockingScript: b.ks.Script,
				Satoshis:      *input.SourceTxSatoshis(),
			}
			b.utxoCh <- unusedUtxo
		}
	}
}

func (b *UTXORateBroadcaster) putNewUTXOSInChannel(resp []*metamorph_api.TransactionStatus) {
	for _, res := range resp {
		sat, found := b.satoshiMap.Load(res.Txid)
		satoshis, isValid := sat.(uint64)

		hash, err := chainhash.NewHashFromHex(res.Txid)
		if err != nil {
			b.logger.Error("failed to create chainhash txid", slog.String("err", err.Error()))
			continue
		}

		if found && isValid {
			newUtxo := &sdkTx.UTXO{
				TxID:          hash,
				Vout:          0,
				LockingScript: b.ks.Script,
				Satoshis:      satoshis,
			}
			b.utxoCh <- newUtxo
		}

		b.satoshiMap.Delete(res.Txid)

		atomic.AddInt64(&b.totalTxs, 1)
	}
}

func (b *UTXORateBroadcaster) Shutdown() {
	b.cancelAll()

	b.wg.Wait()
}

func (b *UTXORateBroadcaster) Wait() {
	b.wg.Wait()
}

func (b *UTXORateBroadcaster) GetLimit() int64 {
	return b.limit
}

func (b *UTXORateBroadcaster) GetTxCount() int64 {
	return atomic.LoadInt64(&b.totalTxs)
}

func (b *UTXORateBroadcaster) GetConnectionCount() int64 {
	return atomic.LoadInt64(&b.connectionCount)
}

func (b *UTXORateBroadcaster) GetUtxoSetLen() int {
	return len(b.utxoCh)
}
