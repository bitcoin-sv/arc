package store

import (
	"context"
	"errors"
	"time"

	"github.com/TAAL-GmbH/arc/metamorph/metamorph_api"
)

type StoreData struct {
	StoredAt      time.Time
	AnnouncedAt   time.Time
	MinedAt       time.Time
	Hash          []byte `badgerhold:"key"`
	Status        metamorph_api.Status
	BlockHeight   uint64
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

var ErrNotFound = errors.New("txid could not be found")

type MetamorphStore interface {
	Get(ctx context.Context, key []byte) (*StoreData, error)
	GetUnmined(_ context.Context, callback func(s *StoreData)) error
	Set(ctx context.Context, key []byte, value *StoreData) error
	UpdateStatus(ctx context.Context, hash []byte, status metamorph_api.Status, rejectReason string) error
	UpdateMined(ctx context.Context, hash []byte, blockHash []byte, blockHeight uint64) error
	Del(ctx context.Context, key []byte) error
	Close(ctx context.Context) error
	GetBlockProcessed(ctx context.Context, blockHash []byte) (*time.Time, error)
	SetBlockProcessed(ctx context.Context, blockHash []byte) error
}
