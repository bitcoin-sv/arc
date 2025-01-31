package p2p_tests

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/bitcoin-sv/arc/internal/p2p"
	"github.com/libsv/go-p2p/wire"
	"github.com/stretchr/testify/require"
)

func TestWireReader_ReadNextMsg(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// given
		expectedMsg := wire.NewMsgGetBlocks(blockHash)

		var buff bytes.Buffer
		err := wire.WriteMessage(&buff, expectedMsg, wire.ProtocolVersion, bitcoinNet)
		require.NoError(t, err)

		sut := p2p.NewWireReader(&buff, 4096)

		// when
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := sut.ReadNextMsg(ctx, wire.ProtocolVersion, bitcoinNet)

		// then
		require.NoError(t, err)
		require.Equal(t, expectedMsg, res)
	})

	t.Run("Unknown msg", func(t *testing.T) {
		// given
		unknownMsg := unknownMsg{}

		expectedMsg := wire.NewMsgGetBlocks(blockHash)

		var buff bytes.Buffer
		// first write unknown msg
		err := wire.WriteMessage(&buff, &unknownMsg, wire.ProtocolVersion, bitcoinNet)
		require.NoError(t, err)

		// next write regular msg
		err = wire.WriteMessage(&buff, expectedMsg, wire.ProtocolVersion, bitcoinNet)
		require.NoError(t, err)

		sut := p2p.NewWireReader(&buff, 4096)

		// when
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()

		res, err := sut.ReadNextMsg(ctx, wire.ProtocolVersion, bitcoinNet)

		// then
		require.NoError(t, err)
		require.Equal(t, expectedMsg, res)
	})

	t.Run("Context cancelled", func(t *testing.T) {
		// given
		expectedMsg := wire.NewMsgGetBlocks(blockHash)

		var buff bytes.Buffer
		err := wire.WriteMessage(&buff, expectedMsg, wire.ProtocolVersion, bitcoinNet)
		require.NoError(t, err)

		sut := p2p.NewWireReader(&buff, 4096)

		// when
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel the context immediately

		res, err := sut.ReadNextMsg(ctx, wire.ProtocolVersion, bitcoinNet)

		// then
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, res)
	})

	t.Run("Read error", func(t *testing.T) {
		var buff bytes.Buffer
		sut := p2p.NewWireReader(&buff, 4096)

		// when
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := sut.ReadNextMsg(ctx, wire.ProtocolVersion, bitcoinNet)

		// then
		require.Error(t, err)
		require.Nil(t, res)
	})
}

type unknownMsg struct {
}

func (m *unknownMsg) Bsvdecode(_ io.Reader, _ uint32, _ wire.MessageEncoding) error {
	return nil
}

func (m *unknownMsg) BsvEncode(_ io.Writer, _ uint32, _ wire.MessageEncoding) error {
	return nil
}

func (m *unknownMsg) Command() string {
	return "test-cmd"
}

func (m *unknownMsg) MaxPayloadLength(_ uint32) uint64 {
	return 0
}
