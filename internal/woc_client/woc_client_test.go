package woc_client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const testnet_addr = "miGidJu3HrpGuFxhF7gCg1yV13AJzANz4P"

func Test_New(t *testing.T) {
	tcs := []struct {
		name                string
		useMainnet          bool
		expectedRequestPath string
	}{
		{
			name:                "client for mainnet",
			useMainnet:          true,
			expectedRequestPath: "/v1/bsv/main/",
		},
		{
			name:                "client for testnet",
			useMainnet:          false,
			expectedRequestPath: "/v1/bsv/test/",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sut := New(tc.useMainnet)

			// when
			httpReq, err := sut.httpRequest(context.TODO(), "GET", "", nil)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.expectedRequestPath, httpReq.URL.Path)

		})
	}
}

func Test_WithAuth(t *testing.T) {
	tcs := []struct {
		name   string
		apiKey string
	}{
		{
			name:   "client with api key",
			apiKey: "test-api-key",
		},
		{
			name: "client without api key",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// given
			sut := New(false, WithAuth(tc.apiKey))

			// when
			httpReq, err := sut.httpRequest(context.TODO(), "GET", "", nil)

			// then
			require.NoError(t, err)

			authHeader, found := httpReq.Header["Authorization"]
			if tc.apiKey != "" {
				require.True(t, found)
				require.Equal(t, tc.apiKey, authHeader[0])
			} else {
				require.False(t, found)
			}
		})
	}
}

func Test_GetUTXOs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// given
	sut := New(false)

	// when
	res, err := sut.GetUTXOs(context.TODO(), nil, testnet_addr)

	// then
	require.NoError(t, err)
	require.NotNil(t, res)
}

func Test_GetBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// given
	sut := New(false)

	// when
	_, _, err := sut.GetBalance(context.TODO(), testnet_addr)

	// then
	require.NoError(t, err)
}

func Test_TopUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("use on testnet", func(t *testing.T) {
		// given
		sut := New(false)

		// when
		err := sut.TopUp(context.TODO(), testnet_addr)

		// then
		require.NoError(t, err)
	})

	t.Run("try use on mainnet - should fail", func(t *testing.T) {
		// given
		const useOnMainnet = true
		sut := New(useOnMainnet)

		// when
		err := sut.TopUp(context.TODO(), testnet_addr)

		// then
		require.Error(t, err)
		require.ErrorIs(t, err, ErrWOCFailedToTopUp)
	})
}

func Test_GetRawTxs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// given
	sut := New(false)

	const idsCount = 37
	ids := make([]string, idsCount)
	for i := 0; i < idsCount; i++ {
		ids[i] = fmt.Sprintf("id%d", i)
	}

	// when
	res, err := sut.GetRawTxs(context.TODO(), ids)

	// then
	require.NoError(t, err)
	require.NotNil(t, res)

}
