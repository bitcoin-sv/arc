package metamorph

import (
	"encoding/json"
	"errors"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

const CacheStatusUpdateKey = "status-updates"

var ErrFailedToSerialize = errors.New("failed to serialize value")

func (p *Processor) GetProcessorMapSize() int {
	return p.responseProcessor.getMapLen()
}

func (p *Processor) updateStatusMap(statusUpdate store.UpdateStatus) (map[chainhash.Hash]store.UpdateStatus, error) {
	statusUpdatesMap := p.getStatusUpdateMap()

	foundStatusUpdate, found := statusUpdatesMap[statusUpdate.Hash]

	if !found || shouldUpdateStatus(statusUpdate, foundStatusUpdate) {
		if len(statusUpdate.CompetingTxs) > 0 {
			statusUpdate.CompetingTxs = mergeUnique(statusUpdate.CompetingTxs, foundStatusUpdate.CompetingTxs)
		}

		statusUpdatesMap[statusUpdate.Hash] = statusUpdate
	}

	err := p.setStatusUpdateMap(statusUpdatesMap)
	if err != nil {
		return nil, err
	}

	return statusUpdatesMap, nil
}

func (p *Processor) setStatusUpdateMap(statusUpdatesMap map[chainhash.Hash]store.UpdateStatus) error {
	bytes, err := serialize(statusUpdatesMap)
	if err != nil {
		return err
	}

	err = p.cacheStore.Set(CacheStatusUpdateKey, bytes, processStatusUpdatesIntervalDefault)
	if err != nil {
		return err
	}
	return nil
}

func (p *Processor) getStatusUpdateMap() map[chainhash.Hash]store.UpdateStatus {
	existingMap, err := p.cacheStore.Get(CacheStatusUpdateKey)
	var statusUpdatesMap map[chainhash.Hash]store.UpdateStatus

	if err == nil {
		err = json.Unmarshal(existingMap, &statusUpdatesMap)
		if err == nil {
			return statusUpdatesMap
		}
	}

	// If the key doesn't exist or there was an error unmarshalling the value return new map
	statusUpdatesMap = make(map[chainhash.Hash]store.UpdateStatus)

	return statusUpdatesMap
}

func shouldUpdateStatus(new, found store.UpdateStatus) bool {
	if new.Status > found.Status {
		return true
	}

	if new.Status == found.Status && !unorderedEqual(new.CompetingTxs, found.CompetingTxs) {
		return true
	}

	return false
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

func serialize(value interface{}) ([]byte, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrFailedToSerialize, err)
	}
	return bytes, nil
}
