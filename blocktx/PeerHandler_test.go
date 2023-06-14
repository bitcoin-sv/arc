package blocktx

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/libsv/go-bt/v2"
	"github.com/libsv/go-p2p"
	"github.com/libsv/go-p2p/bsvutil"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/libsv/go-p2p/wire"
	"github.com/ordishs/go-utils/expiringmap"
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
	btTx, err := bt.NewTxFromBytes(buff.Bytes())
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
	btTx, err := bt.NewTxFromBytes(buff.Bytes())
	require.NoError(t, err)

	height := extractHeightFromCoinbaseTx(btTx)

	assert.Equalf(t, uint64(2012), height, "height should be 2012, got %d", height)
}

func TestGetAnnouncedCacheBlockHashes(t *testing.T) {
	peerHandler := PeerHandler{
		announcedCache: expiringmap.New[chainhash.Hash, []p2p.PeerI](5 * time.Minute),
	}

	peer, err := p2p.NewPeerMock("", &peerHandler, wire.MainNet)
	assert.NoError(t, err)

	hash, err := chainhash.NewHashFromStr("00000000000000000e3c9aafb4c823562dd38f15b75849be348131a785154e33")
	assert.NoError(t, err)
	peerHandler.announcedCache.Set(*hash, []p2p.PeerI{peer})

	hash, err = chainhash.NewHashFromStr("00000000000000000cd097bf90c0f8480b930c88f3994503abccf45d579c601c")
	assert.NoError(t, err)
	peerHandler.announcedCache.Set(*hash, []p2p.PeerI{peer})

	hashes := peerHandler.getAnnouncedCacheBlockHashes()

	assert.ElementsMatch(t, hashes, []string{"00000000000000000e3c9aafb4c823562dd38f15b75849be348131a785154e33", "00000000000000000cd097bf90c0f8480b930c88f3994503abccf45d579c601c"})
}
