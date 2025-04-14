package beef

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/bitcoin-sv/arc/internal/api"
	sdkTx "github.com/bsv-blockchain/go-sdk/transaction"
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

var (
	ErrBEEFNoBytesProvided = errors.New("cannot decode BEEF - no bytes provided")
	ErrBEEFLackOfBUMPs     = errors.New("cannot decode BEEF - lack of BUMPs")
	ErrBEEFBytesNotEqual   = errors.New("cannot decode BEEF - bytes not equal")
	ErrBEEFNoTransactions  = errors.New("invalid BEEF - no transactions")
	ErrBEEFNotEnoughTx     = errors.New("invalid BEEF - not enough transactions provided to decode BEEF")
	ErrBEEFNoFlag          = errors.New("invalid BEEF - no HasBUMP flag")
	ErrBEEFNoBUMPIndex     = errors.New("invalid BEEF - HasBUMP flag set, but no BUMP index")
	ErrBEEFInvalidFlag     = errors.New("invalid BEEF - invalid HasCMP flag")
	ErrBEEFNoMarker        = errors.New("invalid format of transaction, BEEF marker not found")
)

type TxData struct {
	Transaction *sdkTx.Transaction
	BumpIndex   *sdkTx.VarInt
	txID        string
}

func (td *TxData) IsMined() bool {
	return td.BumpIndex != nil
}

func (td *TxData) GetTxID() string {
	if len(td.txID) == 0 {
		td.txID = td.Transaction.TxID().String()
	}

	return td.txID
}

type BEEF struct {
	BUMPs        []*sdkTx.MerklePath
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

func (d *BEEF) GetLatestTx() *sdkTx.Transaction {
	return d.Transactions[len(d.Transactions)-1].Transaction // get the last transaction as the processed transaction - it should be the last one because of khan's ordering
}

func decodeBUMPs(beefBytes []byte) ([]*sdkTx.MerklePath, []byte, error) {
	if len(beefBytes) == 0 {
		return nil, nil, ErrBEEFNoBytesProvided
	}

	nBump, bytesUsed := sdkTx.NewVarIntFromBytes(beefBytes)

	if nBump == 0 {
		return nil, nil, ErrBEEFLackOfBUMPs
	}

	beefBytes = beefBytes[bytesUsed:]

	bumps := make([]*sdkTx.MerklePath, 0, uint64(nBump))
	for i := uint64(0); i < uint64(nBump); i++ {
		bump, err := sdkTx.NewMerklePathFromBinary(beefBytes)
		if err != nil {
			return nil, nil, err
		}

		// calculate the number of bytes used to encode the bump
		bumpBytes := bump.Bytes()
		usedBytes := beefBytes[:len(bumpBytes)]
		if !bytes.Equal(bumpBytes, usedBytes) {
			return nil, nil, ErrBEEFBytesNotEqual
		}

		beefBytes = beefBytes[len(bumpBytes):]

		bumps = append(bumps, bump)
	}

	return bumps, beefBytes, nil
}

func decodeTransactionsWithPathIndexes(beefBytes []byte) ([]*TxData, []byte, error) {
	if len(beefBytes) == 0 {
		return nil, nil, ErrBEEFNoTransactions
	}

	nTransactions, bytesUsed := sdkTx.NewVarIntFromBytes(beefBytes)

	if nTransactions < 2 {
		return nil, nil, ErrBEEFNotEnoughTx
	}

	beefBytes = beefBytes[bytesUsed:]

	ntxs, err := api.SafeInt64ToInt(uint64(nTransactions))
	if err != nil {
		return nil, nil, err
	}
	transactions := make([]*TxData, 0, ntxs)

	for i := 0; i < int(nTransactions); i++ {
		tx, bytesUsed, err := sdkTx.NewTransactionFromStream(beefBytes)
		if err != nil {
			return nil, nil, err
		}
		beefBytes = beefBytes[bytesUsed:]

		if len(beefBytes) == 0 {
			return nil, nil, ErrBEEFNoFlag
		}

		var pathIndex *sdkTx.VarInt

		switch beefBytes[0] {
		case hasBump:
			beefBytes = beefBytes[1:]
			if len(beefBytes) == 0 {
				return nil, nil, ErrBEEFNoBUMPIndex
			}
			value, bytesUsed := sdkTx.NewVarIntFromBytes(beefBytes)
			pathIndex = &value
			beefBytes = beefBytes[bytesUsed:]
		case hasNoBump:
			beefBytes = beefBytes[1:]
		default:
			return nil, nil, errors.Join(ErrBEEFInvalidFlag, fmt.Errorf("transaction at index %d", i))
		}

		transactions = append(transactions, &TxData{
			Transaction: tx,
			BumpIndex:   pathIndex,
			txID:        tx.TxID().String(),
		})
	}

	return transactions, beefBytes, nil
}

func extractBytesWithoutVersionAndMarker(beefBytes []byte) ([]byte, error) {
	if !CheckBeefFormat(beefBytes) {
		return nil, ErrBEEFNoMarker
	}

	return beefBytes[beefVersionBytesCount:], nil
}
