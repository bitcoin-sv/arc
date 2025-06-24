package beef

import (
	"errors"
	"fmt"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

const (
	beefVersionBytesCount = 4
)

const (
	beefMarkerPart1 = 0xBE
	beefMarkerPart2 = 0xEF
)

var (
	ErrBEEFNoMarker = errors.New("invalid format of transaction, BEEF marker not found")
	ErrBEEFPanic    = errors.New("panic while parsing beef")
	ErrBEEFParse    = errors.New("failed to parse beef")
)

func CheckBeefFormat(txHex []byte) bool {
	if len(txHex) < beefVersionBytesCount {
		return false
	}

	if txHex[2] != beefMarkerPart1 || txHex[3] != beefMarkerPart2 {
		return false
	}

	return true
}

func DecodeBEEF(beefHex []byte) (tx *sdkTx.Beef, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Join(ErrBEEFPanic, fmt.Errorf("%v", r))
		}
	}()

	beef, _, _, err := sdkTx.ParseBeef(beefHex)
	if err != nil {
		return nil, errors.Join(ErrBEEFParse, err)
	}

	return beef, nil
}

func GetUnminedTx(d *sdkTx.Beef) *sdkTx.Transaction {
	for _, beefTx := range d.Transactions {
		if beefTx.DataFormat == sdkTx.RawTx {
			return beefTx.Transaction
		}
	}

	return nil
}
