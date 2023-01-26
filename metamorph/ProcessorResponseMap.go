package metamorph

import (
	"fmt"
	"log"
	"time"
)

type ProcessorResponseMap struct {
	mu     deadloc.RWMutex
	expiry time.Duration
	items  map[string]*ProcessorResponse
}

func NewProcessorResponseMap(expiry time.Duration) *ProcessorResponseMap {
	m := &ProcessorResponseMap{
		expiry: expiry,
		items:  make(map[string]*ProcessorResponse),
	}

	go func() {
		for range time.NewTicker(10 * time.Second).C {
			m.clean()
		}
	}()

	return m
}

func (m *ProcessorResponseMap) Set(key string, value *ProcessorResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = value
}

func (m *ProcessorResponseMap) Get(key string) (*ProcessorResponse, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processorResponse, ok := m.items[key]
	if !ok {
		return nil, false
	}

	if time.Since(processorResponse.Start) > m.expiry {
		return nil, false
	}

	return processorResponse, true
}

func (m *ProcessorResponseMap) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
}

func (m *ProcessorResponseMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.items)
}

func (m *ProcessorResponseMap) Retries(key string) uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.items[key]; !ok {
		return 0
	}

	return m.items[key].Retries()
}

func (m *ProcessorResponseMap) IncrementRetry(key string) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.items[key]; !ok {
		return 0
	}

	return m.items[key].IncrementRetry()
}

// Hashes will return a slice of the hashes in the map.
// If a filter function is provided, only hashes that pass the filter will be returned.
// If no filter function is provided, all hashes will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Hashes(filterFunc ...func(*ProcessorResponse) bool) [][]byte {
	// Default filter function returns true for all items
	fn := func(p *ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	hashes := make([][]byte, 0, m.Len())

	for _, item := range m.items {
		if fn(item) {
			hashes = append(hashes, item.Hash)
		}
	}

	return hashes
}

// Items will return a copy of the map.
// If a filter function is provided, only items that pass the filter will be returned.
// If no filter function is provided, all items will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Items(filterFunc ...func(*ProcessorResponse) bool) map[string]*ProcessorResponse {
	// Default filter function returns true for all items
	fn := func(p *ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	items := make(map[string]*ProcessorResponse, m.Len())

	m.mu.RLock()
	defer m.mu.RUnlock()

	for key, item := range m.items {
		if fn(item) {
			items[key] = item
		}
	}

	return items
}

func (m *ProcessorResponseMap) PrintItems() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, value := range m.items {
		fmt.Printf("tx2ChMap: %s\n", value.String())
	}

}

// Clear clears the map.
func (m *ProcessorResponseMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]*ProcessorResponse)
}

func (m *ProcessorResponseMap) clean() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, item := range m.items {
		if time.Since(item.Start) > m.expiry {
			log.Printf("ProcessorResponseMap: Expired %s", key)
			delete(m.items, key)
		}

	}
}
