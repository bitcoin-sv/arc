package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/bitcoin-sv/arc/internal/metamorph"
)

func TestCheckSwagger(t *testing.T) {
	tt := []struct {
		name string
		path string
	}{
		{
			name: "valid request",
			path: "/v1/policy",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(""))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

			swagger := CheckSwagger(e)
			assert.NotNil(t, swagger)
		})
	}
}

func TestFilterStatusesByTxIDs(t *testing.T) {
	tcs := []struct {
		name     string
		txIDs    []string
		statuses []*metamorph.TransactionStatus
		expected []*metamorph.TransactionStatus
	}{
		{
			name:  "Single txID with matching status",
			txIDs: []string{"tx1"},
			statuses: []*metamorph.TransactionStatus{
				{TxID: "tx1"},
			},
			expected: []*metamorph.TransactionStatus{
				{TxID: "tx1"},
			},
		},
		{
			name:  "Single txID with non-matching status",
			txIDs: []string{"tx1"},
			statuses: []*metamorph.TransactionStatus{
				{TxID: "tx2"},
			},
			expected: []*metamorph.TransactionStatus{},
		},
		{
			name:  "Multiple txIDs with some matching statuses",
			txIDs: []string{"tx1", "tx3"},
			statuses: []*metamorph.TransactionStatus{
				{TxID: "tx1"},
				{TxID: "tx2"},
				{TxID: "tx3"},
			},
			expected: []*metamorph.TransactionStatus{
				{TxID: "tx1"},
				{TxID: "tx3"},
			},
		},
		{
			name:  "No txIDs",
			txIDs: []string{},
			statuses: []*metamorph.TransactionStatus{
				{TxID: "tx1"},
				{TxID: "tx2"},
			},
			expected: []*metamorph.TransactionStatus{},
		},
		{
			name:     "No statuses",
			txIDs:    []string{"tx1", "tx2"},
			statuses: []*metamorph.TransactionStatus{},
			expected: []*metamorph.TransactionStatus{},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res := filterStatusesByTxIDs(tc.txIDs, tc.statuses)
			if !reflect.DeepEqual(res, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, res)
			}
		})
	}
}

func BenchmarkFilterStatusesByTxIDs(b *testing.B) {
	bcs := []int{1, 10, 100, 1000, 10000}

	for _, n := range bcs {
		b.Run(fmt.Sprintf("txIDs-%d", n), func(b *testing.B) {
			txIDs := make([]string, n)
			for i := 0; i < n; i++ {
				txIDs[i] = fmt.Sprintf("tx-%d", i)
			}

			statuses := make([]*metamorph.TransactionStatus, n)
			for i := 0; i < n; i++ {
				statuses[i] = &metamorph.TransactionStatus{TxID: fmt.Sprintf("tx-%d", i)}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = filterStatusesByTxIDs(txIDs, statuses)
			}
		})
	}
}
