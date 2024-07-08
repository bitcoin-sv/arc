package validator

import (
	"context"

	"github.com/bitcoin-sv/arc/internal/beef"
	"github.com/libsv/go-bt/v2"
)

type Outpoint struct {
	Txid string
	Idx  uint32
}

type OutpointData struct {
	ScriptPubKey []byte
	Satoshis     int64
}

type FeeValidation byte

const (
	NoneFeeValidation FeeValidation = iota
	StandardFeeValidation
)

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)

type DefaultValidator interface {
	ValidateTransaction(ctx context.Context, tx *bt.Tx, feeValidation FeeValidation, scriptValidation ScriptValidation) error
	IsExtended(tx *bt.Tx) bool
}

type BeefValidator interface {
	ValidateTransaction(ctx context.Context, beef *beef.BEEF, feeValidation FeeValidation, scriptValidation ScriptValidation) (*bt.Tx, error)
}

type HexFormat byte

const (
	Raw HexFormat = iota
	Ef
	Beef
)

func GetHexFormat(hex []byte) HexFormat {
	if beef.CheckBeefFormat(hex) {
		return Beef
	}

	if isEf(hex) {
		return Ef
	}

	return Raw
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
