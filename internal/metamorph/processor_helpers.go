package metamorph

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-bt/v2/chainhash"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/callbacker/callbacker_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/bitcoin-sv/arc/pkg/tracing"
)

type StatusUpdateMap map[chainhash.Hash]store.UpdateStatus

var CacheStatusUpdateHash = "mtm-tx-status-update"

var ErrFailedToSerialize = errors.New("failed to serialize value")

func (p *Processor) GetProcessorMapSize() int {
	return p.responseProcessor.getMapLen()
}

func (p *Processor) updateStatusMap(statusUpdate store.UpdateStatus) error {
	currentStatusUpdate, err := p.getTransactionStatus(statusUpdate.Hash)
	if err != nil {
		if errors.Is(err, cache.ErrCacheNotFound) {
			// if the record doesn't exist, store a new one
			return p.setTransactionStatus(statusUpdate)
		}
		return err
	}

	if shouldUpdateCompetingTxs(statusUpdate, *currentStatusUpdate) {
		currentStatusUpdate.CompetingTxs = mergeUnique(statusUpdate.CompetingTxs, currentStatusUpdate.CompetingTxs)
	}

	if shouldUpdateStatus(statusUpdate, *currentStatusUpdate) {
		currentStatusUpdate.StatusHistory = append(currentStatusUpdate.StatusHistory, store.NewStatusWithTimestamp(currentStatusUpdate.Status, currentStatusUpdate.Timestamp))
		currentStatusUpdate.Status = statusUpdate.Status
		currentStatusUpdate.Timestamp = statusUpdate.Timestamp
	}

	return p.setTransactionStatus(*currentStatusUpdate)
}

func (p *Processor) setTransactionStatus(status store.UpdateStatus) error {
	bytes, err := json.Marshal(status)
	if err != nil {
		return errors.Join(ErrFailedToSerialize, err)
	}

	err = p.cacheStore.MapSet(CacheStatusUpdateHash, status.Hash.String(), bytes)
	if err != nil {
		return err
	}
	return nil
}

func (p *Processor) getTransactionStatus(hash chainhash.Hash) (*store.UpdateStatus, error) {
	bytes, err := p.cacheStore.MapGet(CacheStatusUpdateHash, hash.String())
	if err != nil {
		return nil, err
	}

	var status store.UpdateStatus
	err = json.Unmarshal(bytes, &status)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

func (p *Processor) getAndDeleteAllTransactionStatuses() (StatusUpdateMap, error) {
	statuses := make(StatusUpdateMap)
	keys, err := p.cacheStore.MapExtractAll(CacheStatusUpdateHash)
	if err != nil {
		return nil, err
	}

	for key, value := range keys {
		hash, err := chainhash.NewHashFromStr(key)
		if err != nil {
			p.logger.Error("failed to convert hash from key", slog.String("error", err.Error()), slog.String("key", key))
			continue
		}

		var status store.UpdateStatus
		err = json.Unmarshal(value, &status)
		if err != nil {
			p.logger.Error("failed to unmarshal status", slog.String("error", err.Error()), slog.String("key", key))
			continue
		}

		statuses[*hash] = status
	}

	return statuses, nil
}

func (p *Processor) getStatusUpdateCount() (int, error) {
	count, err := p.cacheStore.MapLen(CacheStatusUpdateHash)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

func shouldUpdateCompetingTxs(newStatus, found store.UpdateStatus) bool {
	if newStatus.Status >= found.Status && !unorderedEqual(newStatus.CompetingTxs, found.CompetingTxs) {
		return true
	}

	return false
}

func shouldUpdateStatus(newStatus, found store.UpdateStatus) bool {
	return newStatus.Status > found.Status
}

// unorderedEqual checks if two string slices contain
// the same elements, regardless of order
func unorderedEqual(sliceOne, sliceTwo []string) bool {
	if len(sliceOne) != len(sliceTwo) {
		return false
	}

	exists := make(map[string]bool)

	for _, value := range sliceOne {
		exists[value] = true
	}

	for _, value := range sliceTwo {
		if !exists[value] {
			return false
		}
	}

	return true
}

// mergeUnique merges two string arrays into one with unique values
func mergeUnique(arr1, arr2 []string) []string {
	valueSet := make(map[string]struct{})

	for _, value := range arr1 {
		valueSet[value] = struct{}{}
	}

	for _, value := range arr2 {
		valueSet[value] = struct{}{}
	}

	uniqueSlice := make([]string, 0, len(valueSet))
	for key := range valueSet {
		uniqueSlice = append(uniqueSlice, key)
	}

	return uniqueSlice
}

func toSendRequest(d *store.TransactionData, timestamp time.Time) []*callbacker_api.SendRequest {
	if len(d.Callbacks) == 0 {
		return nil
	}

	requests := make([]*callbacker_api.SendRequest, 0, len(d.Callbacks))
	for _, c := range d.Callbacks {
		if c.CallbackURL != "" {
			routing := &callbacker_api.CallbackRouting{
				Url:        c.CallbackURL,
				Token:      c.CallbackToken,
				AllowBatch: c.AllowBatch,
			}

			request := &callbacker_api.SendRequest{
				CallbackRouting: routing,

				Txid:         d.Hash.String(),
				Status:       callbacker_api.Status(d.Status),
				MerklePath:   d.MerklePath,
				ExtraInfo:    getCallbackExtraInfo(d),
				CompetingTxs: getCallbackCompetitingTxs(d),

				BlockHash:   getCallbackBlockHash(d),
				BlockHeight: d.BlockHeight,

				Timestamp: timestamppb.New(timestamp),
			}
			requests = append(requests, request)
		}
	}

	return requests
}

func getCallbackExtraInfo(d *store.TransactionData) string {
	if d.Status == metamorph_api.Status_MINED && len(d.CompetingTxs) > 0 {
		return minedDoubleSpendMsg
	}

	return d.RejectReason
}

func getCallbackCompetitingTxs(d *store.TransactionData) []string {
	if d.Status == metamorph_api.Status_MINED {
		return nil
	}

	return d.CompetingTxs
}

func getCallbackBlockHash(d *store.TransactionData) string {
	if d.BlockHash == nil {
		return ""
	}

	return d.BlockHash.String()
}

func (p *Processor) StartRoutine(tickerInterval time.Duration, routine func(context.Context, *Processor) []attribute.KeyValue, routineName string) {
	ticker := time.NewTicker(tickerInterval)
	p.wg.Add(1)

	go func() {
		defer func() {
			p.wg.Done()
			ticker.Stop()
		}()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				ctx, span := tracing.StartTracing(p.ctx, routineName, p.tracingEnabled, p.tracingAttributes...)
				attr := routine(ctx, p)
				if span != nil && len(attr) > 0 {
					span.SetAttributes(attr...)
				}
				tracing.EndTracing(span, nil)
			}
		}
	}()
}

func txBytesFromHex(txs []string) ([][]byte, error) {
	if len(txs) == 0 {
		return nil, nil
	}

	hashes := make([][]byte, 0, len(txs))
	for _, tx := range txs {
		hash, err := hex.DecodeString(tx)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}

	return hashes, nil
}
