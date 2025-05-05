package bcnet

import (
	"bytes"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	peerAddr   string          = "localhost:1234"
	bitcoinNet wire.BitcoinNet = wire.TestNet
)

var (
	blockHash, _ = chainhash.NewHashFromStr("00000000000007b1f872a8abe664223d65acd22a500b1b8eb5db3fe09a9837ff")
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

func TestMessageRead(t *testing.T) {
	t.Run("Message read", func(t *testing.T) {
		// given
		msgBlock := wire.NewMsgBlock(wire.NewBlockHeader(0, blockHash, blockHash, 0, 0))
		tx := []*wire.MsgTx{
			{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{},
							Index: 0xffffffff,
						},
						SignatureScript: []byte{
							0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04,
						},
						Sequence: 0xffffffff,
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value: 0x12a05f200,
						PkScript: []byte{
							0x41, // OP_DATA_65
							0x04, 0x96, 0xb5, 0x38, 0xe8, 0x53, 0x51, 0x9c,
							0x72, 0x6a, 0x2c, 0x91, 0xe6, 0x1e, 0xc1, 0x16,
							0x00, 0xae, 0x13, 0x90, 0x81, 0x3a, 0x62, 0x7c,
							0x66, 0xfb, 0x8b, 0xe7, 0x94, 0x7b, 0xe6, 0x3c,
							0x52, 0xda, 0x75, 0x89, 0x37, 0x95, 0x15, 0xd4,
							0xe0, 0xa6, 0x04, 0xf8, 0x14, 0x17, 0x81, 0xe6,
							0x22, 0x94, 0x72, 0x11, 0x66, 0xbf, 0x62, 0x1e,
							0x73, 0xa8, 0x2c, 0xbf, 0x23, 0x42, 0xc8, 0x58,
							0xee, // 65-byte signature
							0xac, // OP_CHECKSIG
						},
					},
				},
				LockTime: 0,
			},
		}
		err := msgBlock.AddTransaction(tx[0])
		require.NoError(t, err)

		require.NoError(t, err)
		var buff bytes.Buffer
		err = wire.WriteMessage(&buff, msgBlock, wire.ProtocolVersion, bitcoinNet)
		require.NoError(t, err)
		//When then
		_, _, _, err = wire.ReadMessageN(&buff, 0, wire.TestNet)
		require.NoError(t, err)
	})
}
