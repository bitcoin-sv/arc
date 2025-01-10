package p2p

import (
	"io"

	"github.com/libsv/go-p2p/wire"
)

type Message interface {
	Bsvdecode(io.Reader, uint32, wire.MessageEncoding) error
	BsvEncode(io.Writer, uint32, wire.MessageEncoding) error
	Command() string
	MaxPayloadLength(uint32) uint64
}
