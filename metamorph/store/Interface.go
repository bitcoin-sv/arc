package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
)

type StoreData struct {
	RawTx         []byte               `dynamodbav:"raw_tx"`
	StoredAt      time.Time            `dynamodbav:"stored_at"`
	AnnouncedAt   time.Time            `dynamodbav:"announced_at"`
	MinedAt       time.Time            `dynamodbav:"mined_at"`
	Hash          *chainhash.Hash      `badgerhold:"key" dynamodbav:"tx_hash"`
	Status        metamorph_api.Status `dynamodbav:"tx_status"`
	BlockHeight   uint64               `dynamodbav:"block_height"`
	BlockHash     *chainhash.Hash      `dynamodbav:"block_hash"`
	MerkleProof   bool                 `dynamodbav:"merkle_proof"`
	CallbackUrl   string               `dynamodbav:"callback_url"`
	CallbackToken string               `dynamodbav:"callback_token"`
	RejectReason  string               `dynamodbav:"reject_reason"`
}

var ErrNotFound = errors.New("txid could not be found")

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, key []byte, value *StoreData) error
	Del(ctx context.Context, key []byte) error

	IsCentralised() bool
	GetUnmined(_ context.Context, callback func(s *StoreData)) error
	UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error
	UpdateMined(ctx context.Context, hash *chainhash.Hash, blockHash *chainhash.Hash, blockHeight uint64) error
	Close(ctx context.Context) error
	GetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) (*time.Time, error)
	SetBlockProcessed(ctx context.Context, blockHash *chainhash.Hash) error
}

func encodeTime(buf *bytes.Buffer, tm time.Time) error {
	if tm.IsZero() {
		return binary.Write(buf, binary.BigEndian, int64(0))
	}
	return binary.Write(buf, binary.BigEndian, tm.UnixNano())
}

func decodeTime(buf io.Reader) (time.Time, error) {
	var tmpInt64 int64
	if err := binary.Read(buf, binary.BigEndian, &tmpInt64); err != nil {
		return time.Time{}, err
	}
	if tmpInt64 == 0 {
		return time.Time{}, nil
	}

	return time.Unix(0, tmpInt64), nil
}

func encodeHash(buf *bytes.Buffer, hash *chainhash.Hash) error {
	if hash == nil {
		if err := buf.WriteByte(0x00); err != nil {
			return err
		}
		return nil
	}

	if err := buf.WriteByte(0x01); err != nil {
		return err
	}
	if _, err := buf.Write(hash.CloneBytes()); err != nil {
		return err
	}

	return nil
}

func decodeHash(buf io.Reader) (*chainhash.Hash, error) {
	var hashCount [1]byte
	n, err := buf.Read(hashCount[:])
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, errors.New("invalid hash count")
	}

	if hashCount[0] == 0x01 {
		tmpHash32 := make([]byte, 32)
		var err error

		n, err := buf.Read(tmpHash32)
		if err != nil {
			return nil, err
		}
		if n != 32 {
			return nil, errors.New("invalid hash length")
		}

		return chainhash.NewHash(tmpHash32)
	}

	var nilHash *chainhash.Hash
	return nilHash, nil
}

func encodeString(buf *bytes.Buffer, str string) error {
	if err := binary.Write(buf, binary.BigEndian, uint16(len(str))); err != nil {
		return err
	}

	if _, err := buf.Write([]byte(str)); err != nil {
		return err
	}

	return nil
}

func decodeString(buf io.Reader) (string, error) {
	var tmpUint16 uint16
	if err := binary.Read(buf, binary.BigEndian, &tmpUint16); err != nil {
		return "", err
	}

	if tmpUint16 == 0 {
		return "", nil
	}

	tmpBuf := make([]byte, tmpUint16)
	n, err := buf.Read(tmpBuf)
	if err != nil {
		return "", err
	}
	if n != int(tmpUint16) {
		return "", errors.New("invalid string length")
	}

	return string(tmpBuf), nil
}
