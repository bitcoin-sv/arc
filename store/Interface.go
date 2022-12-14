package store

import (
	"context"
	"errors"
	"time"

	pb "github.com/TAAL-GmbH/arc/metamorph_api"
)

type StoreData struct {
	StoredAt      time.Time
	AnnouncedAt   time.Time
	MinedAt       time.Time
	Hash          []byte `badgerhold:"key"`
	Status        pb.Status
	BlockHeight   int32
	BlockHash     []byte
	ApiKeyId      int64
	StandardFeeId int64
	DataFeeId     int64
	SourceIp      string
	CallbackUrl   string
	CallbackToken string
	MerkleProof   bool
	RawTx         []byte
	RejectReason  string
}

var ErrNotFound = errors.New("key could not be found")

type Store interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	GetUnseen(_ context.Context, callback func(s *StoreData)) error
	Set(ctx context.Context, key []byte, value *StoreData) error
	UpdateStatus(ctx context.Context, hash []byte, status pb.Status, rejectReason string) error
	Del(ctx context.Context, key []byte) error
	Close(ctx context.Context) error
}
