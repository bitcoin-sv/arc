package p2p

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLittleEndian(t *testing.T) {
	le := binary.LittleEndian.Uint32([]byte{0x50, 0xcc, 0x0b, 0x00})

	require.Equal(t, uint32(773200), le)
}
