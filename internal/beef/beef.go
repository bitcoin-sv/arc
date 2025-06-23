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

func DecodeBEEF(beefHex []byte) (tx *sdkTx.Beef, remainingBytest []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Join(ErrBEEFPanic, fmt.Errorf("%v", r))
		}
	}()

	beef, _, _, err := sdkTx.ParseBeef(beefHex)
	if err != nil {
		return nil, nil, errors.Join(ErrBEEFParse, err)
	}

	remainingBytes, err := extractBytesWithoutVersionAndMarker(beefHex)
	if err != nil {
		return nil, nil, err
	}

	return beef, remainingBytes, nil
}

func GetLatestTx(d *sdkTx.Beef) *sdkTx.Transaction {
	for _, beefTx := range d.Transactions {
		if beefTx.BumpIndex == 0 {
			return beefTx.Transaction
		}
	}

	return nil
}

func extractBytesWithoutVersionAndMarker(beefBytes []byte) ([]byte, error) {
	if !CheckBeefFormat(beefBytes) {
		return nil, ErrBEEFNoMarker
	}

	return beefBytes[beefVersionBytesCount:], nil
}
