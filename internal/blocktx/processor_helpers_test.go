package blocktx

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/bitcoinsv/bsvutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHeight(t *testing.T) {
	coinbase, _ := hex.DecodeString("01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff570350cc0b041547b5630cfabe6d6d0000000000000000000000000000000000000000000000000000000000000000010000000000000047ed20542096bd0000000000143362663865373833636662643732306431383436000000000140be4025000000001976a914c9b0abe09b7dd8e9d1e8c1e3502d32ab0d7119e488ac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := sdkTx.NewTransactionFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := extractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(773200), height, "height should be 773200, got %d", height)
}

func TestExtractHeightForRegtest(t *testing.T) {
	coinbase, _ := hex.DecodeString("02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0502dc070101ffffffff012f500900000000002321032efe256e14fd77eea05d0453374f8920e0a7a4a573bb3937ef3f567f3937129cac00000000")
	tx, err := bsvutil.NewTxFromBytes(coinbase)
	require.NoError(t, err)

	buff := bytes.NewBuffer(nil)
	err = tx.MsgTx().Serialize(buff)
	require.NoError(t, err)
	btTx, err := sdkTx.NewTransactionFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := extractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(2012), height, "height should be 2012, got %d", height)
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
		t.Run(name, func(t *testing.T) {
			cw := CalculateChainwork(params.bits)

			require.Equal(t, cw.String(), params.expectedChainWork)
		})
	}
}
