package metamorph

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/test"
	"github.com/libsv/go-bt/v2"
	"github.com/stretchr/testify/require"
)

type SendStatusForTransactionCall struct {
	HashStr string
	Status  metamorph_api.Status
	Err     error
}

type ProcessorMock struct {
	Stats                         *ProcessorStats
	mu                            sync.Mutex
	processTransactionCalls       []*ProcessorRequest
	SendStatusForTransactionCalls []*SendStatusForTransactionCall
}

func NewProcessorMock() *ProcessorMock {
	return &ProcessorMock{
		Stats:                         &ProcessorStats{},
		processTransactionCalls:       make([]*ProcessorRequest, 0),
		SendStatusForTransactionCalls: make([]*SendStatusForTransactionCall, 0),
	}
}

func (p *ProcessorMock) LoadUnseen() {}

func (p *ProcessorMock) ProcessTransaction(req *ProcessorRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.processTransactionCalls = append(p.processTransactionCalls, req)
}

func (p *ProcessorMock) GetProcessRequest(index int) *ProcessorRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.processTransactionCalls[index]
}

func (p *ProcessorMock) SendStatusForTransaction(hashStr string, status metamorph_api.Status, err error) (bool, error) {
	p.SendStatusForTransactionCalls = append(p.SendStatusForTransactionCalls, &SendStatusForTransactionCall{
		HashStr: hashStr,
		Status:  status,
		Err:     err,
	})
	return true, nil
}

func (p *ProcessorMock) SendStatusMinedForTransaction(hash []byte, blockHash []byte, blockHeight int32) (bool, error) {
	p.SendStatusForTransactionCalls = append(p.SendStatusForTransactionCalls, &SendStatusForTransactionCall{
		HashStr: hex.EncodeToString(bt.ReverseBytes(hash)),
		Status:  metamorph_api.Status_MINED,
		Err:     nil,
	})
	return true, nil
}

func (p *ProcessorMock) GetStats() *ProcessorStats {
	return p.Stats
}

func setStoreTestData(t *testing.T, s store.Store) {
	ctx := context.Background()
	err := s.Set(ctx, test.TX1Bytes, &store.StoreData{
		StoredAt:      test.Time,
		AnnouncedAt:   test.Time.Add(1 * time.Second),
		MinedAt:       test.Time.Add(2 * time.Second),
		Hash:          test.TX1Bytes,
		Status:        metamorph_api.Status_SENT_TO_NETWORK,
		CallbackUrl:   "https://test.com",
		CallbackToken: "token",
	})
	require.NoError(t, err)
	err = s.Set(ctx, test.TX2Bytes, &store.StoreData{
		StoredAt:    test.Time,
		AnnouncedAt: test.Time.Add(1 * time.Second),
		MinedAt:     test.Time.Add(2 * time.Second),
		Hash:        test.TX2Bytes,
		Status:      metamorph_api.Status_SENT_TO_NETWORK,
	})
	require.NoError(t, err)
	err = s.Set(ctx, test.TX3Bytes, &store.StoreData{
		StoredAt:    test.Time,
		AnnouncedAt: test.Time.Add(1 * time.Second),
		MinedAt:     test.Time.Add(2 * time.Second),
		Hash:        test.TX3Bytes,
		Status:      metamorph_api.Status_SEEN_ON_NETWORK,
	})
	require.NoError(t, err)
	err = s.Set(ctx, test.TX4Bytes, &store.StoreData{
		StoredAt:    test.Time,
		AnnouncedAt: test.Time.Add(1 * time.Second),
		MinedAt:     test.Time.Add(2 * time.Second),
		Hash:        test.TX4Bytes,
		Status:      metamorph_api.Status_REJECTED,
	})
	require.NoError(t, err)
}
