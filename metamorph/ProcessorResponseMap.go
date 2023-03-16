package metamorph

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/processor_response"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/gocore"
)

type ProcessorResponseMap struct {
	expiry      time.Duration
	items       sync.Map
	itemsLength atomic.Int64
	logFile     string
	logWorker   chan statResponse
}

func NewProcessorResponseMap(expiry time.Duration) *ProcessorResponseMap {
	logFile, _ := gocore.Config().Get("metamorph_logFile") //, "./data/metamorph.log")

	m := &ProcessorResponseMap{
		expiry:  expiry,
		logFile: logFile,
	}

	go func() {
		for range time.NewTicker(1 * time.Hour).C {
			m.clean()
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
	_ = os.MkdirAll(dir, 0777)

	f, err := os.OpenFile(m.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("error opening log file: %s", err.Error())
	}
	defer f.Close()

	for prl := range m.logWorker {

		var b []byte
		b, err = json.Marshal(prl)
		if err != nil {
			log.Printf("error marshaling log data: %s", err.Error())
		}

		_, err = f.WriteString(string(b) + "\n")
		if err != nil {
			log.Printf("error writing to log file: %s", err.Error())
		}
	}
}

func (m *ProcessorResponseMap) Set(hash *chainhash.Hash, value *processor_response.ProcessorResponse) {
	m.items.Store(*hash, value)
	m.itemsLength.Add(1)
}

func (m *ProcessorResponseMap) Get(hash *chainhash.Hash) (*processor_response.ProcessorResponse, bool) {
	item, ok := m.items.Load(*hash)
	if !ok {
		return nil, false
	}

	processorResponse, ok := item.(*processor_response.ProcessorResponse)
	if !ok {
		return nil, false
	}

	if time.Since(processorResponse.Start) > m.expiry {
		return nil, false
	}

	return processorResponse, true
}

func (m *ProcessorResponseMap) Delete(hash *chainhash.Hash) {
	item, ok := m.items.Load(*hash)
	if !ok {
		return
	}

	// append stats to log file
	if m.logFile != "" && m.logWorker != nil {
		processorResponse, ok := item.(*processor_response.ProcessorResponse)
		if !ok {
			return
		}

		announcedPeers := make([]string, 0, len(processorResponse.AnnouncedPeers))
		for _, peer := range processorResponse.AnnouncedPeers {
			announcedPeers = append(announcedPeers, peer.String())
		}

		utils.SafeSend(m.logWorker, statResponse{
			Txid:                  processorResponse.Hash.String(),
			Start:                 processorResponse.Start,
			Retries:               processorResponse.Retries.Load(),
			Err:                   processorResponse.Err,
			AnnouncedPeers:        announcedPeers,
			Status:                processorResponse.Status,
			NoStats:               processorResponse.NoStats,
			LastStatusUpdateNanos: processorResponse.LastStatusUpdateNanos.Load(),
			Log:                   processorResponse.Log,
		})
	}

	m.items.Delete(*hash)
	m.itemsLength.Add(-1)
	// Check if the item was deleted
	// if _, ok := m.items[*hash]; ok {
	// 	log.Printf("Failed to delete item from map: %v", hash)
	// }
}

func (m *ProcessorResponseMap) Len() int {
	return int(m.itemsLength.Load())
}

func (m *ProcessorResponseMap) Retries(hash *chainhash.Hash) uint32 {
	item, ok := m.items.Load(*hash)
	if !ok {
		return 0
	}

	processorResponse, ok := item.(*processor_response.ProcessorResponse)
	if !ok {
		return 0
	}

	return processorResponse.GetRetries()
}

func (m *ProcessorResponseMap) IncrementRetry(hash *chainhash.Hash) uint32 {
	item, ok := m.items.Load(*hash)
	if !ok {
		return 0
	}

	processorResponse, ok := item.(*processor_response.ProcessorResponse)
	if !ok {
		return 0
	}

	return processorResponse.IncrementRetry()
}

// Hashes will return a slice of the hashes in the map.
// If a filter function is provided, only hashes that pass the filter will be returned.
// If no filter function is provided, all hashes will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Hashes(filterFunc ...func(*processor_response.ProcessorResponse) bool) []chainhash.Hash {
	// Default filter function returns true for all items
	fn := func(p *processor_response.ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	hashes := make([]chainhash.Hash, 0, m.itemsLength.Load())

	m.items.Range(func(key, value interface{}) bool {
		if fn(value.(*processor_response.ProcessorResponse)) {
			hashes = append(hashes, key.(chainhash.Hash))
		}

		return true
	})

	return hashes
}

// Items will return a copy of the map.
// If a filter function is provided, only items that pass the filter will be returned.
// If no filter function is provided, all items will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Items(filterFunc ...func(*processor_response.ProcessorResponse) bool) map[chainhash.Hash]*processor_response.ProcessorResponse {
	// Default filter function returns true for all items
	fn := func(p *processor_response.ProcessorResponse) bool {
		return true
	}

	// If a filter function is provided, use it
	if len(filterFunc) > 0 && filterFunc[0] != nil {
		fn = filterFunc[0]
	}

	items := make(map[chainhash.Hash]*processor_response.ProcessorResponse, m.itemsLength.Load())

	m.items.Range(func(key, item interface{}) bool {
		processorResponse, ok := item.(*processor_response.ProcessorResponse)
		if !ok {
			return false
		}

		if fn(processorResponse) {
			items[key.(chainhash.Hash)] = processorResponse
		}

		return true
	})

	return items
}

func (m *ProcessorResponseMap) PrintItems() {
	m.items.Range(func(key, item interface{}) bool {
		fmt.Printf("processorResponseMap: %s\n", item)
		return true
	})
}

// Clear clears the map.
func (m *ProcessorResponseMap) Clear() {
	m.items = sync.Map{}
}

func (m *ProcessorResponseMap) clean() {
	m.items.Range(func(key, item interface{}) bool {
		processorResponse, ok := item.(*processor_response.ProcessorResponse)
		if !ok {
			return false
		}
		if time.Since(processorResponse.Start) > m.expiry {
			log.Printf("ProcessorResponseMap: Expired %s", key)
			m.items.Delete(key)
			m.itemsLength.Add(-1)
		}

		return true
	})
}
