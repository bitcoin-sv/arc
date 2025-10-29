package blocktx

import (
	"fmt"
	"testing"

	testutils "github.com/bitcoin-sv/arc/pkg/test_utils"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/internal/blocktx/store"
)

func TestExlusiveRightTxs(t *testing.T) {
	// given
	leftTxs := []store.BlockTransaction{
		{
			TxHash: []byte("1"),
		},
		{
			TxHash: []byte("2"),
		},
	}
	rightTxs := []store.BlockTransaction{
		{
			TxHash: []byte("A"),
		},
		{
			TxHash: []byte("B"),
		},
		{
			TxHash: []byte("1"),
		},
	}

	expectedStaleTxs := []store.BlockTransaction{
		{
			TxHash: []byte("A"),
		},
		{
			TxHash: []byte("B"),
		},
	}

	// when
	actualStaleTxs := exclusiveRightTxs(leftTxs, rightTxs)

	// then
	require.Equal(t, expectedStaleTxs, actualStaleTxs)
}

func TestChainWork(t *testing.T) {
	testCases := []struct {
		height            int
		bits              uint32
		expectedChainWork string
	}{
		{
			height:            0,
			bits:              0x1d00ffff,
			expectedChainWork: "4295032833",
		},
		{
			height:            50_000,
			bits:              0x1c2a1115,
			expectedChainWork: "26137323115",
		},
		{
			height:            100_000,
			bits:              0x1b04864c,
			expectedChainWork: "62209952899966",
		},
		{
			height:            200_000,
			bits:              0x1a05db8b,
			expectedChainWork: "12301577519373468",
		},
		{
			height:            300_000,
			bits:              0x1900896c,
			expectedChainWork: "34364008516618225545",
		},
		{
			height:            400_000,
			bits:              0x1806b99f,
			expectedChainWork: "702202025755488147582",
		},
		{
			height:            500_000,
			bits:              0x1809b91a,
			expectedChainWork: "485687622324422197901",
		},
		{
			height:            600_000,
			bits:              0x18089116,
			expectedChainWork: "551244161910380757574",
		},
		{
			height:            700_000,
			bits:              0x181452d3,
			expectedChainWork: "232359535664858305416",
		},
		{
			height:            282_240,
			bits:              0x1901f52c,
			expectedChainWork: "9422648633005683357",
		},
		{
			height:            292_320,
			bits:              0x1900db99,
			expectedChainWork: "21504630620890996935",
		},
	}
	for _, params := range testCases {
		name := fmt.Sprintf("should evaluate bits %d from block %d as chainwork %s",
			params.bits, params.height, params.expectedChainWork)
		testutils.RunParallel(t, true, name, func(t *testing.T) {
			// when
			cw := calculateChainwork(params.bits)

			// then
			require.Equal(t, cw.String(), params.expectedChainWork)
		})
	}
}
