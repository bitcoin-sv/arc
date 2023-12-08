package metamorph

import (
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/sasha-s/go-deadlock"
)

const (
	cleanUpInterval    = 15 * time.Minute
	logFilePathDefault = "./data/metamorph.log"
)

type ProcessorResponseMap struct {
	mu            deadlock.RWMutex
	Expiry        time.Duration
	ResponseItems map[chainhash.Hash]*ProcessorResponse
	logFile       string
	logWorker     chan statResponse
	now           func() time.Time
}

func WithNowResponseMap(nowFunc func() time.Time) func(*ProcessorResponseMap) {
	return func(p *ProcessorResponseMap) {
		p.now = nowFunc
	}
}

func WithLogFile(logFile string) func(*ProcessorResponseMap) {
	return func(p *ProcessorResponseMap) {
		p.logFile = logFile
	}
}

type OptionProcRespMap func(p *ProcessorResponseMap)

func NewProcessorResponseMap(expiry time.Duration, opts ...OptionProcRespMap) *ProcessorResponseMap {

	m := &ProcessorResponseMap{
		Expiry:        expiry,
		ResponseItems: make(map[chainhash.Hash]*ProcessorResponse),
		logFile:       logFilePathDefault,
		now:           time.Now,
	}

	// apply options
	for _, opt := range opts {
		opt(m)
	}

	go func() {
		for range time.NewTicker(cleanUpInterval).C {
			m.Clean()
		}
	}()

	// start log write worker
	if m.logFile != "" {
		m.logWorker = make(chan statResponse, 10000)
		go m.logWriter()
	}

	return m
}

func (m *ProcessorResponseMap) logWriter() {
	dir := path.Dir(m.logFile)
	err := os.MkdirAll(dir, 0750)
	if err != nil {
		log.Fatalf("failed to create folder for logging %s", err)
	}
	f, err := os.OpenFile(m.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("error opening log file: %s", err)
	}
	defer f.Close()

	for prl := range m.logWorker {
		var b []byte
		b, err = json.Marshal(prl)
		if err != nil {
			log.Printf("error marshaling log data: %s", err)
			continue
		}

		_, err = f.WriteString(string(b) + "\n")
		if err != nil {
			log.Printf("error writing to log file: %s", err)
			continue
		}
	}
}

func (m *ProcessorResponseMap) Set(hash *chainhash.Hash, value *ProcessorResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResponseItems[*hash] = value
}

func (m *ProcessorResponseMap) Get(hash *chainhash.Hash) (*ProcessorResponse, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processorResponse, ok := m.ResponseItems[*hash]
	if !ok {
		return nil, false
	}

	if m.now().Sub(processorResponse.Start) > m.Expiry {
		return nil, false
	}

	return processorResponse, true
}

func (m *ProcessorResponseMap) Delete(hash *chainhash.Hash) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.ResponseItems[*hash]
	if !ok {
		return
	}

	// append stats to log file
	if m.logFile != "" && m.logWorker != nil {
		announcedPeers := make([]string, 0, len(item.AnnouncedPeers))
		for _, peer := range item.AnnouncedPeers {
			announcedPeers = append(announcedPeers, peer.String())
		}
		m.logWorker <- statResponse{
			Txid:                  item.Hash.String(),
			Start:                 item.Start,
			Retries:               item.Retries.Load(),
			Err:                   item.Err,
			AnnouncedPeers:        announcedPeers,
			Status:                item.Status,
			NoStats:               item.NoStats,
			LastStatusUpdateNanos: item.LastStatusUpdateNanos.Load(),
			Log:                   item.Log,
		}
	}

	delete(m.ResponseItems, *hash)
	// Check if the item was deleted
	// if _, ok := m.ResponseItems[*hash]; ok {
	// 	log.Printf("Failed to delete item from map: %v", hash)
	// }
}

func (m *ProcessorResponseMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.ResponseItems)
}

func (m *ProcessorResponseMap) Retries(hash *chainhash.Hash) uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.ResponseItems[*hash]; !ok {
		return 0
	}

	return m.ResponseItems[*hash].GetRetries()
}

func (m *ProcessorResponseMap) IncrementRetry(hash *chainhash.Hash) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ResponseItems[*hash]; !ok {
		return 0
	}

	return m.ResponseItems[*hash].IncrementRetry()
}

// Hashes will return a slice of the hashes in the map.
// If a filter function is provided, only hashes that pass the filter will be returned.
// If no filter function is provided, all hashes will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Hashes(filterFunc ...func(*ProcessorResponse) bool) [][32]byte {
	// Default filter function returns true for all ResponseItems
	fn := func(p *ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	hashes := make([][32]byte, 0, len(m.ResponseItems))

	for _, item := range m.ResponseItems {
		if fn(item) {
			var h [32]byte
			copy(h[:], item.Hash[:])

			hashes = append(hashes, h)
		}
	}

	return hashes
}

// Items will return a copy of the map.
// If a filter function is provided, only ResponseItems that pass the filter will be returned.
// If no filter function is provided, all ResponseItems will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Items(filterFunc ...func(*ProcessorResponse) bool) map[chainhash.Hash]*ProcessorResponse {
	// Default filter function returns true for all ResponseItems
	fn := func(p *ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make(map[chainhash.Hash]*ProcessorResponse, len(m.ResponseItems))

	for hash, item := range m.ResponseItems {
		if fn(item) {
			items[hash] = item
		}
	}

	return items
}

func (m *ProcessorResponseMap) logMapItems(logger *slog.Logger) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for hash, processorResponse := range m.ResponseItems {
		logger.Debug("Processor response map item", slog.String("hash", hash.String()), slog.String("status", processorResponse.Status.String()), slog.Int("retries", int(processorResponse.Retries.Load())), slog.String("err", processorResponse.Err.Error()), slog.Time("start", processorResponse.Start))
	}
}

// Clear clears the map.
func (m *ProcessorResponseMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ResponseItems = make(map[chainhash.Hash]*ProcessorResponse)
}

func (m *ProcessorResponseMap) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.ResponseItems {
		item.Close()
	}
}

func (m *ProcessorResponseMap) Clean() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, item := range m.ResponseItems {
		if time.Since(item.Start) > m.Expiry {
			log.Printf("ProcessorResponseMap: Expired %s", key)
			item.Close()
			delete(m.ResponseItems, key)
		}
	}
}
