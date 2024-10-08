package metamorph

import (
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

func (p *Processor) GetProcessorMapSize() int {
	return p.responseProcessor.getMapLen()
}

func updateStatusMap(statusUpdatesMap map[chainhash.Hash]store.UpdateStatus, statusUpdate store.UpdateStatus) {
	foundStatusUpdate, found := statusUpdatesMap[statusUpdate.Hash]

	if !found || shouldUpdateStatus(statusUpdate, foundStatusUpdate) {
		if len(statusUpdate.CompetingTxs) > 0 {
			statusUpdate.CompetingTxs = mergeUnique(statusUpdate.CompetingTxs, foundStatusUpdate.CompetingTxs)
		}

		statusUpdatesMap[statusUpdate.Hash] = statusUpdate
	}
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
