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
	DeepFeeValidation
)

type ScriptValidation byte

const (
	NoneScriptValidation ScriptValidation = iota
	StandardScriptValidation
)

type Validator interface {
	// ValidateEFTransaction Please note that bt.Tx should have all the fields of each input populated.
	ValidateEFTransaction(ctx context.Context, tx *bt.Tx, feeValidation FeeValidation, scriptValidation ScriptValidation) error
	ValidateBeef(ctx context.Context, beef *beef.BEEF, feeValidation FeeValidation, scriptValidation ScriptValidation) (*bt.Tx, error)
	IsExtended(tx *bt.Tx) bool
	IsBeef(txHex []byte) bool
}

type DefaultV interface {
	ValidateTransaction(ctx context.Context, tx *bt.Tx, feeValidation FeeValidation, scriptValidation ScriptValidation) error
}

type BeefV interface {
	ValidateTransaction(ctx context.Context, beef *beef.BEEF, feeValidation FeeValidation, scriptValidation ScriptValidation) (*bt.Tx, error)
}

type HexFormat byte

const (
	Raw HexFormat = iota
	Ef
	Beef
)

func GetHexFormat(hex []byte) HexFormat {

	// check markers (first few bytes)
	/*
	 * EF 4 bytes for version + 6 bytes for marker 	(0000000000EF)
	 * Raw 4 bytes for version
	 * Beef 4 bytes version with marker (2 + 2)	(BEEF)
	 */

	if beef.CheckBeefFormat(hex[:4]) {
		return Beef
	}

	if hex[4] == 0 &&
		hex[5] == 0 &&
		hex[6] == 0 &&
		hex[7] == 0 &&
		hex[8] == 0 &&
		hex[9] == 0xEF {
		return Ef
	}

	return Raw
}
