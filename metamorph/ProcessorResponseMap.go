package metamorph

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
	"github.com/sasha-s/go-deadlock"
)

type processorResponseStats struct {
	key   string
	stats map[int32]int64
}

type ProcessorResponseMap struct {
	mu        deadlock.RWMutex
	expiry    time.Duration
	items     map[string]*ProcessorResponse
	logFile   string
	logWorker chan processorResponseStats
}

func NewProcessorResponseMap(expiry time.Duration) *ProcessorResponseMap {
	logFile, _ := gocore.Config().Get("metamorph_logFile") //, "./data/metamorph.log")

	m := &ProcessorResponseMap{
		expiry:    expiry,
		items:     make(map[string]*ProcessorResponse),
		logFile:   logFile,
		logWorker: make(chan processorResponseStats, 10000),
	}

	go func() {
		for range time.NewTicker(10 * time.Second).C {
			m.clean()
		}
	}()

	// start log write worker
	if m.logFile != "" {
		go func() {
			f, err := os.OpenFile(m.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Printf("error opening log file: %s", err.Error())
			}
			defer f.Close()

			// the status cols will not always be in order if we use the proto map definition
			statusCols := []int32{0, 1, 2, 3, 4, 5, 6, 7, 108, 109}

			for prs := range m.logWorker {
				var statsTimes []string
				for _, status := range statusCols {
					if s, ok := prs.stats[status]; ok {
						statsTimes = append(statsTimes, strconv.Itoa(int(s)))
					} else {
						// add missing value (empty string)
						statsTimes = append(statsTimes, "")
					}
				}

				_, err = f.WriteString(fmt.Sprintf("%s\t%s\n", prs.key, strings.Join(statsTimes, "\t")))
				if err != nil {
					log.Printf("error writing to log file: %s", err.Error())
				}
			}
		}()
	} else {
		// close the channel if we're not using it, this will prevent the Delete to block
		close(m.logWorker)
	}

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

	// append stats to log file
	if m.logFile != "" {
		utils.SafeSend(m.logWorker, processorResponseStats{
			key:   key,
			stats: m.items[key].GetStats(),
		})
	}

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
