package beef

import (
	"errors"

	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
)

const (
	beefVersionBytesCount = 4
	//hashBytesCount        = 32
	//maxTreeHeight         = 64
)

const (
	beefMarkerPart1 = 0xBE
	beefMarkerPart2 = 0xEF
)

//const (
//	hasNoBump = 0x00
//	hasBump   = 0x01
//)

var (
	//ErrBEEFNoBytesProvided = errors.New("cannot decode BEEF - no bytes provided")
	//ErrBEEFLackOfBUMPs     = errors.New("cannot decode BEEF - lack of BUMPs")
	//ErrBEEFNotEnoughTx     = errors.New("invalid BEEF - not enough transactions provided to decode BEEF")
	//ErrBEEFNoBUMPIndex     = errors.New("invalid BEEF - HasBUMP flag set, but no BUMP index")
	ErrBEEFNoMarker = errors.New("invalid format of transaction, BEEF marker not found")
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

func DecodeBEEF(beefHex []byte) (*sdkTx.Beef, []byte, error) {
	beef, err := sdkTx.NewBeefFromBytes(beefHex)
	if err != nil {
		return nil, nil, err
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
