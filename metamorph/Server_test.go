package metamorph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
	"github.com/TAAL-GmbH/arc/metamorph/store"
	"github.com/TAAL-GmbH/arc/metamorph/store/memorystore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewServer(t *testing.T) {
	t.Run("NewServer", func(t *testing.T) {
		server := NewServer(nil, nil, nil)
		assert.IsType(t, &Server{}, server)
	})
}

func TestHealth(t *testing.T) {
	t.Run("Health", func(t *testing.T) {
		processor := NewProcessorMock()
		processor.Stats = &ProcessorStats{
			StartTime:       time.Now(),
			UptimeMillis:    2000,
			WorkerCount:     123,
			QueueLength:     136,
			QueuedCount:     356,
			ProcessedCount:  555,
			ProcessedMillis: 45645,
			ChannelMapSize:  22,
		}
		server := NewServer(nil, nil, processor)
		stats, err := server.Health(context.Background(), &emptypb.Empty{})
		assert.NoError(t, err)
		assert.Equal(t, processor.Stats.ChannelMapSize, stats.MapSize)
		assert.Equal(t, processor.Stats.QueuedCount, stats.Queued)
		assert.Equal(t, processor.Stats.ProcessedCount, stats.Processed)
		assert.Equal(t, processor.Stats.QueueLength, stats.Waiting)
		assert.Equal(t, float32(82.24324), stats.Average)
	})
}

func TestPutTransaction(t *testing.T) {
	t.Run("PutTransaction - ANNOUNCED", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		processor := NewProcessorMock()
		server := NewServer(nil, s, processor)
		server.SetTimeout(100 * time.Millisecond)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: tx1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)

			processor.GetProcessRequest(0).ResponseChannel <- &ProcessorResponse{
				Hash:   tx1Bytes,
				status: metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			}
		}()
		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, txStatus.Status)
		assert.True(t, txStatus.TimedOut)
	})

	t.Run("PutTransaction - SENT to network", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		processor := NewProcessorMock()
		server := NewServer(nil, s, processor)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: tx1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			processor.GetProcessRequest(0).ResponseChannel <- &ProcessorResponse{
				Hash:   tx1Bytes,
				status: metamorph_api.Status_SENT_TO_NETWORK,
			}
		}()
		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, txStatus.Status)
		assert.False(t, txStatus.TimedOut)
	})

	t.Run("PutTransaction - Err", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)

		processor := NewProcessorMock()
		server := NewServer(nil, s, processor)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: tx1RawBytes,
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			processor.GetProcessRequest(0).ResponseChannel <- &ProcessorResponse{
				Hash:   tx1Bytes,
				status: metamorph_api.Status_REJECTED,
				err:    fmt.Errorf("some error"),
			}
		}()
		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_REJECTED, txStatus.Status)
		assert.Equal(t, "some error", txStatus.RejectReason)
		assert.False(t, txStatus.TimedOut)
	})

	t.Run("PutTransaction - Known tx", func(t *testing.T) {
		s, err := memorystore.New()
		require.NoError(t, err)
		err = s.Set(context.Background(), tx1Bytes, &store.StoreData{
			Hash:   tx1Bytes,
			Status: metamorph_api.Status_SEEN_ON_NETWORK,
			RawTx:  tx1RawBytes,
		})
		require.NoError(t, err)

		processor := NewProcessorMock()
		server := NewServer(nil, s, processor)

		var txStatus *metamorph_api.TransactionStatus
		txRequest := &metamorph_api.TransactionRequest{
			RawTx: tx1RawBytes,
		}

		txStatus, err = server.PutTransaction(context.Background(), txRequest)
		assert.NoError(t, err)
		assert.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, txStatus.Status)
		assert.False(t, txStatus.TimedOut)
	})
}

func TestServer_GetTransactionStatus(t *testing.T) {
	s, err := memorystore.New()
	require.NoError(t, err)
	setStoreTestData(t, s)

	tests := []struct {
		name    string
		req     *metamorph_api.TransactionStatusRequest
		want    *metamorph_api.TransactionStatus
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "GetTransactionStatus - not found",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: "a147cc3c71cc13b29f18273cf50ffeb59fc9758152e2b33e21a8092f0b049118",
			},
			want: nil,
			wantErr: func(t assert.TestingT, err error, rest ...interface{}) bool {
				return assert.ErrorIs(t, err, store.ErrNotFound, rest)
			},
		},
		{
			name: "GetTransactionStatus - tx1",
			req: &metamorph_api.TransactionStatusRequest{
				Txid: tx1,
			},
			want: &metamorph_api.TransactionStatus{
				StoredAt:    timestamppb.New(testTime),
				AnnouncedAt: timestamppb.New(testTime.Add(1 * time.Second)),
				MinedAt:     timestamppb.New(testTime.Add(2 * time.Second)),
				Txid:        tx1,
				Status:      metamorph_api.Status_SENT_TO_NETWORK,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(nil, s, nil)
			got, err := server.GetTransactionStatus(context.Background(), tt.req)
			if !tt.wantErr(t, err, fmt.Sprintf("GetTransactionStatus(%v)", tt.req)) {
				return
			}
			assert.Equalf(t, tt.want, got, "GetTransactionStatus(%v)", tt.req)
		})
	}
}
