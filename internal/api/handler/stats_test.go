package handler

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewStats(t *testing.T) {
	t.Run("register, add, unregister stats", func(t *testing.T) {
		sut, err := NewStats()
		require.NoError(t, err)

		sut.Add(5)

		require.Equal(t, 5.0, testutil.ToFloat64(sut.apiTxSubmissions))

		sut.UnregisterStats()
	})
}
