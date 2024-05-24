package beef

import (
	"errors"
	"fmt"

	"github.com/libsv/go-bc"
	"github.com/libsv/go-bt/v2"
)

const (
	beefVersionBytesCount = 4
	hashBytesCount        = 32
	maxTreeHeight         = 64
)

const (
	beefMarkerPart1 = 0xBE
	beefMarkerPart2 = 0xEF
)

const (
	hasNoBump = 0x00
	hasBump   = 0x01
)

type TxData struct {
	Transaction *bt.Tx
	BumpIndex   *bt.VarInt
}

type BEEF struct {
	BUMPs        []*bc.BUMP
	Transactions []*TxData
}

func CheckBeefFormat(txHex []byte) bool {
	if len(txHex) < beefVersionBytesCount {
		return false
	}

	if txHex[2] != beefMarkerPart1 || txHex[3] != beefMarkerPart2 {
		return false
	}

	return true
}

func DecodeBEEF(beefHex []byte) (*bt.Tx, *BEEF, []byte, error) {
	beefBytes, err := extractBytesWithoutVersionAndMarker(beefHex)
	if err != nil {
		return nil, nil, nil, err
	}

	bumps, remainingBytes, err := decodeBUMPs(beefBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	transactions, remainingBytes, err := decodeTransactionsWithPathIndexes(remainingBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	decodedBeef := &BEEF{
		BUMPs:        bumps,
		Transactions: transactions,
	}

	return decodedBeef.GetLatestTx(), decodedBeef, remainingBytes, nil
}

func (d *BEEF) GetLatestTx() *bt.Tx {
	return d.Transactions[len(d.Transactions)-1].Transaction // get the last transaction as the processed transaction - it should be the last one because of khan's ordering
}

func decodeBUMPs(beefBytes []byte) ([]*bc.BUMP, []byte, error) {
	if len(beefBytes) == 0 {
		return nil, nil, errors.New("cannot decode BUMP - no bytes provided")
	}

	nBump, bytesUsed := bt.NewVarIntFromBytes(beefBytes)

	if nBump == 0 {
		return nil, nil, errors.New("invalid BEEF - lack of BUMPs")
	}

	beefBytes = beefBytes[bytesUsed:]

	bumps := make([]*bc.BUMP, 0, uint64(nBump))
	for i := uint64(0); i < uint64(nBump); i++ {
		if len(beefBytes) == 0 {
			return nil, nil, errors.New("insufficient bytes to extract BUMP blockHeight")
		}

		bump, bytesUsed, err := bc.NewBUMPFromStream(beefBytes)
		if err != nil {
			return nil, nil, err
		}

		beefBytes = beefBytes[bytesUsed:]

		bumps = append(bumps, bump)
	}

	return bumps, beefBytes, nil
}

func decodeTransactionsWithPathIndexes(bytes []byte) ([]*TxData, []byte, error) {
	if len(bytes) == 0 {
		return nil, nil, errors.New("invalid BEEF - no transaction")
	}

	nTransactions, offset := bt.NewVarIntFromBytes(bytes)

	if nTransactions < 2 {
		return nil, nil, errors.New("invalid BEEF- not enough transactions provided to decode BEEF")
	}

	bytes = bytes[offset:]

	transactions := make([]*TxData, 0, int(nTransactions))

	for i := 0; i < int(nTransactions); i++ {
		tx, offset, err := bt.NewTxFromStream(bytes)
		if err != nil {
			return nil, nil, err
		}
		bytes = bytes[offset:]

		if len(bytes) == 0 {
			return nil, nil, errors.New("invalid BEEF - no HasBUMP flag")
		}

		var pathIndex *bt.VarInt

		switch bytes[0] {
		case hasBump:
			bytes = bytes[1:]
			if len(bytes) == 0 {
				return nil, nil, errors.New("invalid BEEF - HasBUMP flag set, but no BUMP index")
			}
			value, offset := bt.NewVarIntFromBytes(bytes)
			pathIndex = &value
			bytes = bytes[offset:]
		case hasNoBump:
			bytes = bytes[1:]
		default:
			return nil, nil, fmt.Errorf("invalid HasCMP flag for transaction at index %d", i)
		}

		transactions = append(transactions, &TxData{
			Transaction: tx,
			BumpIndex:   pathIndex,
		})
	}

	return transactions, bytes, nil
}

func extractBytesWithoutVersionAndMarker(beefBytes []byte) ([]byte, error) {
	if !CheckBeefFormat(beefBytes) {
		return nil, errors.New("invalid format of transaction, BEEF marker not found")
	}

	return beefBytes[beefVersionBytesCount:], nil
}
