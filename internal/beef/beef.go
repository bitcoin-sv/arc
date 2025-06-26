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
	ErrBEEFPanic = errors.New("panic while parsing beef")
	ErrBEEFParse = errors.New("failed to parse beef")
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

func DecodeBEEF(beefHex []byte) (tx *sdkTx.Beef, txID string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Join(ErrBEEFPanic, fmt.Errorf("%v", r))
		}
	}()

	beef, _, txHash, err := sdkTx.ParseBeef(beefHex)
	if err != nil {
		return nil, "", errors.Join(ErrBEEFParse, err)
	}

	return beef, txHash.String(), nil
}
