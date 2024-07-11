package validator

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/beef"
)

type HexFormat byte

const (
	RawHex HexFormat = iota
	EfHex
	BeefHex
)

type TxFinderI interface {
	GetRawTxs(ctx context.Context, ids []string) ([]RawTx, error)
}

type RawTx struct {
	TxID    string
	Bytes   []byte
	IsMined bool
}

type MerkleVerifierI interface {
	Verify(ctx context.Context, request []beef.MerkleRootVerificationRequest) ([]uint64, error)
}

func GetHexFormat(hex []byte) HexFormat {
	if beef.CheckBeefFormat(hex) {
		return BeefHex
	}

	if isEf(hex) {
		return EfHex
	}

	return RawHex
}

func isEf(hex []byte) bool {
	// check markers - first 10 bytes
	// 4 bytes for version + 6 bytes for the marker - 0000000000EF
	return len(hex) > 10 &&
		hex[4] == 0 &&
		hex[5] == 0 &&
		hex[6] == 0 &&
		hex[7] == 0 &&
		hex[8] == 0 &&
		hex[9] == 0xEF
}
