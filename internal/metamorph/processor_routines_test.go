package metamorph_test

import (
	"context"
	"testing"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	storeMocks "github.com/bitcoin-sv/arc/internal/metamorph/store/mocks"
)

func TestStartRoutine(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "start routine",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := &storeMocks.MetamorphStoreMock{
				SetUnlockedByNameFunc: func(_ context.Context, _ string) (int64, error) {
					return 0, nil
				},
			}
			messenger := &mocks.MediatorMock{
				AskForTxAsyncFunc:   func(_ context.Context, _ *chainhash.Hash) {},
				AnnounceTxAsyncFunc: func(_ context.Context, _ *chainhash.Hash, _ []byte) {},
			}
			sut, err := metamorph.NewProcessor(
				s,
				nil,
				messenger,
				nil,
			)
			require.NoError(t, err)

			testFunc := func(_ context.Context, _ *metamorph.Processor) []attribute.KeyValue {
				time.Sleep(200 * time.Millisecond)

				return []attribute.KeyValue{attribute.Int("atr", 5)}
			}

			sut.StartRoutine(50*time.Millisecond, testFunc, "testFunc")

			time.Sleep(100 * time.Millisecond)

			sut.Shutdown()
		})
	}
}
