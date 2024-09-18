package metamorph

import (
	"fmt"
	"testing"

	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/store"
	"github.com/stretchr/testify/assert"
)

func BenchmarkUnorderedEqual(b *testing.B) {
	bcs := []int{1, 10, 100, 1000, 10000}

	for _, n := range bcs {
		b.Run(fmt.Sprintf("txIDs-%d", n), func(b *testing.B) {
			txIDs := make([]string, n)
			txIDs2 := make([]string, n)

			for i := 0; i < n; i++ {
				txIDs[i] = fmt.Sprintf("tx-%d", i)
				txIDs2[i] = fmt.Sprintf("tx-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = unorderedEqual(txIDs, txIDs2)
			}
		})
	}
}

func TestShouldUpdateStatus(t *testing.T) {
	testCases := []struct {
		name           string
		existingStatus store.UpdateStatus
		newStatus      store.UpdateStatus
		expectedResult bool
	}{
		{
			name: "new status lower than existing",
			existingStatus: store.UpdateStatus{
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			newStatus: store.UpdateStatus{
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
			expectedResult: false,
		},
		{
			name: "new status higher than existing",
			existingStatus: store.UpdateStatus{
				Status: metamorph_api.Status_ACCEPTED_BY_NETWORK,
			},
			newStatus: store.UpdateStatus{
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			expectedResult: true,
		},
		{
			name: "new status lower than existing, unequal competing txs",
			existingStatus: store.UpdateStatus{
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			newStatus: store.UpdateStatus{
				Status:       metamorph_api.Status_ACCEPTED_BY_NETWORK,
				CompetingTxs: []string{"1234"},
			},
			expectedResult: false,
		},
		{
			name: "statuses equal",
			existingStatus: store.UpdateStatus{
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			newStatus: store.UpdateStatus{
				Status: metamorph_api.Status_SEEN_ON_NETWORK,
			},
			expectedResult: false,
		},
		{
			name: "statuses equal, but unequal competing txs",
			existingStatus: store.UpdateStatus{
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"5678"},
			},
			newStatus: store.UpdateStatus{
				Status:       metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
				CompetingTxs: []string{"1234"},
			},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			actualResult := shouldUpdateStatus(tc.newStatus, tc.existingStatus)

			// then
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
