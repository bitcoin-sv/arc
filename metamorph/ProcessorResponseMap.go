package metamorph

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
)

type getResponse struct {
	key string
	ch  chan *ProcessorResponse
}

type setResponse struct {
	key   string
	value *ProcessorResponse
}

type getHashes struct {
	ch chan [][]byte
	fn func(*ProcessorResponse) bool
}

type getItems struct {
	ch chan map[string]*ProcessorResponse
	fn func(*ProcessorResponse) bool
}

type ProcessorResponseMap struct {
	setCh    chan setResponse
	getCh    chan getResponse
	delCh    chan string
	hashesCh chan getHashes
	itemsCh  chan getItems
	clearCh  chan struct{}
	expiry   time.Duration
	items    map[string]*ProcessorResponse
	length   atomic.Int64
}

func NewProcessorResponseMap(expiry time.Duration) *ProcessorResponseMap {
	m := &ProcessorResponseMap{
		expiry:   expiry,
		items:    make(map[string]*ProcessorResponse),
		setCh:    make(chan setResponse),
		getCh:    make(chan getResponse),
		delCh:    make(chan string),
		hashesCh: make(chan getHashes),
		itemsCh:  make(chan getItems),
		clearCh:  make(chan struct{}),
	}

	go func() {
		for range time.NewTicker(1 * time.Second).C {
			m.clean()
		}
	}()

	go func() {
		for {
			select {

			case setValue := <-m.setCh:
				key := setValue.key
				m.items[key] = setValue.value
				m.length.Store(int64(len(m.items)))

			case getValue := <-m.getCh:
				item, ok := m.items[getValue.key]
				if !ok || time.Since(item.Start) > m.expiry {
					getValue.ch <- nil
				} else {
					getValue.ch <- item
				}

			case deleteKey := <-m.delCh:
				delete(m.items, deleteKey)
				m.length.Store(int64(len(m.items)))

			case getH := <-m.hashesCh:
				hashes := make([][]byte, 0, len(m.items))
				for _, item := range m.items {
					if getH.fn == nil || getH.fn(item) {
						hashes = append(hashes, item.Hash)
					}
				}
				getH.ch <- hashes

			case getI := <-m.itemsCh:
				items := make(map[string]*ProcessorResponse)
				for key, item := range m.items {
					if getI.fn == nil || getI.fn(item) {
						items[key] = item
					}
				}
				getI.ch <- items

			case <-m.clearCh:
				m.items = make(map[string]*ProcessorResponse)
				m.length.Store(0)
			}
		}
	}()

	return m
}

func (m *ProcessorResponseMap) Set(key string, value *ProcessorResponse) {
	m.setCh <- setResponse{key: key, value: value}
}

func (m *ProcessorResponseMap) Get(key string) (*ProcessorResponse, bool) {
	ch := make(chan *ProcessorResponse)
	m.getCh <- getResponse{key: key, ch: ch}

	processorResponse := <-ch

	if processorResponse == nil {
		return nil, false
	}

	return processorResponse, true
}

func (m *ProcessorResponseMap) Delete(key string) {
	m.delCh <- key
}

func (m *ProcessorResponseMap) Len() int {
	return int(m.length.Load())
}

// Hashes will return a slice of the hashes in the map.
// If a filter function is provided, only hashes that pass the filter will be returned.
// If no filter function is provided, all hashes will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Hashes(filterFunc ...func(*ProcessorResponse) bool) [][]byte {
	// Default filter function is nil
	var fn func(p *ProcessorResponse) bool

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	ch := make(chan [][]byte)
	m.hashesCh <- getHashes{fn: fn, ch: ch}
	hashes := <-ch

	return hashes
}

// Items will return a copy of the map.
// If a filter function is provided, only items that pass the filter will be returned.
// If no filter function is provided, all items will be returned.
// The filter function will be called with the lock held, so it should not block.
func (m *ProcessorResponseMap) Items(filterFunc ...func(*ProcessorResponse) bool) map[string]*ProcessorResponse {
	// Default filter function is nil
	var fn func(p *ProcessorResponse) bool

	// If a filter function is provided, use it
	if len(filterFunc) > 0 {
		fn = filterFunc[0]
	}

	ch := make(chan map[string]*ProcessorResponse)
	m.itemsCh <- getItems{fn: fn, ch: ch}
	items := <-ch

	return items
}

func (m *ProcessorResponseMap) PrintItems() {
	hashes := m.Hashes(func(p *ProcessorResponse) bool {
		return time.Since(p.Start) > m.expiry
	})

	for _, hash := range hashes {
		fmt.Printf("tx2ChMap: %s\n", utils.HexEncodeAndReverseBytes(hash))
	}
}

// Clear clears the map.
func (m *ProcessorResponseMap) Clear() {
	m.clearCh <- struct{}{}
}

func (m *ProcessorResponseMap) clean() {
	hashes := m.Hashes(func(p *ProcessorResponse) bool {
		return time.Since(p.Start) > m.expiry
	})

	for _, hash := range hashes {
		txIDStr := hex.EncodeToString(bt.ReverseBytes(hash))
		log.Printf("ProcessorResponseMap: Expired %s", txIDStr)
		m.Delete(txIDStr)
	}
}
