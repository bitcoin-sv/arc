package beef

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/bitcoin-sv/go-sdk/transaction"
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
	Transaction *transaction.Transaction
	BumpIndex   *transaction.VarInt
	txID        string
}

func (td *TxData) IsMined() bool {
	return td.BumpIndex != nil
}

func (td *TxData) GetTxID() string {
	if len(td.txID) == 0 {
		td.txID = td.Transaction.TxID()
	}

	return td.txID
}

type BEEF struct {
	BUMPs        []*transaction.MerklePath
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

func DecodeBEEF(beefHex []byte) (*BEEF, []byte, error) {
	remainingBytes, err := extractBytesWithoutVersionAndMarker(beefHex)
	if err != nil {
		return nil, nil, err
	}

	bumps, remainingBytes, err := decodeBUMPs(remainingBytes)
	if err != nil {
		return nil, nil, err
	}

	transactions, remainingBytes, err := decodeTransactionsWithPathIndexes(remainingBytes)
	if err != nil {
		return nil, nil, err
	}

	decodedBeef := &BEEF{
		BUMPs:        bumps,
		Transactions: transactions,
	}

	return decodedBeef, remainingBytes, nil
}

func (d *BEEF) GetLatestTx() *transaction.Transaction {
	return d.Transactions[len(d.Transactions)-1].Transaction // get the last transaction as the processed transaction - it should be the last one because of khan's ordering
}

func decodeBUMPs(beefBytes []byte) ([]*transaction.MerklePath, []byte, error) {
	if len(beefBytes) == 0 {
		return nil, nil, errors.New("cannot decode BUMP - no bytes provided")
	}

	nBump, bytesUsed := transaction.NewVarIntFromBytes(beefBytes)

	if nBump == 0 {
		return nil, nil, errors.New("invalid BEEF - lack of BUMPs")
	}

	beefBytes = beefBytes[bytesUsed:]

	bumps := make([]*transaction.MerklePath, 0, uint64(nBump))
	for i := uint64(0); i < uint64(nBump); i++ {
		fmt.Println(len(beefBytes))
		bump, err := transaction.NewMerklePathFromBinary(beefBytes)
		if err != nil {
			return nil, nil, err
		}

		// calculate the number of bytes used to encode the bump
		bumpBytes := bump.Bytes()
		usedBytes := beefBytes[:len(bumpBytes)]
		if !bytes.Equal(bumpBytes, usedBytes) {
			return nil, nil, errors.New("beef bytes not equal")
		}

		beefBytes = beefBytes[len(bumpBytes):]

		bumps = append(bumps, bump)
	}

	return bumps, beefBytes, nil
}

func decodeTransactionsWithPathIndexes(beefBytes []byte) ([]*TxData, []byte, error) {
	if len(beefBytes) == 0 {
		return nil, nil, errors.New("invalid BEEF - no transaction")
	}

	nTransactions, bytesUsed := transaction.NewVarIntFromBytes(beefBytes)

	if nTransactions < 2 {
		return nil, nil, errors.New("invalid BEEF- not enough transactions provided to decode BEEF")
	}

	beefBytes = beefBytes[bytesUsed:]

	transactions := make([]*TxData, 0, int(nTransactions))

	for i := 0; i < int(nTransactions); i++ {
		tx, bytesUsed, err := transaction.NewTransactionFromStream(beefBytes)
		if err != nil {
			return nil, nil, err
		}
		beefBytes = beefBytes[bytesUsed:]

		if len(beefBytes) == 0 {
			return nil, nil, errors.New("invalid BEEF - no HasBUMP flag")
		}

		var pathIndex *transaction.VarInt

		switch beefBytes[0] {
		case hasBump:
			beefBytes = beefBytes[1:]
			if len(beefBytes) == 0 {
				return nil, nil, errors.New("invalid BEEF - HasBUMP flag set, but no BUMP index")
			}
			value, bytesUsed := transaction.NewVarIntFromBytes(beefBytes)
			pathIndex = &value
			beefBytes = beefBytes[bytesUsed:]
		case hasNoBump:
			beefBytes = beefBytes[1:]
		default:
			return nil, nil, fmt.Errorf("invalid HasCMP flag for transaction at index %d", i)
		}

		transactions = append(transactions, &TxData{
			Transaction: tx,
			BumpIndex:   pathIndex,
			txID:        tx.TxID(),
		})
	}

	return transactions, beefBytes, nil
}

func extractBytesWithoutVersionAndMarker(beefBytes []byte) ([]byte, error) {
	if !CheckBeefFormat(beefBytes) {
		return nil, errors.New("invalid format of transaction, BEEF marker not found")
	}

	return beefBytes[beefVersionBytesCount:], nil
}
