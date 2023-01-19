package keyset

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	keySet, err := New()
	require.NoError(t, err)

	assert.NotNil(t, keySet)
	assert.Empty(t, keySet.Path)

	t.Logf("master: %#v", keySet)
}

func TestReadExtendedPrivateKey(t *testing.T) {
	extendedBytes, err := os.ReadFile("arc.key")
	if err != nil {
		if os.IsNotExist(err) {
			panic("arc.key not found. Please create this file with the xpriv you want to use")
		}
		panic(err.Error())
	}

	key, err := NewFromExtendedKeyStr(string(extendedBytes), "0/0")
	require.NoError(t, err)

	assert.Equal(t, "76a914117af07edf84bcd40950f46a8254f7f78d85243088ac", key.Script.String())
	assert.Equal(t, "12bRmF638dgkocbTpvBCn3bFKr2LjT4EwC", key.Address(true))
	assert.Equal(t, "mh7P4JB1wf81aj55YV9abxoaBqd3gJkt4Z", key.Address(false))

	key2, err := key.DeriveChildFromPath("0/1")
	require.NoError(t, err)

	assert.Equal(t, "76a914c9c8e92c73da1eb2374d834a0082e93e86dd6e8288ac", key2.Script.String())
	assert.Equal(t, "1KPwW656jGvQrJ25vg3kXrhU9UDS1RWHuZ", key2.Address(true))
	assert.Equal(t, "myuto9A5YJMfdQVheF28Mmuo1Tp8tCSEMa", key2.Address(false))

	unspent, err := key.GetUTXOs(true)
	require.NoError(t, err)

	for _, utxo := range unspent {
		t.Logf("%s:%d (%d sats)", utxo.TxIDStr(), utxo.Vout, utxo.Satoshis)
	}
}
