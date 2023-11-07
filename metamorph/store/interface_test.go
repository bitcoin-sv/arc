package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTime(t *testing.T) {
	t1 := time.Now()

	var buf bytes.Buffer

	err := encodeTime(&buf, t1)
	require.NoError(t, err)

	t2, err := decodeTime(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	assert.Equal(t, t1.UnixNano(), t2.UnixNano())
}

func TestHash(t *testing.T) {
	h1, err := chainhash.NewHashFromStr("fb1fcc63bb0cc62a2a821d674c670799834c0cea352f30fe295b197fec90b623")
	require.NoError(t, err)

	var buf bytes.Buffer

	err = encodeHash(&buf, h1)
	require.NoError(t, err)

	h2, err := decodeHash(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	assert.Equal(t, h1, h2)
}

func TestEmptyHash(t *testing.T) {
	var h1 *chainhash.Hash
	require.Nil(t, h1)

	var buf bytes.Buffer

	err := encodeHash(&buf, h1)
	require.NoError(t, err)

	h2, err := decodeHash(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)

	assert.Equal(t, h1, h2)
}
