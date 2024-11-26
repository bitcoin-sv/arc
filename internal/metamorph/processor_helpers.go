package metamorph

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/libsv/go-p2p/chaincfg/chainhash"

	"github.com/bitcoin-sv/arc/internal/cache"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
)

type StatusUpdateMap map[chainhash.Hash]store.UpdateStatus

var CacheStatusUpdateHash = "mtm-tx-status-update"

var (
	ErrFailedToSerialize   = errors.New("failed to serialize value")
	ErrFailedToDeserialize = errors.New("failed to deserialize value")
)

func (p *Processor) GetProcessorMapSize() int {
	return p.responseProcessor.getMapLen()
}

func (p *Processor) updateStatusMap(statusUpdate store.UpdateStatus) error {
	currentStatusUpdate, err := p.getTransactionStatus(statusUpdate.Hash)
	if err != nil {
		if errors.Is(err, cache.ErrCacheNotFound) {
			p.logger.Info("updateStatusMap - status record not found, creating new one", slog.String("hash", statusUpdate.Hash.String()))
			// if record doesn't exist, save new one
			return p.setTransactionStatus(statusUpdate)
		}
		return err
	}

	if shouldUpdateCompetingTxs(statusUpdate, *currentStatusUpdate) {
		p.logger.Info("updateStatusMap - updating competing txs", slog.String("hash", statusUpdate.Hash.String()))
		currentStatusUpdate.CompetingTxs = mergeUnique(statusUpdate.CompetingTxs, currentStatusUpdate.CompetingTxs)
	}

	if shouldUpdateStatus(statusUpdate, *currentStatusUpdate) {
		p.logger.Info("updateStatusMap - updating status", slog.String("hash", statusUpdate.Hash.String()), slog.String("old_status", currentStatusUpdate.Status.String()), slog.String("new_status", statusUpdate.Status.String()))
		currentStatusUpdate.StatusHistory = append(currentStatusUpdate.StatusHistory, store.StatusWithTimestamp{
			Status:    currentStatusUpdate.Status,
			Timestamp: p.now(),
		})
		currentStatusUpdate.Status = statusUpdate.Status
	}

	p.logger.Info("updateStatusMap - updating status in cache", slog.String("hash", currentStatusUpdate.Hash.String()), slog.String("status", currentStatusUpdate.Status.String()), slog.Any("status_history", currentStatusUpdate.StatusHistory))

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

func shouldUpdateCompetingTxs(new, found store.UpdateStatus) bool {
	if new.Status >= found.Status && !unorderedEqual(new.CompetingTxs, found.CompetingTxs) {
		return true
	}

	return false
}

func shouldUpdateStatus(new, found store.UpdateStatus) bool {
	return new.Status > found.Status
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
