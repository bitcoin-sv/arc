package keyset

import (
	"testing"

	chaincfg "github.com/bsv-blockchain/go-sdk/transaction/chaincfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	keySet, err := New(&chaincfg.MainNet)
	require.NoError(t, err)

	assert.NotNil(t, keySet)
	assert.Empty(t, keySet.Path)

	t.Logf("master: %s", keySet.master.String())
}

func TestReadExtendedPrivateKey(t *testing.T) {
	key, err := NewFromExtendedKeyStr("xprv9s21ZrQH143K3uWZ5zfEG9v1JimHetdddkbnFAVKx2ELSws3T51wHoQuhfxsXTF4XGREBt7fVVbJiVpXJzrzb3dUVGsMsve5HaMGma4r6SG", "0/0")
	require.NoError(t, err)

	assert.Equal(t, "76a914fb4efeac628d6feda608898f543fee6520e8d33888ac", key.Script.String())
	assert.Equal(t, "1PuoEoj48mjgWdLetdvP9y4fx1uKefQoP1", key.Address(true))
	assert.Equal(t, "n4RkXrp2woAwHjpGcCtkytGzp1W2b1haPP", key.Address(false))

	key2, err := key.DeriveChildFromPath("0/1")
	require.NoError(t, err)

	assert.Equal(t, "76a9148a2558fc3f4e2c50f41c7380d9a1b1cfc0beb5f288ac", key2.Script.String())
	assert.Equal(t, "1DbSzRu4PTQFXNLTqD2hE8Fn5cGtQWxJsb", key2.Address(true))
	assert.Equal(t, "mt7QHUz3CUqWJUp5Yn1543U6wbsbNEi6YU", key2.Address(false))

	/* Don't get data from the network in tests
	unspent, err := key.GetUTXOs(true)
	require.NoError(t, err)

	for _, utxo := range unspent {
		t.Logf("%s:%d (%d sats)", utxo.TxIDStr(), utxo.Vout, utxo.Satoshis)
	}
	*/
}
