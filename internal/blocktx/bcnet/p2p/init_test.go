package p2p

import (
	"testing"

	sdkTx "github.com/bitcoin-sv/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractHeight(t *testing.T) {
	// given
	tx, err := sdkTx.NewTransactionFromHex("01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff570350cc0b041547b5630cfabe6d6d0000000000000000000000000000000000000000000000000000000000000000010000000000000047ed20542096bd0000000000143362663865373833636662643732306431383436000000000140be4025000000001976a914c9b0abe09b7dd8e9d1e8c1e3502d32ab0d7119e488ac00000000")
	require.NoError(t, err)

	// when
	height := extractHeightFromCoinbaseTx(tx)

	// then
	assert.Equalf(t, uint64(773200), height, "height should be 773200, got %d", height)
}

func TestExtractHeightForRegtest(t *testing.T) {
	// given
	tx, err := sdkTx.NewTransactionFromHex("02000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0502dc070101ffffffff012f500900000000002321032efe256e14fd77eea05d0453374f8920e0a7a4a573bb3937ef3f567f3937129cac00000000")
	require.NoError(t, err)

	// when
	height := extractHeightFromCoinbaseTx(tx)

	// then
	assert.Equalf(t, uint64(2012), height, "height should be 2012, got %d", height)
}
