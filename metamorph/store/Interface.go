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
	Hash          *chainhash.Hash      `badgerhold:"key"            dynamodbav:"tx_hash"`
	Status        metamorph_api.Status `dynamodbav:"tx_status"`
	BlockHeight   uint64               `dynamodbav:"block_height"`
	BlockHash     *chainhash.Hash      `dynamodbav:"block_hash"`
	MerkleProof   bool                 `dynamodbav:"merkle_proof"`
	CallbackUrl   string               `dynamodbav:"callback_url"`
	CallbackToken string               `dynamodbav:"callback_token"`
	RejectReason  string               `dynamodbav:"reject_reason"`
	LockedBy      string               `dynamodbav:"locked_by"`
	Ttl           int64                `dynamodbav:"ttl"`
}

func (sd *StoreData) EncodeToBytes() ([]byte, error) {
	var buf bytes.Buffer

	// Version
	if err := buf.WriteByte(0x01); err != nil {
		return nil, err
	}

	// RawTx
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(sd.RawTx))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(sd.RawTx); err != nil {
		return nil, err
	}

	// StoredAt
	if err := encodeTime(&buf, sd.StoredAt); err != nil {
		return nil, err
	}

	// AnnouncedAt
	if err := encodeTime(&buf, sd.AnnouncedAt); err != nil {
		return nil, err
	}

	// MinedAt
	if err := encodeTime(&buf, sd.MinedAt); err != nil {
		return nil, err
	}

	// Hash
	if err := encodeHash(&buf, sd.Hash); err != nil {
		return nil, err
	}

	// Status
	if err := binary.Write(&buf, binary.BigEndian, sd.Status); err != nil {
		return nil, err
	}

	// BlockHeight
	if err := binary.Write(&buf, binary.BigEndian, sd.BlockHeight); err != nil {
		return nil, err
	}

	// BlockHash
	if err := encodeHash(&buf, sd.BlockHash); err != nil {
		return nil, err
	}

	// MerkleProof
	if sd.MerkleProof {
		if err := buf.WriteByte(0x01); err != nil {
			return nil, err
		}
	} else {
		if err := buf.WriteByte(0x00); err != nil {
			return nil, err
		}
	}

	// CallbackUrl
	if err := encodeString(&buf, sd.CallbackUrl); err != nil {
		return nil, err
	}

	// CallbackToken
	if err := encodeString(&buf, sd.CallbackToken); err != nil {
		return nil, err
	}

	// RejectReason
	if err := encodeString(&buf, sd.RejectReason); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeFromBytes(b []byte) (*StoreData, error) {
	sd := &StoreData{}

	buf := bytes.NewReader(b)

	// Version
	version, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	if version != 0x01 {
		return nil, errors.New("invalid version")
	}

	// RawTx
	var tmpUint32 uint32
	if err := binary.Read(buf, binary.BigEndian, &tmpUint32); err != nil {
		return nil, err
	}

	sd.RawTx = make([]byte, tmpUint32)
	n, err := buf.Read(sd.RawTx)
	if err != nil {
		return nil, err
	}
	if n != int(tmpUint32) {
		return nil, errors.New("invalid rawTx length")
	}

	// StoredAt
	if sd.StoredAt, err = decodeTime(buf); err != nil {
		return nil, err
	}

	// AnnouncedAt
	if sd.AnnouncedAt, err = decodeTime(buf); err != nil {
		return nil, err
	}

	// MinedAt
	if sd.MinedAt, err = decodeTime(buf); err != nil {
		return nil, err
	}

	// Hash
	if sd.Hash, err = decodeHash(buf); err != nil {
		return nil, err
	}

	// Status
	var tmpInt32 int32
	if err := binary.Read(buf, binary.BigEndian, &tmpInt32); err != nil {
		return nil, err
	}
	sd.Status = metamorph_api.Status(tmpInt32)

	// BlockHeight
	if err := binary.Read(buf, binary.BigEndian, &sd.BlockHeight); err != nil {
		return nil, err
	}

	// BlockHash
	if sd.BlockHash, err = decodeHash(buf); err != nil {
		return nil, err
	}

	// MerkleProof
	var tmpByte byte
	if tmpByte, err = buf.ReadByte(); err != nil {
		return nil, err
	}
	sd.MerkleProof = tmpByte == 0x01

	// CallbackUrl
	if sd.CallbackUrl, err = decodeString(buf); err != nil {
		return nil, err
	}

	// CallbackToken
	if sd.CallbackToken, err = decodeString(buf); err != nil {
		return nil, err
	}

	// RejectReason
	if sd.RejectReason, err = decodeString(buf); err != nil {
		return nil, err
	}

	return sd, nil
}

var ErrNotFound = errors.New("key could not be found")

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	Set(ctx context.Context, key []byte, value *StoreData) error
	Del(ctx context.Context, key []byte) error

	IsCentralised() bool
	SetUnlocked(ctx context.Context, hashes []*chainhash.Hash) error
	SetUnlockedByName(ctx context.Context, lockedBy string) (int, error)
	GetUnmined(_ context.Context, callback func(s *StoreData)) error
	UpdateStatus(ctx context.Context, hash *chainhash.Hash, status metamorph_api.Status, rejectReason string) error
	RemoveCallbacker(ctx context.Context, hash *chainhash.Hash) error
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

	if _, err := buf.WriteString(str); err != nil {
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
