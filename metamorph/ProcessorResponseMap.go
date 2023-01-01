package metamorph

import (
	"log"
	"sync"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
)

type ProcessorResponseMap struct {
	mu     sync.RWMutex
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

	if time.Since(processorResponse.Start) > m.expiry || processorResponse.GetStatus() >= metamorph_api.Status_SEEN_ON_NETWORK {
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

func (m *ProcessorResponseMap) Items() map[string]*ProcessorResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make(map[string]*ProcessorResponse, len(m.items))

	for key, item := range m.items {
		items[key] = item
	}

	return items
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

		if item.status >= metamorph_api.Status_SEEN_ON_NETWORK {
			log.Printf("ProcessorResponseMap: Deleting %s (%s)", key, item.status.String())
			delete(m.items, key)
		}
	}
}
