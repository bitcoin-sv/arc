package metamorph

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
	"github.com/stretchr/testify/require"
)

var (
	tx1            = "b042f298deabcebbf15355aa3a13c7d7cfe96c44ac4f492735f936f8e50d06f6"
	tx1Bytes, _    = utils.DecodeAndReverseHexString(tx1)
	tx1Raw         = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1a0386c40b2f7461616c2e636f6d2f00cf47ad9c7af83836000000ffffffff0117564425000000001976a914522cf9e7626d9bd8729e5a1398ece40dad1b6a2f88ac00000000"
	tx1RawBytes, _ = hex.DecodeString(tx1Raw)
	tx2            = "1a8fda8c35b8fc30885e88d6eb0214e2b3a74c96c82c386cb463905446011fdf"
	tx2Bytes, _    = utils.DecodeAndReverseHexString(tx2)
	tx3            = "3f63399b3d9d94ba9c5b7398b9328dcccfcfd50f07ad8b214e766168c391642b"
	tx3Bytes, _    = utils.DecodeAndReverseHexString(tx3)
	tx4            = "88eab41a8d0b7b4bc395f8f988ea3d6e63c8bc339526fd2f00cb7ce6fd7df0f7"
	tx4Bytes, _    = utils.DecodeAndReverseHexString(tx4)
	testTime       = time.Date(2009, 1, 03, 18, 15, 05, 0, time.UTC)
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
	err := s.Set(ctx, tx1Bytes, &store.StoreData{
		StoredAt:      testTime,
		AnnouncedAt:   testTime.Add(1 * time.Second),
		MinedAt:       testTime.Add(2 * time.Second),
		Hash:          tx1Bytes,
		Status:        metamorph_api.Status_SENT_TO_NETWORK,
		CallbackUrl:   "https://test.com",
		CallbackToken: "token",
	})
	require.NoError(t, err)
	err = s.Set(ctx, tx2Bytes, &store.StoreData{
		StoredAt:    testTime,
		AnnouncedAt: testTime.Add(1 * time.Second),
		MinedAt:     testTime.Add(2 * time.Second),
		Hash:        tx2Bytes,
		Status:      metamorph_api.Status_SENT_TO_NETWORK,
	})
	require.NoError(t, err)
	err = s.Set(ctx, tx3Bytes, &store.StoreData{
		StoredAt:    testTime,
		AnnouncedAt: testTime.Add(1 * time.Second),
		MinedAt:     testTime.Add(2 * time.Second),
		Hash:        tx3Bytes,
		Status:      metamorph_api.Status_SEEN_ON_NETWORK,
	})
	require.NoError(t, err)
	err = s.Set(ctx, tx4Bytes, &store.StoreData{
		StoredAt:    testTime,
		AnnouncedAt: testTime.Add(1 * time.Second),
		MinedAt:     testTime.Add(2 * time.Second),
		Hash:        tx4Bytes,
		Status:      metamorph_api.Status_REJECTED,
	})
	require.NoError(t, err)
}
