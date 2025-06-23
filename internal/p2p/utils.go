package p2p

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func slogUpperString(key, val string) slog.Attr {
	return slog.String(key, strings.ToUpper(val))
}

const slogLvlTrace slog.Level = slog.LevelDebug - 4

// VarInt (variable integer) is a field used in transaction data to indicate the number of
// upcoming fields, or the length of an upcoming field.
// See http://learnmeabitcoin.com/glossary/varint
type VarInt uint64

// Length return the length of the underlying byte representation of the `transaction.VarInt`.
func (v VarInt) Length() int {
	if v < 253 {
		return 1
	}
	if v < 65536 {
		return 3
	}
	if v < 4294967296 {
		return 5
	}
	return 9
}

// ReadFrom reads the next varint from the io.Reader and assigned it to itself.
func (v *VarInt) ReadFrom(r io.Reader) (int64, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, errors.Join(err, errors.New("could not read varint type"))
	}

	switch b[0] {
	case 0xff:
		bb := make([]byte, 8)
		if n, err := io.ReadFull(r, bb); err != nil {
			return 9, errors.Join(err, fmt.Errorf("varint(8): got %d bytes", n))
		}
		*v = VarInt(binary.LittleEndian.Uint64(bb))
		return 9, nil

	case 0xfe:
		bb := make([]byte, 4)
		if n, err := io.ReadFull(r, bb); err != nil {
			return 5, errors.Join(err, fmt.Errorf("varint(4): got %d bytes", n))
		}
		*v = VarInt(binary.LittleEndian.Uint32(bb))
		return 5, nil

	case 0xfd:
		bb := make([]byte, 2)
		if n, err := io.ReadFull(r, bb); err != nil {
			return 3, errors.Join(err, fmt.Errorf("varint(2): got %d bytes", n))
		}
		*v = VarInt(binary.LittleEndian.Uint16(bb))
		return 3, nil

	default:
		*v = VarInt(binary.LittleEndian.Uint16([]byte{b[0], 0x00}))
		return 1, nil
	}
}
