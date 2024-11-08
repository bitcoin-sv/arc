package validator

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/beef"
)

type HexFormat byte

const (
	RawHex HexFormat = iota
	EfHex
	BeefHex
)

type FindSourceFlag byte

const (
	SourceTransactionHandler FindSourceFlag = 1 << iota // 1 (binary 0001)
	SourceNodes                                         // 2 (binary 0010)
	SourceWoC                                           // 4 (binary 0100)
)

func (flag FindSourceFlag) Has(v FindSourceFlag) bool {
	return v&flag != 0
}

type TxFinderI interface {
	GetRawTxs(ctx context.Context, source FindSourceFlag, ids []string) ([]RawTx, error)
	GetMempoolAncestors(ctx context.Context, ids []string) ([]RawTx, error)
}

func NewRawTx(id, hexTx string, blockH uint64) (RawTx, error) {
	b, err := hex.DecodeString(hexTx)
	if err != nil {
		return RawTx{}, fmt.Errorf("invalid raw tx: %w", err)
	}

	rt := RawTx{
		TxID:    id,
		Bytes:   b,
		IsMined: blockH > 0,
	}

	return rt, nil
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
