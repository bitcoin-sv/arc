package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadExtendedPrivateKey(t *testing.T) {
	key, err := GetPrivateKey("0/0")
	require.NoError(t, err)

	assert.Equal(t, "76a914117af07edf84bcd40950f46a8254f7f78d85243088ac", key.ScriptPubKey)
	assert.Equal(t, "12bRmF638dgkocbTpvBCn3bFKr2LjT4EwC", key.Address(true))
	assert.Equal(t, "mh7P4JB1wf81aj55YV9abxoaBqd3gJkt4Z", key.Address(false))

	unspent, err := key.GetUTXOs(true)
	require.NoError(t, err)

	for _, utxo := range unspent {
		t.Logf("%s", utxo)
	}
}
